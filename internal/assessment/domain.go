package assessment

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/observation"
)

type Scope struct {
	Workspace    []WorkspaceScope  `json:"workspace,omitempty"`
	Kubernetes   []KubernetesScope `json:"kubernetes,omitempty"`
	Accounts     []Account         `json:"accounts"`
	Repositories []Repository      `json:"repositories"`
	Description  string            `json:"description"`
	Discovery    *ScopeDiscovery   `json:"discovery,omitempty"`
}

type WorkspaceScope struct {
	CustomerID string   `json:"customer_id"`
	OrgUnitIDs []string `json:"org_unit_ids"`
	GroupIDs   []string `json:"group_ids"`
}

type NamespaceScope struct {
	Name string `json:"name"`
	UID  string `json:"uid"`
}

type KubernetesScope struct {
	ClusterID   string           `json:"cluster_id"`
	Server      string           `json:"server"`
	ClusterWide bool             `json:"cluster_wide"`
	Namespaces  []NamespaceScope `json:"namespaces"`
}

// Scope metadata records discovery limitations alongside the saved candidate.
// Like the scope itself, this is editable configuration, not verified evidence.
type ScopeDiscovery struct {
	ObservedAt  time.Time `json:"observed_at"`
	Errors      []string  `json:"errors"`
	Limitations []string  `json:"limitations"`
}
type Account struct {
	ID       string   `json:"id"`
	Regions  []string `json:"regions"`
	Clusters []string `json:"clusters"`
	RoleARN  string   `json:"role_arn,omitempty"`
}
type Repository struct {
	Name   string   `json:"name"`
	Commit string   `json:"commit"`
	Paths  []string `json:"paths"`
}
type Ref struct {
	ID       string `json:"id"`
	Revision int    `json:"revision"`
}
type Measure struct {
	Policy                map[string]json.RawMessage `json:"policy,omitempty"`
	ConfigurationRevision int                        `json:"configuration_revision,omitempty"`
	Name                  string                     `json:"name"`
	Rationale             string                     `json:"rationale"`
	CheckID               string                     `json:"check_id,omitempty"`
	Expected              []string                   `json:"expected"`
	FreshnessHours        int                        `json:"freshness_hours"`
	ScopeRevision         int                        `json:"scope_revision"`
}
type Evidence struct {
	Document      *DocumentaryProvenance `json:"document,omitempty"`
	ObservationID string                 `json:"observation_id,omitempty"`
	Target        *observation.Target    `json:"target,omitempty"`
	ToolVersion   string                 `json:"tool_version,omitempty"`
	Source        string                 `json:"source"`
	URL           string                 `json:"url,omitempty"`
	Description   string                 `json:"description"`
	ScopeRevision int                    `json:"scope_revision"`
	ObservedAt    time.Time              `json:"observed_at"`
	CollectedAt   time.Time              `json:"collected_at"`
	Verification  string                 `json:"verification"`
	ArtifactKey   string                 `json:"artifact_key,omitempty"`
	Collector     string                 `json:"collector,omitempty"`
	Limitations   []string               `json:"limitations"`
}
type Draft struct {
	Explanation     string `json:"explanation"`
	Evidence        []Ref  `json:"evidence"`
	Measures        []Ref  `json:"measures"`
	ProfileRevision int    `json:"profile_revision"`
	ScopeRevision   int    `json:"scope_revision"`
}
type Review struct {
	Assessment Ref    `json:"assessment"`
	Notes      string `json:"notes"`
}
type Gap struct {
	Explanation string `json:"explanation"`
	Status      string `json:"status"`
}

