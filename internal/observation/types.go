// Package observation defines the versioned, transport-independent evidence contract.
package observation

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

const SchemaVersion = "observation/v1"
const MaxArtifactBytes = 1 << 20
const FutureTolerance = time.Minute

var ErrTooLarge = errors.New("artifact_too_large")
var ErrScope = errors.New("out_of_scope")
var ErrInvalid = errors.New("invalid_observation")

type Ref struct {
	ID       string `json:"id"`
	Revision int    `json:"revision"`
}
type Target struct {
	ClusterID    string `json:"cluster_id,omitempty"`
	NamespaceUID string `json:"namespace_uid,omitempty"`
	Kind         string `json:"kind"`
	AccountID    string `json:"account_id,omitempty"`
	Region       string `json:"region,omitempty"`
	ResourceID   string `json:"resource_id,omitempty"`
	Cluster      string `json:"cluster,omitempty"`
	Namespace    string `json:"namespace,omitempty"`
	ResourceKind string `json:"resource_kind,omitempty"`
	UID          string `json:"uid,omitempty"`
	Repository   string `json:"repository,omitempty"`
	Commit       string `json:"commit,omitempty"`
	Path         string `json:"path,omitempty"`
	Worktree     string `json:"worktree,omitempty"`
}

type Coverage struct {
	Recovery *Recovery `json:"recovery,omitempty"`
	Expected int       `json:"expected"`
	Observed int       `json:"observed"`
	Complete bool      `json:"complete"`
	State    string    `json:"state"`
	Error    string    `json:"error,omitempty"`
}
type Evaluation struct {
	Condition   string `json:"condition"`
	Outcome     string `json:"outcome"`
	Explanation string `json:"explanation"`
}
type Link struct {
	Conditions     []string `json:"conditions,omitempty"`
	RequirementID  string   `json:"requirement_id"`
	Measure        Ref      `json:"measure"`
	MappingVersion string   `json:"mapping_version"`
	Rationale      string   `json:"rationale"`
}

// Artifact is configuration-only; it intentionally has no credential or payload field.
// Values are restricted to schema-specific, scalar configuration observations.
type Artifact struct {
	Schema        string                     `json:"schema"`
	Configuration map[string]json.RawMessage `json:"configuration"`
}
type Submission struct {
	ServiceID       string       `json:"service_id"`
	ScopeRevision   int          `json:"scope_revision"`
	ProfileRevision int          `json:"profile_revision"`
	IdempotencyKey  string       `json:"idempotency_key"`
	Target          Target       `json:"target"`
	ObservedAt      time.Time    `json:"observed_at"`
	Method          string       `json:"method"`
	ToolVersion     string       `json:"tool_version"`
	Limitations     []string     `json:"limitations"`
	Coverage        Coverage     `json:"coverage"`
	Artifact        Artifact     `json:"artifact"`
	Evaluations     []Evaluation `json:"evaluations"`
	Links           []Link       `json:"links"`
}
type Finding struct {
	Target        Target    `json:"target"`
	ObservationID string    `json:"observation_id"`
	ObservedAt    time.Time `json:"observed_at"`
	ID            string    `json:"id"`
	Condition     string    `json:"condition"`
	Outcome       string    `json:"outcome"`
	Explanation   string    `json:"explanation"`
	Basis         string    `json:"basis"`
}
type Receipt struct {
	ID           string    `json:"id"`
	Evidence     []Ref     `json:"evidence"`
	ArtifactHash string    `json:"artifact_hash"`
	SourceKind   string    `json:"source_kind"`
	ActorID      string    `json:"actor_id"`
	ReceivedAt   time.Time `json:"received_at"`
	Findings     []Finding `json:"findings"`
}

var fields = map[string][]string{
	"aws.rds/v1":                      {"engine", "storage_encrypted", "kms_key_id", "backup_retention_days", "source_resource", "cluster_id", "backup_owner", "resource_type"},
	"aws.elasticache/v1":              {"engine", "engine_version", "transit_encryption_enabled", "transit_encryption_mode", "resource_type", "replication_group"},
	"aws.eks.endpoint/v1":             {"public_access", "private_access", "public_cidrs"},
	"aws.guardduty/v1":                {"detector_id", "enabled", "enabled_features"},
	"aws.eks.logging/v1":              {"enabled_log_types"},
	"repository.configuration/v1":     {"blob_sha", "matches", "evidence_kind"},
	"kubernetes.network-isolation/v1": {"policy_names", "ingress_isolated", "egress_isolated", "enforcement", "cni", "namespace_excluded"},
}

