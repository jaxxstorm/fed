package assessment

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/observation"
	"github.com/jaxxstorm/fed/internal/storage"
)

// IngestObservation accepts caller-observed data, never server collector provenance.
func (s *Store) IngestObservation(ctx context.Context, a auth.Actor, in observation.Submission) (observation.Receipt, error) {
	if err := auth.Authorize(ctx, s.DB, a, in.ServiceID, "edit"); err != nil {
		return observation.Receipt{}, err
	}
	if err := in.Validate(time.Now()); err != nil {
		return observation.Receipt{}, err
	}
	if s.Artifacts == nil {
		return observation.Receipt{}, errors.New("artifact_storage_unavailable")
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return observation.Receipt{}, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, in.ServiceID, "edit"); err != nil {
		return observation.Receipt{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, in.ServiceID); err != nil {
		return observation.Receipt{}, err
	}
	// Re-encoding through interface normalizes raw JSON key ordering before hashing.
	var canonical any
	encoded, err := json.Marshal(in)
	if err != nil {
		return observation.Receipt{}, observation.ErrInvalid
	}
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.UseNumber()
	if err = decoder.Decode(&canonical); err != nil {
		return observation.Receipt{}, observation.ErrInvalid
	}
	encoded, err = json.Marshal(canonical)
	if err != nil {
		return observation.Receipt{}, observation.ErrInvalid
	}
	digest := storage.Hash(encoded)
	var oldHash string
	var old []byte
	err = tx.QueryRow(ctx, `SELECT request_hash,response FROM observation_ingestions WHERE service_id=$1 AND actor_id=$2 AND operation='agent' AND key=$3`, in.ServiceID, a.ID, in.IdempotencyKey).Scan(&oldHash, &old)
	if err == nil {
		if oldHash != digest {
			return observation.Receipt{}, ErrConflict
		}
		var out observation.Receipt
		err = json.Unmarshal(old, &out)
		return out, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return observation.Receipt{}, err
	}
	scopeRecord, err := reference(ctx, tx, in.ServiceID, "scope", Ref{"scope", in.ScopeRevision})
	if err != nil {
		return observation.Receipt{}, err
	}
	scope, err := Decode[Scope](scopeRecord)
	if err != nil {
		return observation.Receipt{}, err
	}
	if err = validateObservationTarget(scope, in.Target); err != nil {
		return observation.Receipt{}, err
	}
	profileRecord, err := reference(ctx, tx, in.ServiceID, "profile", Ref{"profile", in.ProfileRevision})
	if err != nil {
		return observation.Receipt{}, err
	}
	profile, err := Decode[catalog.Profile](profileRecord)
	if err != nil {
		return observation.Receipt{}, err
	}
	for _, link := range in.Links {
		mr, err := reference(ctx, tx, in.ServiceID, "measure", Ref{link.Measure.ID, link.Measure.Revision})
		if err != nil {
			return observation.Receipt{}, err
		}
		m, err := Decode[Measure](mr)
		if err != nil {
			return observation.Receipt{}, err
		}
		if mr.RequirementID != link.RequirementID || m.ScopeRevision != in.ScopeRevision {
			return observation.Receipt{}, observation.ErrScope
		}
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM requirements WHERE catalog_id=$1 AND id=$2)`, profile.CatalogID, link.RequirementID).Scan(&exists); err != nil {
			return observation.Receipt{}, err
		}
		if !exists {
			return observation.Receipt{}, ErrInvalid
		}
	}
	artifact, err := json.Marshal(in.Artifact)
	if err != nil {
		return observation.Receipt{}, err
	}
	key, err := s.Artifacts.Put(ctx, artifact)
	if err != nil {
		return observation.Receipt{}, err
	}
	out, err := persistObservation(ctx, tx, a, in, key, "agent_observation", digest)
	if err != nil {
		return observation.Receipt{}, err
	}
	return out, tx.Commit(ctx)
}

func persistObservation(ctx context.Context, tx pgx.Tx, a auth.Actor, in observation.Submission, key, source, digest string) (observation.Receipt, error) {
	out := observation.Receipt{ID: uuid.NewString(), ArtifactHash: key, SourceKind: source, ActorID: a.ID, Evidence: []observation.Ref{}, Findings: []observation.Finding{}}
	targetBytes, _ := json.Marshal(in.Target)
	targetKey := storage.Hash(targetBytes)
	err := tx.QueryRow(ctx, `INSERT INTO observations(service_id,id,scope_revision,profile_revision,actor_id,source_kind,artifact_hash,observed_at,target_key,data) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING received_at`, in.ServiceID, out.ID, in.ScopeRevision, in.ProfileRevision, a.ID, out.SourceKind, key, in.ObservedAt, targetKey, data(in)).Scan(&out.ReceivedAt)
	if err != nil {
		return observation.Receipt{}, err
	}
	for _, link := range in.Links {
		id := uuid.NewString()
		e := Evidence{Source: "agent", Description: in.Method + " observation", ScopeRevision: in.ScopeRevision, ObservedAt: in.ObservedAt, CollectedAt: out.ReceivedAt, Verification: out.SourceKind, ArtifactKey: key, Limitations: append(append([]string{}, in.Limitations...), "Agent-supplied observation; method and evaluations are declarations, not server collection.")}
		e.ObservationID = out.ID
		e.Target = &in.Target
		e.ToolVersion = in.ToolVersion
		if source == "collector_observation" {
			e.Source = "collector"
			e.Collector = in.Method + "@" + in.ToolVersion
			e.Limitations = in.Limitations
		}
		if _, err = tx.Exec(ctx, `INSERT INTO records(service_id,kind,id,revision,requirement_id,actor_id,data) VALUES($1,'evidence',$2,1,$3,$4,$5)`, in.ServiceID, id, link.RequirementID, a.ID, data(e)); err != nil {
			return observation.Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO observation_links(service_id,observation_id,requirement_id,measure_id,measure_revision,evidence_id,data) VALUES($1,$2,$3,$4,$5,$6,$7)`, in.ServiceID, out.ID, link.RequirementID, link.Measure.ID, link.Measure.Revision, id, data(link)); err != nil {
			return observation.Receipt{}, err
		}
		out.Evidence = append(out.Evidence, observation.Ref{ID: id, Revision: 1})
	}
	for _, evaluation := range in.Evaluations {
		if evaluation.Outcome != "pass" && evaluation.Outcome != "fail" {
			continue
		}
		// Non-failing partial observations cannot resolve a condition.
		if evaluation.Outcome == "pass" && (!in.Coverage.Complete || in.Coverage.State != "observed") {
			continue
		}
		id := storage.Hash([]byte(in.ServiceID + "/" + targetKey + "/" + in.Artifact.Schema + "/" + evaluation.Condition + "/" + source))
		finding := observation.Finding{ID: id, Condition: evaluation.Condition, Outcome: evaluation.Outcome, Explanation: evaluation.Explanation, Basis: "agent_assertion"}
		finding.Target = in.Target
		finding.ObservationID = out.ID
		finding.ObservedAt = in.ObservedAt
		if source == "collector_observation" {
			finding.Basis = "collector_evaluation"
		}
		if _, err = tx.Exec(ctx, `INSERT INTO finding_events(service_id,finding_id,observation_id,condition,outcome,data) VALUES($1,$2,$3,$4,$5,$6)`, in.ServiceID, id, out.ID, evaluation.Condition, evaluation.Outcome, data(finding)); err != nil {
			return observation.Receipt{}, err
		}
		out.Findings = append(out.Findings, finding)
	}
	operation := "agent"
	if source == "collector_observation" {
		operation = "collector"
	}
	if _, err = tx.Exec(ctx, `INSERT INTO observation_ingestions(service_id,actor_id,operation,key,request_hash,observation_id,response) VALUES($1,$2,$3,$4,$5,$6,$7)`, in.ServiceID, a.ID, operation, in.IdempotencyKey, digest, out.ID, data(out)); err != nil {
		return observation.Receipt{}, err
	}
	if err = audit(ctx, tx, a, in.ServiceID, "observation.ingested", out.ID, 1); err != nil {
		return observation.Receipt{}, err
	}
	return out, nil
}

