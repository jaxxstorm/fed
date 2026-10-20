package assessment

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/storage"
)

const AssessmentReleaseFormat = "fed-assessment-release/v1"
const MaxReleaseArchiveBytes int64 = 256 << 20

type ServiceContext struct {
	Description      string               `json:"description"`
	Contacts         []string             `json:"contacts"`
	Boundary         string               `json:"boundary"`
	Responsibilities string               `json:"responsibilities"`
	Deliverables     []ReleaseDeliverable `json:"deliverables"`
}
type ReleaseDeliverable struct {
	RequirementID string `json:"requirement_id"`
	Description   string `json:"description"`
	Evidence      Ref    `json:"evidence"`
}
type PublicRequirement struct {
	RequirementID string `json:"requirement_id"`
	Summary       string `json:"summary"`
}
type PublicRelease struct {
	ServiceName  string              `json:"service_name"`
	Summary      string              `json:"summary"`
	Contacts     []string            `json:"contacts"`
	Requirements []PublicRequirement `json:"requirements"`
}
type ReleaseEvidence struct {
	Reference Ref      `json:"reference"`
	Hash      string   `json:"hash"`
	Path      string   `json:"path"`
	Metadata  Evidence `json:"metadata"`
}
type ReleaseRequirement struct {
	Deliverables           []ReleaseDeliverable    `json:"deliverables"`
	UnresolvedDeliverables []string                `json:"unresolved_deliverables"`
	Requirement            catalog.Resolved        `json:"requirement"`
	ExpectedDeliverables   []string                `json:"expected_deliverables"`
	Implementation         *ImplementationContext  `json:"implementation,omitempty"`
	Reviews                *AssessmentReviewTarget `json:"reviews,omitempty"`
	Collection             ObservationStatus       `json:"collection"`
	Issues                 []Record                `json:"issues"`
}
type ReleaseManifest struct {
	Format            string               `json:"format"`
	Label             string               `json:"label"`
	ServiceID         string               `json:"service_id"`
	PreparedAt        time.Time            `json:"prepared_at"`
	Period            AssessmentPeriod     `json:"period"`
	Profile           catalog.Profile      `json:"profile"`
	ProfileRevision   int                  `json:"profile_revision"`
	Scope             Record               `json:"scope"`
	Context           ServiceContext       `json:"context"`
	Requirements      []ReleaseRequirement `json:"requirements"`
	Evidence          []ReleaseEvidence    `json:"evidence"`
	Public            PublicRelease        `json:"public"`
	PreviousReleaseID string               `json:"previous_release_id,omitempty"`
	Changes           []ReleaseChange      `json:"changes"`
	Blockers          []string             `json:"blockers"`
	Incomplete        bool                 `json:"incomplete"`
}
type ReleaseChange struct {
	Category      string `json:"category"`
	RequirementID string `json:"requirement_id,omitempty"`
	Explanation   string `json:"explanation"`
}
type PrepareReleaseInput struct {
	ServiceID         string           `json:"service_id"`
	IdempotencyKey    string           `json:"idempotency_key"`
	Period            AssessmentPeriod `json:"period"`
	Public            PublicRelease    `json:"public"`
	PreviousReleaseID string           `json:"previous_release_id,omitempty"`
}
type AssessmentRelease struct {
	ID           string          `json:"id"`
	Revision     int             `json:"revision"`
	State        string          `json:"state"`
	ManifestHash string          `json:"manifest_hash"`
	AudienceHash string          `json:"audience_hash"`
	ArchiveHash  string          `json:"archive_hash"`
	Manifest     ReleaseManifest `json:"manifest"`
}