func data(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
func record(service, kind, id, requirement string, v any) Record {
	return Record{ServiceID: service, Kind: kind, ID: id, RequirementID: requirement, Data: data(v)}
}
func Decode[T any](r Record) (T, error) { var v T; err := json.Unmarshal(r.Data, &v); return v, err }

func reference(ctx context.Context, q auth.Querier, service, kind string, ref Ref) (Record, error) {
	var r Record
	err := q.QueryRow(ctx, `SELECT service_id,kind,id,revision,requirement_id,actor_id,data,created_at FROM records WHERE service_id=$1 AND kind=$2 AND id=$3 AND revision=$4`, service, kind, ref.ID, ref.Revision).Scan(&r.ServiceID, &r.Kind, &r.ID, &r.Revision, &r.RequirementID, &r.ActorID, &r.Data, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Record{}, ErrInvalid
	}
	return r, err
}

func (s *Store) SaveScope(ctx context.Context, a auth.Actor, service string, expected int, v Scope) (Record, error) {
	if err := validateScope(v); err != nil {
		return Record{}, err
	}
	return s.mutate(ctx, a, record(service, "scope", "scope", "", v), expected, "edit", nil)
}

func validateScope(v Scope) error {
	if strings.TrimSpace(v.Description) == "" {
		return ErrInvalid
	}
	for _, account := range v.Accounts {
		if !regexp.MustCompile(`^\d{12}$`).MatchString(account.ID) || len(account.Regions) == 0 {
			return ErrInvalid
		}
		for _, region := range account.Regions {
			if !regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-\d+$`).MatchString(region) {
				return ErrInvalid
			}
		}
		if account.RoleARN != "" && !strings.Contains(account.RoleARN, ":"+account.ID+":role/") {
			return ErrInvalid
		}
	}
	for _, repo := range v.Repositories {
		if !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(repo.Name) || !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(repo.Commit) {
			return ErrInvalid
		}
		for _, path := range repo.Paths {
			if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, "..") {
				return ErrInvalid
			}
		}
	}
	return validateProviderScope(v)
}

func (s *Store) SaveProfile(ctx context.Context, a auth.Actor, service string, expected int, v catalog.Profile) (Record, error) {
	if err := v.Validate(); err != nil {
		return Record{}, ErrInvalid
	}
	return s.mutate(ctx, a, record(service, "profile", "profile", "", v), expected, "admin", func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM catalogs WHERE id=$1)`, v.CatalogID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrInvalid
		}
		return nil
	})
}

type ProfileChange struct {
	ID     string `json:"id"`
	Before string `json:"before"`
	After  string `json:"after"`
}

func (s *Store) PreviewProfile(ctx context.Context, a auth.Actor, service string, p catalog.Profile) ([]ProfileChange, error) {
	if err := auth.Authorize(ctx, s.DB, a, service, "read"); err != nil {
		return nil, err
	}
	all, err := catalog.Load(ctx, s.query(), p.CatalogID)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, ErrInvalid
	}
	next, err := catalog.Resolve(all, p)
	if err != nil {
		return nil, err
	}
	before := map[string]string{}
	current, err := s.Requirements(ctx, a, service)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	for _, r := range current {
		before[r.Requirement.ID] = r.Applicability + "|" + r.Statement
	}
	out := []ProfileChange{}
	for _, r := range next {
		after := r.Applicability + "|" + r.Statement
		if before[r.Requirement.ID] != after {
			out = append(out, ProfileChange{r.Requirement.ID, before[r.Requirement.ID], after})
		}
		delete(before, r.Requirement.ID)
	}
	for id, value := range before {
		out = append(out, ProfileChange{id, value, "removed"})
	}
	return out, nil
}

func (s *Store) Requirements(ctx context.Context, a auth.Actor, service string) ([]catalog.Resolved, error) {
	r, err := s.Get(ctx, a, service, "profile", "profile")
	if err != nil {
		return nil, err
	}
	p, err := Decode[catalog.Profile](r)
	if err != nil {
		return nil, err
	}
	all, err := catalog.Load(ctx, s.query(), p.CatalogID)
	if err != nil {
		return nil, err
	}
	return catalog.Resolve(all, p)
}