// StageCollected is for the runner only: a current job lease authorizes the
// source classification, and the caller commits observation and job state together.
func StageCollected(ctx context.Context, tx pgx.Tx, a auth.Actor, job, lease, key string, in observation.Submission) (observation.Receipt, error) {
	if err := auth.Authorize(ctx, tx, a, in.ServiceID, "check"); err != nil {
		return observation.Receipt{}, err
	}
	var valid bool
	err := tx.QueryRow(ctx, `SELECT actor_id=$4 AND lease_id=$3 AND state='running' AND lease_until>now() AND (input->>'scope_revision')::int=$5 AND (input->>'profile_revision')::int=$6 FROM jobs WHERE id=$1 AND service_id=$2`, job, in.ServiceID, lease, a.ID, in.ScopeRevision, in.ProfileRevision).Scan(&valid)
	if err != nil {
		return observation.Receipt{}, err
	}
	if !valid {
		return observation.Receipt{}, ErrConflict
	}
	return persistObservation(ctx, tx, a, in, key, "collector_observation", storage.Hash(data(in)))
}

func validateObservationTarget(scope Scope, t observation.Target) error {
	switch t.Kind {
	case "aws", "kubernetes":
		if t.Kind == "kubernetes" && t.ClusterID != "" {
			if t.AccountID != "" || t.Region != "" || t.Namespace == "" || t.NamespaceUID == "" || t.UID == "" || t.ResourceID == "" || t.ResourceKind == "" {
				return observation.ErrScope
			}
			for _, cluster := range scope.Kubernetes {
				if cluster.ClusterID != t.ClusterID {
					continue
				}
				if cluster.ClusterWide {
					return nil
				}
				for _, namespace := range cluster.Namespaces {
					if namespace.Name == t.Namespace && namespace.UID == t.NamespaceUID {
						return nil
					}
				}
			}
			return observation.ErrScope
		}
		for _, account := range scope.Accounts {
			if account.ID != t.AccountID || !slices.Contains(account.Regions, t.Region) {
				continue
			}
			if t.ResourceID == "" {
				return observation.ErrScope
			}
			if t.Kind == "aws" {
				arn := strings.SplitN(t.ResourceID, ":", 6)
				if len(arn) != 6 || arn[0] != "arn" || arn[3] != t.Region || arn[4] != t.AccountID {
					return observation.ErrScope
				}
			} else if t.Cluster == "" || !slices.Contains(account.Clusters, t.Cluster) || t.Namespace == "" || t.ResourceKind == "" || t.UID == "" {
				return observation.ErrScope
			}
			return nil
		}
	case "repository":
		for _, repo := range scope.Repositories {
			if t.Repository == repo.Name && t.Commit == repo.Commit && slices.Contains(repo.Paths, t.Path) && slices.Contains([]string{"committed", "dirty"}, t.Worktree) {
				return nil
			}
		}
	}
	return observation.ErrScope
}
