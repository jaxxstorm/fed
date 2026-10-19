package readiness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/storage"
)

type WorkBinding struct {
	CatalogID       string `json:"catalog_id"`
	ReleaseRevision int    `json:"release_revision"`
	ReleaseHash     string `json:"release_hash"`
	ProfileRevision int    `json:"profile_revision"`
	ScopeRevision   int    `json:"scope_revision"`
}

// ScopeKey uses saved boundary identifiers, not display names or access status.
// Kubernetes namespaces bind their UID to avoid reuse after namespace recreation.
type ScopeKey struct {
	Kind         string `json:"kind"`
	AccountID    string `json:"account_id,omitempty"`
	Region       string `json:"region,omitempty"`
	ClusterID    string `json:"cluster_id,omitempty"`
	NamespaceUID string `json:"namespace_uid,omitempty"`
	Repository   string `json:"repository,omitempty"`
}

type EvidenceRef struct {
	ID       string `json:"id"`
	Revision int    `json:"revision"`
}

type WorkData struct {
	Duty           *DutyRef          `json:"duty,omitempty"`
	CapabilityID   string            `json:"capability_id"`
	Scope          ScopeKey          `json:"scope"`
	OwnerRole      string            `json:"owner_role"`
	AssigneeID     string            `json:"assignee_id,omitempty"`
	Implementation string            `json:"implementation"`
	AlternativeID  string            `json:"alternative_id,omitempty"`
	Decisions      map[string]string `json:"decisions"`
	Trigger        string            `json:"trigger"`
	Evidence       []EvidenceRef     `json:"evidence"`
	Notes          string            `json:"notes"`
}

type DutyRef struct {
	PlaybookID string `json:"playbook_id"`
	StepID     string `json:"step_id"`
}

func WorkInstanceID(service string, data WorkData) string {
	if data.Duty == nil {
		return InstanceID(service, data.CapabilityID, data.Scope)
	}
	raw, _ := json.Marshal(struct {
		Service string
		Duty    DutyRef
		Scope   ScopeKey
	}{service, *data.Duty, data.Scope})
	return storage.Hash(raw)
}

type WorkChange struct {
	ServiceID        string      `json:"service_id"`
	IdempotencyKey   string      `json:"idempotency_key"`
	ExpectedRevision int         `json:"expected_revision"`
	Binding          WorkBinding `json:"binding"`
	Data             WorkData    `json:"data"`
}

type WorkRecord struct {
	ID        string      `json:"id"`
	ServiceID string      `json:"service_id"`
	Revision  int         `json:"revision"`
	ActorID   string      `json:"actor_id"`
	CreatedAt time.Time   `json:"created_at"`
	Binding   WorkBinding `json:"binding"`
	Data      WorkData    `json:"data"`
}

func InstanceID(service, capability string, scope ScopeKey) string {
	raw, _ := json.Marshal(struct {
		Service    string
		Capability string
		Scope      ScopeKey
	}{service, capability, scope})
	return storage.Hash(raw)
}

// MatchScope requires an exact canonical identity in the current saved scope.
// It never expands a namespace or repository selection just to reuse work.
func MatchScope(key ScopeKey, raw []byte) bool {
	var scope struct {
		Accounts []struct {
			ID       string
			Regions  []string
			Clusters []string
		}
		Kubernetes []struct {
			ClusterID   string `json:"cluster_id"`
			ClusterWide bool   `json:"cluster_wide"`
			Namespaces  []struct{ UID string }
		}
		Repositories []struct{ Name string }
	}
	if json.Unmarshal(raw, &scope) != nil {
		return false
	}
	valid := func(candidate ScopeKey) bool { return key == candidate }
	if valid(ScopeKey{Kind: "service"}) {
		return true
	}
	for _, a := range scope.Accounts {
		for _, r := range a.Regions {
			if valid(ScopeKey{Kind: "account", AccountID: a.ID, Region: r}) {
				return true
			}
			for _, c := range a.Clusters {
				if valid(ScopeKey{Kind: "cluster", AccountID: a.ID, Region: r, ClusterID: c}) {
					return true
				}
			}
		}
	}
	for _, c := range scope.Kubernetes {
		// A namespace-limited grant does not imply cluster-wide scope.
		if c.ClusterWide && valid(ScopeKey{Kind: "cluster", ClusterID: c.ClusterID}) {
			return true
		}
		for _, n := range c.Namespaces {
			if n.UID != "" && valid(ScopeKey{Kind: "namespace", ClusterID: c.ClusterID, NamespaceUID: n.UID}) {
				return true
			}
		}
	}
	for _, r := range scope.Repositories {
		if valid(ScopeKey{Kind: "repository", Repository: r.Name}) {
			return true
		}
	}
	return false
}