func requireRule(ctx context.Context, tx pgx.Tx, service, id string) error {
	r, err := latest(ctx, tx, service, "profile", "profile")
	if err != nil {
		return err
	}
	p, err := Decode[catalog.Profile](r)
	if err != nil {
		return err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM requirements WHERE catalog_id=$1 AND id=$2)`, p.CatalogID, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrInvalid
	}
	return nil
}

func (s *Store) SaveMeasure(ctx context.Context, a auth.Actor, service, id, requirement string, expected int, v Measure) (Record, error) {
	if v.Name == "" || v.Rationale == "" || v.FreshnessHours < 1 || v.FreshnessHours > 24*366 {
		return Record{}, ErrInvalid
	}
	return s.mutate(ctx, a, record(service, "measure", id, requirement, v), expected, "edit", func(tx pgx.Tx) error {
		if err := requireRule(ctx, tx, service, requirement); err != nil {
			return err
		}
		_, err := reference(ctx, tx, service, "scope", Ref{"scope", v.ScopeRevision})
		return err
	})
}

func externalURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Host != "" && u.Scheme == "https" && u.User == nil
}

func (s *Store) AttachEvidence(ctx context.Context, a auth.Actor, service, requirement, key string, v Evidence) (Record, error) {
	if err := auth.Authorize(ctx, s.DB, a, service, "edit"); err != nil {
		return Record{}, err
	}
	if v.Source == "" || v.Description == "" || !externalURL(v.URL) || v.ObservedAt.IsZero() || v.ObservedAt.After(time.Now().Add(time.Minute)) {
		return Record{}, ErrInvalid
	}
	// Caller-supplied provenance cannot manufacture collector observations.
	v.Verification = "unverified_reference"
	v.Document = nil
	v.ObservationID = ""
	v.Target = nil
	v.ToolVersion = ""
	v.Collector = ""
	v.ArtifactKey = ""
	v.CollectedAt = time.Time{}
	id := uuid.NewString()
	if key != "" {
		id = uuid.NewSHA1(uuid.NameSpaceURL, []byte(a.ID+"/"+service+"/evidence/"+key)).String()
	}
	r := record(service, "evidence", id, requirement, v)
	if old, err := s.Get(ctx, a, service, "evidence", id); err == nil {
		var prior Evidence
		if json.Unmarshal(old.Data, &prior) == nil && string(data(prior)) == string(data(v)) && old.RequirementID == requirement {
			return old, nil
		}
		return Record{}, ErrConflict
	}
	return s.mutate(ctx, a, r, 0, "edit", func(tx pgx.Tx) error {
		if err := requireRule(ctx, tx, service, requirement); err != nil {
			return err
		}
		_, err := reference(ctx, tx, service, "scope", Ref{"scope", v.ScopeRevision})
		return err
	})
}

func (s *Store) Artifact(ctx context.Context, a auth.Actor, service, id string) ([]byte, error) {
	r, err := s.Get(ctx, a, service, "evidence", id)
	if err != nil {
		return nil, err
	}
	v, err := Decode[Evidence](r)
	if err != nil {
		return nil, err
	}
	if v.ArtifactKey == "" || s.Artifacts == nil {
		return nil, ErrNotFound
	}
	return s.Artifacts.Get(ctx, v.ArtifactKey)
}

func (s *Store) SaveDraft(ctx context.Context, a auth.Actor, service, requirement string, expected int, v Draft) (Record, error) {
	if strings.TrimSpace(v.Explanation) == "" {
		return Record{}, ErrInvalid
	}
	return s.mutate(ctx, a, record(service, "assessment", requirement, requirement, v), expected, "edit", func(tx pgx.Tx) error {
		if err := requireRule(ctx, tx, service, requirement); err != nil {
			return err
		}
		for kind, revision := range map[string]int{"profile": v.ProfileRevision, "scope": v.ScopeRevision} {
			r, err := latest(ctx, tx, service, kind, kind)
			if err != nil {
				return err
			}
			if r.Revision != revision {
				return ErrConflict
			}
		}
		for kind, refs := range map[string][]Ref{"evidence": v.Evidence, "measure": v.Measures} {
			for _, ref := range refs {
				r, err := reference(ctx, tx, service, kind, ref)
				if err != nil {
					return err
				}
				if r.RequirementID != requirement {
					if kind != "evidence" {
						return ErrInvalid
					}
					var linked bool
					if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM rule_evidence_links WHERE service_id=$1 AND requirement_id=$2 AND evidence_id=$3 AND evidence_revision=$4)`, service, requirement, ref.ID, ref.Revision).Scan(&linked); err != nil {
						return err
					}
					if !linked {
						return ErrInvalid
					}
				}
				var scoped struct {
					ScopeRevision int `json:"scope_revision"`
				}
				if err = json.Unmarshal(r.Data, &scoped); err != nil {
					return err
				}
				if scoped.ScopeRevision != v.ScopeRevision {
					return ErrInvalid
				}
			}
		}
		return nil
	})
}

func (s *Store) Review(ctx context.Context, a auth.Actor, service, requirement string, v Review) (Record, error) {
	if v.Assessment.ID != requirement {
		return Record{}, ErrInvalid
	}
	return s.mutate(ctx, a, record(service, "review", uuid.NewString(), requirement, v), 0, "review", func(tx pgx.Tx) error {
		r, err := latest(ctx, tx, service, "assessment", requirement)
		if err != nil {
			return err
		}
		if r.Revision != v.Assessment.Revision {
			return ErrConflict
		}
		return nil
	})
}

func (s *Store) SaveGap(ctx context.Context, a auth.Actor, service, id, requirement string, expected int, v Gap) (Record, error) {
	if v.Explanation == "" || (v.Status != "open" && v.Status != "resolved") {
		return Record{}, ErrInvalid
	}
	return s.mutate(ctx, a, record(service, "gap", id, requirement, v), expected, "edit", func(tx pgx.Tx) error { return requireRule(ctx, tx, service, requirement) })
}