func (s Submission) Validate(now time.Time) error {
	b, err := json.Marshal(s.Artifact)
	if err != nil {
		return ErrInvalid
	}
	if len(b) > MaxArtifactBytes {
		return ErrTooLarge
	}
	if s.ServiceID == "" || s.ScopeRevision < 1 || s.ProfileRevision < 1 || len(s.IdempotencyKey) == 0 || len(s.IdempotencyKey) > 256 || s.Method == "" || s.ToolVersion == "" || s.ObservedAt.IsZero() || s.ObservedAt.After(now.Add(FutureTolerance)) || len(s.Links) == 0 || len(s.Links) > 100 {
		return ErrInvalid
	}
	if !slices.Contains([]string{"aws", "kubernetes", "repository"}, s.Target.Kind) {
		return ErrInvalid
	}
	if s.Coverage.Expected < 0 || s.Coverage.Observed < 0 || s.Coverage.Observed > s.Coverage.Expected || !slices.Contains([]string{"observed", "partial", "error", "no_resources"}, s.Coverage.State) {
		return ErrInvalid
	}
	if s.Coverage.Complete && (s.Coverage.Error != "" || s.Coverage.Expected != s.Coverage.Observed || !slices.Contains([]string{"observed", "no_resources"}, s.Coverage.State)) {
		return ErrInvalid
	}
	if s.Coverage.State == "no_resources" && (!s.Coverage.Complete || s.Coverage.Observed != 0) {
		return ErrInvalid
	}
	allowed, ok := fields[s.Artifact.Schema]
	if !ok || len(s.Artifact.Configuration) == 0 {
		return ErrInvalid
	}
	if !strings.HasPrefix(s.Artifact.Schema, s.Target.Kind+".") {
		return ErrInvalid
	}
	for k, v := range s.Artifact.Configuration {
		if !slices.Contains(allowed, k) || !json.Valid(v) {
			return fmt.Errorf("%w: configuration field %s", ErrInvalid, k)
		}
		var value any
		if json.Unmarshal(v, &value) != nil {
			return ErrInvalid
		}
		if value != nil {
			switch k {
			case "storage_encrypted", "transit_encryption_enabled", "public_access", "private_access", "enabled", "ingress_isolated", "egress_isolated", "namespace_excluded":
				if _, ok := value.(bool); !ok {
					return ErrInvalid
				}
			case "backup_retention_days":
				n, ok := value.(float64)
				if !ok || n < 0 || n != float64(int64(n)) || n > 3650 {
					return ErrInvalid
				}
			case "public_cidrs", "enabled_features", "enabled_log_types", "policy_names", "matches":
				if _, ok := value.([]any); !ok {
					return ErrInvalid
				}
			default:
				if _, ok := value.(string); !ok {
					return ErrInvalid
				}
			}
		}
		switch x := value.(type) {
		case string, bool, float64, nil:
		case []any:
			for _, item := range x {
				if _, ok := item.(string); !ok {
					return ErrInvalid
				}
			}
		default:
			return ErrInvalid
		}
	}
	seen := map[string]bool{}
	for _, l := range s.Links {
		if l.RequirementID == "" || l.Measure.ID == "" || l.Measure.Revision < 1 || l.MappingVersion == "" || l.Rationale == "" || seen[l.RequirementID+"/"+l.Measure.ID] {
			return ErrInvalid
		}
		seen[l.RequirementID+"/"+l.Measure.ID] = true
	}
	seen = map[string]bool{}
	for _, e := range s.Evaluations {
		if e.Condition == "" || e.Explanation == "" || !slices.Contains([]string{"pass", "fail", "unknown", "policy_required", "error"}, e.Outcome) || seen[e.Condition] {
			return ErrInvalid
		}
		seen[e.Condition] = true
	}
	return nil
}