func (s *Store) SaveServiceContext(ctx context.Context, a auth.Actor, service string, expected int, v ServiceContext) (Record, error) {
	if strings.TrimSpace(v.Description) == "" || strings.TrimSpace(v.Boundary) == "" || strings.TrimSpace(v.Responsibilities) == "" || len(v.Contacts) == 0 || len(v.Contacts) > 100 || len(v.Deliverables) > 1000 || len(data(v)) > 1<<20 {
		return Record{}, ErrInvalid
	}
	for _, contact := range v.Contacts {
		if strings.TrimSpace(contact) == "" || len(contact) > 4096 {
			return Record{}, ErrInvalid
		}
	}
	return s.mutate(ctx, a, record(service, "service_context", "service_context", "", v), expected, "edit", func(tx pgx.Tx) error {
		for _, d := range v.Deliverables {
			if d.Description == "" {
				return ErrInvalid
			}
			if err := requireRule(ctx, tx, service, d.RequirementID); err != nil {
				return err
			}
			if _, err := reference(ctx, tx, service, "evidence", d.Evidence); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) GetRelease(ctx context.Context, a auth.Actor, service, id string) (AssessmentRelease, error) {
	if err := auth.Authorize(ctx, s.query(), a, service, "read"); err != nil {
		return AssessmentRelease{}, err
	}
	return s.release(ctx, service, id)
}
func (s *Store) release(ctx context.Context, service, id string) (AssessmentRelease, error) {
	out := AssessmentRelease{ID: id}
	var raw []byte
	err := s.query().QueryRow(ctx, `SELECT r.revision,r.state,r.manifest_hash,r.audience_hash,r.archive_key,d.data FROM assessment_releases r JOIN records d ON d.service_id=r.service_id AND d.kind='assessment_release' AND d.id=r.id AND d.revision=1 WHERE r.service_id=$1 AND r.id=$2`, service, id).Scan(&out.Revision, &out.State, &out.ManifestHash, &out.AudienceHash, &out.ArchiveHash, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal(raw, &out.Manifest); err != nil {
		return out, err
	}
	if storage.Hash(data(out.Manifest)) != out.ManifestHash || storage.Hash(data(out.Manifest.Public)) != out.AudienceHash {
		return out, fmt.Errorf("%w: release manifest integrity failure", ErrInvalid)
	}
	return out, nil
}
func (s *Store) ListReleases(ctx context.Context, a auth.Actor, service string, limit, offset int) ([]AssessmentRelease, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, ErrInvalid
	}
	if err := auth.Authorize(ctx, s.query(), a, service, "read"); err != nil {
		return nil, err
	}
	rows, err := s.query().Query(ctx, `SELECT id FROM assessment_releases WHERE service_id=$1 ORDER BY prepared_at DESC,id LIMIT $2 OFFSET $3`, service, limit, offset)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	out := []AssessmentRelease{}
	for _, id := range ids {
		r, err := s.release(ctx, service, id)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *Store) PrepareAssessmentRelease(ctx context.Context, a auth.Actor, in PrepareReleaseInput) (AssessmentRelease, error) {
	if in.IdempotencyKey == "" || len(in.IdempotencyKey) > 256 || in.Period.Start.IsZero() || in.Period.End.Before(in.Period.Start) || in.Period.End.After(s.now()) || strings.TrimSpace(in.Public.ServiceName) == "" || len(data(in.Public)) > 1<<20 {
		return AssessmentRelease{}, ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return AssessmentRelease{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, in.ServiceID, "export"); err != nil {
		return AssessmentRelease{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, in.ServiceID); err != nil {
		return AssessmentRelease{}, err
	}
	digest := storage.Hash(data(in))
	var receipt struct {
		ID string `json:"id"`
	}
	view := *s
	view.reader = tx
	view.presentationCache = nil
	if found, e := readOperation(ctx, tx, a, in.ServiceID, "prepare_assessment_release", in.IdempotencyKey, digest, &receipt); found || e != nil {
		if e != nil {
			return AssessmentRelease{}, e
		}
		return view.release(ctx, in.ServiceID, receipt.ID)
	}
	id := uuid.NewString()
	manifest := ReleaseManifest{Format: AssessmentReleaseFormat, Label: "Fed assessment package; not an authorization or an official schema-validated submission. Unassessed and partial requirements remain explicit.", ServiceID: in.ServiceID, PreparedAt: s.now().UTC(), Period: in.Period, Public: in.Public, PreviousReleaseID: in.PreviousReleaseID, Requirements: []ReleaseRequirement{}, Evidence: []ReleaseEvidence{}, Changes: []ReleaseChange{}, Blockers: []string{}}
	view.clock = func() time.Time { return manifest.PreparedAt }
	view.presentationCache, err = view.loadPresentationCache(ctx, in.ServiceID, a.ID, manifest.PreparedAt)
	if err != nil {
		return AssessmentRelease{}, err
	}
	contextRecord, err := view.Get(ctx, a, in.ServiceID, "service_context", "service_context")
	if err != nil {
		return AssessmentRelease{}, err
	}
	manifest.Context, err = Decode[ServiceContext](contextRecord)
	if err != nil {
		return AssessmentRelease{}, err
	}
	profile, err := view.Get(ctx, a, in.ServiceID, "profile", "profile")
	if err != nil {
		return AssessmentRelease{}, err
	}
	manifest.Profile, err = Decode[catalog.Profile](profile)
	if err != nil {
		return AssessmentRelease{}, err
	}
	manifest.ProfileRevision = profile.Revision
	manifest.Scope, err = view.Get(ctx, a, in.ServiceID, "scope", "scope")
	if err != nil {
		return AssessmentRelease{}, err
	}
	requirements, err := view.Requirements(ctx, a, in.ServiceID)
	if err != nil {
		return AssessmentRelease{}, err
	}
	slices.SortFunc(requirements, func(a, b catalog.Resolved) int { return strings.Compare(a.Requirement.ID, b.Requirement.ID) })
	evidence := map[Ref]bool{}
	for _, d := range manifest.Context.Deliverables {
		evidence[d.Evidence] = true
	}
	known := map[string]bool{}
	for _, requirement := range requirements {
		rid := requirement.Requirement.ID
		known[rid] = true
		instructions, err := view.RuleInstructions(ctx, a, in.ServiceID, rid)
		if err != nil {
			return AssessmentRelease{}, fmt.Errorf("release instructions %s: %w", rid, err)
		}
		status, err := view.Status(ctx, a, in.ServiceID, rid, manifest.PreparedAt)
		if err != nil {
			return AssessmentRelease{}, fmt.Errorf("release status %s: %w", rid, err)
		}
		item := ReleaseRequirement{Requirement: requirement, ExpectedDeliverables: actionExpectations(requirement), Implementation: instructions.Implementation, Reviews: instructions.ReviewTarget, Collection: status.Collection, Issues: []Record{}}
		item.Deliverables = []ReleaseDeliverable{}
		item.UnresolvedDeliverables = []string{}
		for _, d := range manifest.Context.Deliverables {
			if d.RequirementID == rid {
				item.Deliverables = append(item.Deliverables, d)
			}
		}
		if requirement.Applicability != "not_applicable" && len(item.ExpectedDeliverables) > 0 && len(item.Deliverables) == 0 {
			item.UnresolvedDeliverables = append(item.UnresolvedDeliverables, "No exact evidence mapping provided for the pinned requirement's expected supporting materials. Human review must establish adequacy of any supplied mapping.")
			manifest.Incomplete = true
		}
		for offset := 0; ; offset += 100 {
			issues, err := view.List(ctx, a, in.ServiceID, "assessment_issue", rid, 100, offset)
			if err != nil {
				return AssessmentRelease{}, err
			}
			item.Issues = append(item.Issues, issues...)
			if len(issues) < 100 {
				break
			}
		}
		if item.Implementation == nil || item.Implementation.Assessment == nil {
			if requirement.Applicability != "not_applicable" {
				manifest.Incomplete = true
			}
		} else {
			v := item.Implementation.Assessment
			if !v.Period.Start.Equal(in.Period.Start) || !v.Period.End.Equal(in.Period.End) {
				manifest.Blockers = append(manifest.Blockers, rid+": assessment period differs from release period")
			}
			if !item.Implementation.Readiness.Ready {
				manifest.Blockers = append(manifest.Blockers, rid+": assessment requires structural revalidation")
			}
			if item.Reviews == nil || item.Reviews.Adoption != "accepted" || item.Reviews.Review != "accepted" {
				manifest.Blockers = append(manifest.Blockers, rid+": organizational adoption and human evidence review required")
			}
			for _, assertion := range v.Assertions {
				for _, event := range assertion.Sample.Events {
					evidence[event.Citation.Evidence] = true
				}
				if assertion.Implementation.Outcome != "supported" || assertion.Operating.Outcome != "supported" {
					manifest.Incomplete = true
				}
				for _, support := range []EvidenceSupport{assertion.Implementation, assertion.Operating} {
					for _, c := range support.Citations {
						evidence[c.Evidence] = true
					}
				}
			}
		}
		for _, issueRecord := range item.Issues {
			issue, err := Decode[AssessmentIssue](issueRecord)
			if err != nil {
				return AssessmentRelease{}, err
			}
			if issue.State != "closed" {
				manifest.Incomplete = true
			}
			for _, c := range issue.ClosureEvidence {
				evidence[c.Evidence] = true
			}
			for _, milestone := range issue.Milestones {
				for _, c := range milestone.Evidence {
					evidence[c.Evidence] = true
				}
			}
		}
		manifest.Requirements = append(manifest.Requirements, item)
	}
	publicIDs := map[string]bool{}
	for _, r := range in.Public.Requirements {
		if !known[r.RequirementID] || publicIDs[r.RequirementID] || strings.TrimSpace(r.Summary) == "" {
			return AssessmentRelease{}, ErrInvalid
		}
		publicIDs[r.RequirementID] = true
	}
	for ref := range evidence {
		r, err := reference(ctx, tx, in.ServiceID, "evidence", ref)
		if err != nil {
			return AssessmentRelease{}, err
		}
		ev, err := Decode[Evidence](r)
		if err != nil {
			return AssessmentRelease{}, err
		}
		if ev.ArtifactKey == "" || s.Artifacts == nil {
			return AssessmentRelease{}, fmt.Errorf("%w: artifact unavailable for %s", ErrInvalid, ref.ID)
		}
		manifest.Evidence = append(manifest.Evidence, ReleaseEvidence{Reference: ref, Hash: ev.ArtifactKey, Path: "evidence/" + ev.ArtifactKey, Metadata: ev})
	}
	slices.SortFunc(manifest.Evidence, func(a, b ReleaseEvidence) int {
		if x := strings.Compare(a.Reference.ID, b.Reference.ID); x != 0 {
			return x
		}
		return a.Reference.Revision - b.Reference.Revision
	})
	if in.PreviousReleaseID != "" {
		previous, err := view.release(ctx, in.ServiceID, in.PreviousReleaseID)
		if err != nil {
			return AssessmentRelease{}, err
		}
		manifest.Changes = CompareAssessmentReleases(previous.Manifest, manifest)
	}
	if err = validateReleaseManifest(manifest); err != nil {
		return AssessmentRelease{}, err
	}
	archiveDigest := sha256.New()
	if err = s.WriteReleaseArchive(ctx, archiveDigest, manifest); err != nil {
		return AssessmentRelease{}, err
	}
	out := AssessmentRelease{ID: id, Revision: 1, State: "prepared", ManifestHash: storage.Hash(data(manifest)), AudienceHash: storage.Hash(data(manifest.Public)), ArchiveHash: hex.EncodeToString(archiveDigest.Sum(nil)), Manifest: manifest}
	if len(data(manifest)) > storage.MaxArtifactBytes {
		return AssessmentRelease{}, fmt.Errorf("%w: manifest exceeds size limit", ErrInvalid)
	}
	if _, err = insertRecord(ctx, tx, a, record(in.ServiceID, "assessment_release", id, "", manifest), 1); err != nil {
		return AssessmentRelease{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO assessment_releases(service_id,id,manifest_hash,audience_hash,archive_key,state) VALUES($1,$2,$3,$4,$5,'prepared')`, in.ServiceID, id, out.ManifestHash, out.AudienceHash, out.ArchiveHash); err != nil {
		return AssessmentRelease{}, err
	}
	receipt.ID = id
	if err = saveOperation(ctx, tx, a, in.ServiceID, "prepare_assessment_release", in.IdempotencyKey, digest, receipt); err != nil {
		return AssessmentRelease{}, err
	}
	return out, commitOperation(ctx, tx, a, in.ServiceID, "assessment.release_prepared", Record{ID: id, Revision: 1})
}

// Archive generation streams bounded artifacts one at a time. The archive hash
// is verified during preparation; prepared manifest and local paths are stable.
func (s *Store) WriteReleaseArchive(ctx context.Context, w io.Writer, m ReleaseManifest) error {
	if err := validateReleaseManifest(m); err != nil {
		return err
	}
	zw := zip.NewWriter(w)
	var total int64
	write := func(name string, content []byte) error {
		total += int64(len(content))
		if total > MaxReleaseArchiveBytes {
			return fmt.Errorf("%w: release exceeds archive size limit", ErrInvalid)
		}
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
		entry, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = entry.Write(content)
		return err
	}
	if err := write("manifest.json", data(m)); err != nil {
		return err
	}
	if err := write("schema.json", []byte(ReleaseManifestSchema)); err != nil {
		return err
	}
	html, err := ReleaseHTML(m)
	if err != nil {
		return err
	}
	if err = write("index.html", html); err != nil {
		return err
	}
	checksums := ""
	seen := map[string]bool{}
	for _, ev := range m.Evidence {
		if seen[ev.Hash] {
			continue
		}
		seen[ev.Hash] = true
		if s.Artifacts == nil {
			return ErrInvalid
		}
		content, err := s.Artifacts.Get(ctx, ev.Hash)
		if err != nil {
			return fmt.Errorf("artifact %s: %w", ev.Reference.ID, err)
		}
		if storage.Hash(content) != ev.Hash {
			return fmt.Errorf("%w: artifact integrity failure", ErrInvalid)
		}
		if err = write(ev.Path, content); err != nil {
			return err
		}
		checksums += ev.Hash + "  " + ev.Path + "\n"
	}
	checksums += storage.Hash(data(m)) + "  manifest.json\n" + storage.Hash(html) + "  index.html\n" + storage.Hash([]byte(ReleaseManifestSchema)) + "  schema.json\n"
	if err = write("SHA256SUMS", []byte(checksums)); err != nil {
		return err
	}
	return zw.Close()
}

var releaseTemplate = template.Must(template.New("release").Funcs(template.FuncMap{
	"issue":    Decode[AssessmentIssue],
	"decision": Decode[AssessmentDecision],
	"evidencePath": func(m ReleaseManifest, ref Ref) string {
		for _, ev := range m.Evidence {
			if ev.Reference == ref {
				return ev.Path
			}
		}
		return ""
	},
}).Parse(releaseHTMLTemplate))

func ReleaseHTML(m ReleaseManifest) ([]byte, error) {
	var b strings.Builder
	err := releaseTemplate.Execute(&b, m)
	return []byte(b.String()), err
}

func CompareAssessmentReleases(old, next ReleaseManifest) []ReleaseChange {
	out := []ReleaseChange{}
	assessmentContent := func(v *ImplementationContext) any {
		if v == nil {
			return nil
		}
		return v.Assessment
	}
	freshness := func(v *ImplementationContext) string {
		if v == nil {
			return ""
		}
		return v.CurrentFreshness
	}
	if string(data(old.Period)) != string(data(next.Period)) || string(data(old.Evidence)) != string(data(next.Evidence)) {
		out = append(out, ReleaseChange{Category: "evidence_freshness", Explanation: "Reporting period or pinned evidence changed."})
	}
	if old.ProfileRevision != next.ProfileRevision || string(data(old.Profile)) != string(data(next.Profile)) || string(data(old.Scope)) != string(data(next.Scope)) {
		out = append(out, ReleaseChange{Category: "applicability_boundary", Explanation: "Pinned profile or boundary changed."})
	}
	if string(data(old.Context)) != string(data(next.Context)) {
		out = append(out, ReleaseChange{Category: "implementation", Explanation: "Service context or supporting deliverables changed."})
	}
	prior := map[string]ReleaseRequirement{}
	for _, r := range old.Requirements {
		prior[r.Requirement.Requirement.ID] = r
	}
	for _, r := range next.Requirements {
		id := r.Requirement.Requirement.ID
		p, exists := prior[id]
		if !exists {
			out = append(out, ReleaseChange{Category: "applicability_boundary", RequirementID: id, Explanation: "Requirement added."})
			continue
		}
		delete(prior, id)
		for _, c := range []struct {
			category string
			a, b     any
		}{{"implementation", assessmentContent(p.Implementation), assessmentContent(r.Implementation)}, {"evidence_freshness", struct {
			Collection ObservationStatus
			Freshness  string
		}{p.Collection, freshness(p.Implementation)}, struct {
			Collection ObservationStatus
			Freshness  string
		}{r.Collection, freshness(r.Implementation)}}, {"review", p.Reviews, r.Reviews}, {"issues", p.Issues, r.Issues}, {"applicability_boundary", p.Requirement, r.Requirement}} {
			if string(data(c.a)) != string(data(c.b)) {
				out = append(out, ReleaseChange{Category: c.category, RequirementID: id, Explanation: "Content changed since the selected previous release."})
			}
		}
	}
	for id := range prior {
		out = append(out, ReleaseChange{Category: "applicability_boundary", RequirementID: id, Explanation: "Requirement removed from selected profile."})
	}
	slices.SortFunc(out, func(a, b ReleaseChange) int {
		return strings.Compare(a.Category+"/"+a.RequirementID, b.Category+"/"+b.RequirementID)
	})
	return out
}