type Status struct {
	Work          EvidenceWork      `json:"evidence_work"`
	Automation    string            `json:"automation"`
	Collection    ObservationStatus `json:"collection"`
	RequirementID string            `json:"requirement_id"`
	Technical     string            `json:"technical"`
	Evidence      string            `json:"evidence"`
	Review        string            `json:"review"`
	Draft         *Record           `json:"draft,omitempty"`
	Reasons       []string          `json:"reasons"`
}

func (s *Store) Status(ctx context.Context, a auth.Actor, service, requirement string, now time.Time) (out Status, err error) {
	defer func() {
		if out.Collection.Count > 0 {
			if out.Draft == nil {
				out.Evidence = out.Collection.Freshness
			}
			out.Technical = out.Collection.Technical
		}
	}()
	x := Status{RequirementID: requirement, Technical: "unknown", Evidence: "missing", Review: "pending", Reasons: []string{}}
	x.Work, err = s.WorkStatus(ctx, a, service, requirement, now)
	if err != nil {
		return x, err
	}
	r, err := s.Get(ctx, a, service, "assessment", requirement)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return x, err
	}
	missingDraft := errors.Is(err, ErrNotFound)
	x.Technical, err = s.technical(ctx, service, requirement)
	if err != nil {
		return x, err
	}
	x.Collection, err = s.observationStatus(ctx, service, requirement, now)
	if err != nil {
		return x, err
	}
	x.Automation = "awaiting_collection_plan"
	var planErr error
	if s.presentationCache != nil {
		if v := s.presentationCache.automation[requirement]; v != "" {
			x.Automation = v
		}
	} else {
		planErr = s.query().QueryRow(ctx, `SELECT data->>'state' FROM collection_entries WHERE service_id=$1 AND data->>'requirement_id'=$2 AND plan_revision=(SELECT max(revision) FROM collection_plans WHERE service_id=$1) ORDER BY id LIMIT 1`, service, requirement).Scan(&x.Automation)
	}
	if planErr != nil && !errors.Is(planErr, pgx.ErrNoRows) {
		return x, planErr
	}
	for _, reason := range x.Collection.Errors {
		if strings.Contains(reason, "credential") {
			x.Automation = "blocked_credentials"
		} else if strings.Contains(reason, "binding") {
			x.Automation = "blocked_binding"
		}
	}
	if x.Collection.Count > 0 {
		x.Evidence = x.Collection.Freshness
		x.Technical = x.Collection.Technical
	}
	if missingDraft {
		return x, nil
	}
	x.Draft = &r
	draft, err := Decode[Draft](r)
	if err != nil {
		return x, err
	}
	changed := false
	for kind, revision := range map[string]int{"profile": draft.ProfileRevision, "scope": draft.ScopeRevision} {
		current, err := s.Get(ctx, a, service, kind, kind)
		if err != nil {
			return x, err
		}
		if current.Revision != revision {
			changed = true
			x.Reasons = append(x.Reasons, kind+" changed")
		}
	}
	if len(draft.Evidence) > 0 {
		x.Evidence = "current"
	}
	for _, ref := range draft.Measures {
		current, err := s.Get(ctx, a, service, "measure", ref.ID)
		if err != nil {
			return x, err
		}
		if current.Revision != ref.Revision {
			changed = true
			x.Reasons = append(x.Reasons, "measure changed")
		}
		mr, err := s.presentationReference(ctx, service, "measure", ref)
		if err != nil {
			return x, err
		}
		m, err := Decode[Measure](mr)
		if err != nil {
			return x, err
		}
		for _, e := range draft.Evidence {
			er, err := s.presentationReference(ctx, service, "evidence", e)
			if err != nil {
				return x, err
			}
			ev, err := Decode[Evidence](er)
			if err != nil {
				return x, err
			}
			if ev.Document == nil && ev.ObservedAt.Add(time.Duration(m.FreshnessHours)*time.Hour).Before(now) {
				x.Evidence = "stale"
				changed = true
			}
		}
	}
	for _, ref := range draft.Evidence {
		er, e := s.presentationReference(ctx, service, "evidence", ref)
		if e != nil {
			return x, e
		}
		ev, e := Decode[Evidence](er)
		if e != nil {
			return x, e
		}
		if ev.Document != nil {
			if ev.ObservedAt.Add(time.Duration(ev.Document.FreshnessHours) * time.Hour).Before(now) {
				x.Evidence = "stale"
				changed = true
			}
			source, e := s.Get(ctx, a, service, "evidence_source", ev.Document.Source.ID)
			if e != nil {
				return x, e
			}
			if source.Revision != ev.Document.Source.Revision {
				changed = true
				x.Evidence = "stale"
				x.Reasons = append(x.Reasons, "evidence source changed")
			}
		} else if len(draft.Measures) == 0 {
			x.Reasons = append(x.Reasons, "No measure freshness policy; evidence freshness not established")
			x.Evidence = "missing"
		}
	}
	var failureTime *time.Time
	if s.presentationCache != nil {
		failureTime = s.presentationCache.failures[requirement]
	} else {
		err = s.query().QueryRow(ctx, `SELECT max(updated_at) FROM jobs WHERE service_id=$1 AND requirement_id=$2 AND (result->>'outcome' IN ('fail','error','unknown') OR state IN ('failed','timed-out'))`, service, requirement).Scan(&failureTime)
	}
	if err != nil {
		return x, err
	}
	if changed && x.Technical == "pass" {
		x.Technical = "unknown"
	}
	var reviewData []byte
	var reviewedAt time.Time
	if s.presentationCache != nil {
		if review, ok := s.presentationCache.reviews[requirement]; ok {
			reviewData = review.Data
			reviewedAt = review.CreatedAt
		} else {
			err = pgx.ErrNoRows
		}
	} else {
		err = s.query().QueryRow(ctx, `SELECT data,created_at FROM records WHERE service_id=$1 AND kind='review' AND requirement_id=$2 ORDER BY created_at DESC LIMIT 1`, service, requirement).Scan(&reviewData, &reviewedAt)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return x, nil
	}
	if err != nil {
		return x, err
	}
	var review Review
	if err = json.Unmarshal(reviewData, &review); err != nil {
		return x, err
	}
	x.Review = "reviewed"
	if changed || x.Evidence != "current" || review.Assessment.Revision != r.Revision || (failureTime != nil && failureTime.After(reviewedAt)) {
		x.Review = "review-needed"
	}
	var newFailure bool
	if s.presentationCache != nil {
		newFailure = s.presentationCache.findingTimes[requirement].After(reviewedAt)
	} else {
		err = s.query().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM finding_events f JOIN observations o ON o.service_id=f.service_id AND o.id=f.observation_id JOIN observation_links l ON l.service_id=o.service_id AND l.observation_id=o.id WHERE f.service_id=$1 AND l.requirement_id=$2 AND f.outcome='fail' AND o.received_at>$3)`, service, requirement, reviewedAt).Scan(&newFailure)
	}
	if err != nil {
		return x, err
	}
	if newFailure {
		x.Review = "review-needed"
	}
	return x, nil
}

// Aggregate the latest run of each check/measure, so one passing check cannot
// hide another check's failure. Pending and failed collection remain visible.
func (s *Store) technical(ctx context.Context, service, requirement string) (string, error) {
	if s.presentationCache != nil {
		if value := s.presentationCache.technical[requirement]; value != "" {
			return value, nil
		}
		return "unknown", nil
	}
	rows, err := s.query().Query(ctx, `SELECT state,coalesce(result->>'outcome','unknown') FROM (SELECT DISTINCT ON(check_id,input->'measure'->>'id') state,result,check_id,input,created_at FROM jobs WHERE service_id=$1 AND requirement_id=$2
AND (NOT input ? 'scope_revision' OR (input->>'scope_revision')::int=(SELECT max(revision) FROM records WHERE service_id=$1 AND kind='scope' AND id='scope'))
AND (coalesce((input->>'profile_revision')::int,0)=0 OR (input->>'profile_revision')::int=(SELECT max(revision) FROM records WHERE service_id=$1 AND kind='profile' AND id='profile'))
AND (coalesce((input->>'configuration_revision')::int,0)=0 OR (input->>'configuration_revision')::int=(SELECT revision FROM collection_schedules WHERE service_id=$1))
ORDER BY check_id,input->'measure'->>'id',created_at DESC) latest`, service, requirement)
	if err != nil {
		return "unknown", err
	}
	defer rows.Close()
	result := "unknown"
	rank := 0
	for rows.Next() {
		var state, outcome string
		if err = rows.Scan(&state, &outcome); err != nil {
			return "unknown", err
		}
		if state == "failed" || state == "timed-out" {
			outcome = "error"
		}
		value := map[string]int{"pass": 1, "unknown": 2, "fail": 3, "error": 4}[outcome]
		if value == 0 {
			outcome = "unknown"
			value = 2
		}
		if value > rank {
			rank = value
			result = outcome
		}
	}
	return result, rows.Err()
}