func (s *Store) SaveWork(ctx context.Context, a auth.Actor, in WorkChange) (WorkRecord, error) {
	if in.ServiceID == "" || strings.TrimSpace(in.IdempotencyKey) == "" || len(in.IdempotencyKey) > 200 || in.ExpectedRevision < 0 || (in.Data.CapabilityID == "") == (in.Data.Duty == nil) || strings.TrimSpace(in.Data.OwnerRole) == "" || !slices.Contains([]string{"unknown", "declared_missing", "declared_present"}, in.Data.Implementation) || !slices.Contains([]string{"unknown", "not_triggered", "due", "observed"}, in.Data.Trigger) {
		return WorkRecord{}, ErrInvalid
	}
	for k, v := range in.Data.Decisions {
		if !strings.HasPrefix(k, "decision.") || len(k) <= len("decision.") || strings.TrimSpace(v) == "" {
			return WorkRecord{}, ErrInvalid
		}
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return WorkRecord{}, err
	}
	if len(raw) > 256*1024 {
		return WorkRecord{}, fmt.Errorf("%w: work record exceeds 256 KiB", ErrInvalid)
	}
	requestHash := storage.Hash(raw)
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return WorkRecord{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, in.ServiceID, "edit"); err != nil {
		return WorkRecord{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, in.ServiceID); err != nil {
		return WorkRecord{}, err
	}
	var priorHash string
	var prior []byte
	err = tx.QueryRow(ctx, `SELECT request_hash,response FROM readiness_work_requests WHERE service_id=$1 AND actor_id=$2 AND idempotency_key=$3`, in.ServiceID, a.ID, in.IdempotencyKey).Scan(&priorHash, &prior)
	if err == nil {
		if priorHash != requestHash {
			return WorkRecord{}, ErrConflict
		}
		var out WorkRecord
		if err = json.Unmarshal(prior, &out); err != nil {
			return out, err
		}
		return out, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return WorkRecord{}, err
	}
	var scopeRaw, profileRaw []byte
	var scopeRevision, profileRevision int
	if err = tx.QueryRow(ctx, `SELECT revision,data FROM records WHERE service_id=$1 AND kind='scope' AND id='scope' ORDER BY revision DESC LIMIT 1`, in.ServiceID).Scan(&scopeRevision, &scopeRaw); err != nil {
		return WorkRecord{}, err
	}
	if err = tx.QueryRow(ctx, `SELECT revision,data FROM records WHERE service_id=$1 AND kind='profile' AND id='profile' ORDER BY revision DESC LIMIT 1`, in.ServiceID).Scan(&profileRevision, &profileRaw); err != nil {
		return WorkRecord{}, err
	}
	var profile catalog.Profile
	if err = json.Unmarshal(profileRaw, &profile); err != nil {
		return WorkRecord{}, err
	}
	if scopeRevision != in.Binding.ScopeRevision || profileRevision != in.Binding.ProfileRevision || profile.CatalogID != in.Binding.CatalogID {
		return WorkRecord{}, ErrConflict
	}
	if !MatchScope(in.Data.Scope, scopeRaw) {
		return WorkRecord{}, fmt.Errorf("%w: capability instance outside saved scope", ErrInvalid)
	}
	var release []byte
	var revision int
	var hash string
	if err = tx.QueryRow(ctx, `SELECT revision,content_hash,payload FROM readiness_service_releases WHERE organization_id=$1 AND service_id=$2 AND catalog_id=$3 ORDER BY revision DESC LIMIT 1`, a.OrganizationID, in.ServiceID, in.Binding.CatalogID).Scan(&revision, &hash, &release); err != nil {
		return WorkRecord{}, err
	}
	if revision != in.Binding.ReleaseRevision || hash != in.Binding.ReleaseHash {
		return WorkRecord{}, ErrConflict
	}
	var p Package
	if err = json.Unmarshal(release, &p); err != nil {
		return WorkRecord{}, err
	}
	if s.Rules == nil || s.Markdown == nil {
		return WorkRecord{}, ErrReviewRequired
	}
	if err = VerifySources(p, s.Rules, s.Markdown); err != nil {
		return WorkRecord{}, ErrReviewRequired
	}
	found := false
	if in.Data.Duty != nil {
		if in.Data.Scope != (ScopeKey{Kind: "service"}) || in.Data.AlternativeID != "" {
			return WorkRecord{}, ErrInvalid
		}
		all, e := catalog.Load(ctx, tx, profile.CatalogID)
		if e != nil {
			return WorkRecord{}, e
		}
		resolved, e := catalog.Resolve(all, profile)
		if e != nil {
			return WorkRecord{}, e
		}
		variants := map[string]bool{}
		for _, r := range resolved {
			v, e := catalog.Variant(r)
			if e != nil {
				return WorkRecord{}, e
			}
			variants[v.ID] = true
		}
		for _, book := range p.Playbooks {
			if book.ID != in.Data.Duty.PlaybookID || book.Kind != "actionable" {
				continue
			}
			applies := false
			for _, id := range book.VariantIDs {
				applies = applies || variants[id]
			}
			if !applies {
				continue
			}
			for _, step := range book.Operate.Items {
				if step.ID != in.Data.Duty.StepID {
					continue
				}
				found = true
				if in.Data.Trigger == "not_triggered" && step.Trigger != "event" {
					return WorkRecord{}, fmt.Errorf("%w: not-triggered is only valid for event duties", ErrInvalid)
				}
			}
		}
	}
	for _, c := range p.Capabilities {
		if in.Data.Duty != nil {
			break
		}
		if c.ID != in.Data.CapabilityID {
			continue
		}
		found = true
		if c.ScopeKind != in.Data.Scope.Kind {
			return WorkRecord{}, ErrInvalid
		}
		if in.Data.AlternativeID != "" {
			valid := false
			for _, alt := range c.Alternatives {
				if alt.ID == in.Data.AlternativeID {
					valid = true
				}
			}
			if !valid {
				return WorkRecord{}, fmt.Errorf("%w: unknown implementation alternative", ErrInvalid)
			}
		}
	}
	if !found {
		return WorkRecord{}, fmt.Errorf("%w: unknown capability", ErrInvalid)
	}
	if in.Data.AssigneeID != "" {
		var org, kind string
		if err = tx.QueryRow(ctx, `SELECT organization_id,kind FROM identities WHERE id=$1`, in.Data.AssigneeID).Scan(&org, &kind); err != nil || org != a.OrganizationID {
			return WorkRecord{}, fmt.Errorf("%w: ineligible assignee", ErrInvalid)
		}
		assignee := auth.Actor{ID: in.Data.AssigneeID, OrganizationID: org, Kind: kind}
		if err = auth.Authorize(ctx, tx, assignee, in.ServiceID, "edit"); err != nil {
			return WorkRecord{}, fmt.Errorf("%w: assignee cannot edit service", ErrInvalid)
		}
		if kind == "agent" {
			var active bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM api_tokens WHERE identity_id=$1 AND revoked_at IS NULL AND expires_at>now() AND $2=ANY(service_ids) AND 'edit'=ANY(actions))`, assignee.ID, in.ServiceID).Scan(&active); err != nil {
				return WorkRecord{}, err
			}
			if !active {
				return WorkRecord{}, fmt.Errorf("%w: agent assignee has no active service grant", ErrInvalid)
			}
		}
	}
	seen := map[EvidenceRef]bool{}
	for _, ref := range in.Data.Evidence {
		if ref.ID == "" || ref.Revision < 1 || seen[ref] {
			return WorkRecord{}, ErrInvalid
		}
		seen[ref] = true
		var evidence []byte
		if err = tx.QueryRow(ctx, `SELECT data FROM records WHERE service_id=$1 AND kind='evidence' AND id=$2 AND revision=$3`, in.ServiceID, ref.ID, ref.Revision).Scan(&evidence); err != nil {
			return WorkRecord{}, fmt.Errorf("%w: evidence reference unavailable", ErrInvalid)
		}
		var e struct {
			ScopeRevision int `json:"scope_revision"`
		}
		if err = json.Unmarshal(evidence, &e); err != nil {
			return WorkRecord{}, err
		}
		if e.ScopeRevision != scopeRevision {
			return WorkRecord{}, fmt.Errorf("%w: evidence belongs to a different scope revision", ErrInvalid)
		}
	}
	id := WorkInstanceID(in.ServiceID, in.Data)
	var current int
	if err = tx.QueryRow(ctx, `SELECT COALESCE(max(revision),0) FROM readiness_work WHERE service_id=$1 AND id=$2`, in.ServiceID, id).Scan(&current); err != nil {
		return WorkRecord{}, err
	}
	if current != in.ExpectedRevision {
		return WorkRecord{}, ErrConflict
	}
	out := WorkRecord{ID: id, ServiceID: in.ServiceID, Revision: current + 1, ActorID: a.ID, Binding: in.Binding, Data: in.Data}
	binding, _ := json.Marshal(in.Binding)
	data, _ := json.Marshal(in.Data)
	if err = tx.QueryRow(ctx, `INSERT INTO readiness_work(service_id,id,revision,actor_id,binding,data) VALUES($1,$2,$3,$4,$5,$6) RETURNING created_at`, in.ServiceID, id, out.Revision, a.ID, binding, data).Scan(&out.CreatedAt); err != nil {
		return out, err
	}
	response, err := json.Marshal(out)
	if err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO readiness_work_requests(service_id,actor_id,idempotency_key,request_hash,response) VALUES($1,$2,$3,$4,$5)`, in.ServiceID, a.ID, in.IdempotencyKey, requestHash, response); err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,service_id,actor_id,action,target,revision) VALUES($1,$2,$3,'readiness.work.saved',$4,$5)`, a.OrganizationID, in.ServiceID, a.ID, id, out.Revision); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

func (s *Store) Work(ctx context.Context, a auth.Actor, service string, limit, offset int) ([]WorkRecord, error) {
	if err := auth.Authorize(ctx, s.DB, a, service, "read"); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, ErrInvalid
	}
	rows, err := s.DB.Query(ctx, `SELECT id,revision,actor_id,created_at,binding,data FROM (SELECT DISTINCT ON(id) * FROM readiness_work WHERE service_id=$1 ORDER BY id,revision DESC) latest ORDER BY id LIMIT $2 OFFSET $3`, service, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WorkRecord{}
	for rows.Next() {
		r := WorkRecord{ServiceID: service}
		var binding, data []byte
		if err = rows.Scan(&r.ID, &r.Revision, &r.ActorID, &r.CreatedAt, &binding, &data); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(binding, &r.Binding); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(data, &r.Data); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
