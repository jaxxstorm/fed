package assessment

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
)

const TemplateVersion = "collection-v1"
const MappedCatalog = catalog.PinnedCommit + ":fb48d677a85d77fb72735a325de2c1945a1e0ceab1d2e655292fe9ebe3e4e0b4"

type CollectionTemplate struct {
	ID             string   `json:"id"`
	RequirementIDs []string `json:"requirement_ids"`
	Rationale      string   `json:"rationale"`
	Expected       []string `json:"expected"`
	PolicyInputs   []string `json:"policy_inputs"`
}

func CollectionTemplates() []CollectionTemplate {
	return []CollectionTemplate{
		{"aws.guardduty.configuration", []string{"KSI-CNA-RVP"}, "Detector configuration supports unwanted-activity protection review; does not establish effective response or DDoS protection.", []string{"detector"}, nil},
		{"aws.eks.control-plane-logging", []string{"KSI-MLA-LET"}, "Control-plane log types support event logging inventory; delivery, retention and review require separate evidence.", []string{"api", "audit", "authenticator", "controllerManager", "scheduler"}, nil},
		{"github.pulumi.workflow-evidence", []string{"KSI-SVC-ACM"}, "Pinned workflow configuration is intent, not proof of deployment or drift detection.", []string{"pulumi"}, nil},
		{"aws.rds.configuration", []string{"KSI-SVC-SIN", "KSI-RPL-ABO"}, "RDS encryption and backup configuration partially supports securing information and backup-objective review; restore success is not established.", nil, []string{"minimum_retention_days"}},
		{"aws.elasticache.transit-encryption", []string{"KSI-SVC-SIN"}, "TLS enforcement supports information protection in transit; client verification is not established.", nil, nil},
		{"aws.eks.endpoint-access", []string{"KSI-CNA-RNT", "KSI-CNA-MAT"}, "Endpoint settings support network restriction and attack-surface review; they do not establish all network reachability.", nil, []string{"approved_cidrs"}},
		{"kubernetes.network-isolation", []string{"KSI-CNA-RNT", "KSI-CNA-ULN", "KSI-CNA-MAT"}, "Workload isolation and CNI configuration support traffic-flow review; policy existence alone does not prove enforcement.", nil, nil},
	}
}

type CollectionEntry struct {
	Conditions    []string `json:"conditions,omitempty"`
	ID            string   `json:"id"`
	RequirementID string   `json:"requirement_id"`
	CheckID       string   `json:"check_id,omitempty"`
	Measure       Ref      `json:"measure"`
	State         string   `json:"state"`
	Rationale     string   `json:"rationale"`
	PolicyInputs  []string `json:"policy_inputs,omitempty"`
}
type CollectionPlan struct {
	ConfigurationRevision int               `json:"configuration_revision"`
	ServiceID             string            `json:"service_id"`
	Revision              int               `json:"revision"`
	ScopeRevision         int               `json:"scope_revision"`
	ProfileRevision       int               `json:"profile_revision"`
	TemplateVersion       string            `json:"template_version"`
	Entries               []CollectionEntry `json:"entries"`
}

func (s *Store) GetCollectionPlan(ctx context.Context, a auth.Actor, service string) (CollectionPlan, error) {
	if err := auth.Authorize(ctx, s.DB, a, service, "read"); err != nil {
		return CollectionPlan{}, err
	}
	var raw []byte
	err := s.DB.QueryRow(ctx, `SELECT data FROM collection_plans WHERE service_id=$1 ORDER BY revision DESC LIMIT 1`, service).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return CollectionPlan{}, ErrNotFound
	}
	if err != nil {
		return CollectionPlan{}, err
	}
	return Decode[CollectionPlan](Record{Data: raw})
}

// ReconcileCollection is an internal worker operation. Transport adapters expose
// authorized plan reads/configuration, not this service-identity provisioning path.
func (s *Store) ReconcileCollection(ctx context.Context, service string) (CollectionPlan, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return CollectionPlan{}, err
	}
	defer tx.Rollback(ctx)
	var org string
	if err = tx.QueryRow(ctx, `SELECT organization_id FROM services WHERE id=$1 FOR UPDATE`, service).Scan(&org); err != nil {
		return CollectionPlan{}, err
	}
	sr, err := latest(ctx, tx, service, "scope", "scope")
	if err != nil {
		return CollectionPlan{}, err
	}
	pr, err := latest(ctx, tx, service, "profile", "profile")
	if err != nil {
		return CollectionPlan{}, err
	}
	p, err := Decode[catalog.Profile](pr)
	if err != nil {
		return CollectionPlan{}, err
	}
	var raw []byte
	var configRevision, freshness int
	var policyRaw []byte
	err = tx.QueryRow(ctx, `SELECT coalesce((SELECT revision FROM collection_schedules WHERE service_id=$1),1),coalesce((SELECT freshness_hours FROM collection_schedules WHERE service_id=$1),48),coalesce((SELECT overrides FROM collection_schedules WHERE service_id=$1),'{}'::jsonb)`, service).Scan(&configRevision, &freshness, &policyRaw)
	if err != nil {
		return CollectionPlan{}, err
	}
	policy, err := Decode[map[string]json.RawMessage](Record{Data: policyRaw})
	if err != nil {
		return CollectionPlan{}, err
	}
	err = tx.QueryRow(ctx, `SELECT data FROM collection_plans WHERE service_id=$1 AND scope_revision=$2 AND profile_revision=$3 AND template_version=$4 AND configuration_revision=$5`, service, sr.Revision, pr.Revision, TemplateVersion, configRevision).Scan(&raw)
	if err == nil {
		return Decode[CollectionPlan](Record{Data: raw})
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return CollectionPlan{}, err
	}
	rows, err := tx.Query(ctx, `SELECT data FROM requirements WHERE catalog_id=$1 ORDER BY id`, p.CatalogID)
	if err != nil {
		return CollectionPlan{}, err
	}
	all := []catalog.Requirement{}
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			rows.Close()
			return CollectionPlan{}, err
		}
		v, e := Decode[catalog.Requirement](Record{Data: b})
		if e != nil {
			rows.Close()
			return CollectionPlan{}, e
		}
		all = append(all, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return CollectionPlan{}, err
	}
	resolved, err := catalog.Resolve(all, p)
	if err != nil {
		return CollectionPlan{}, err
	}
	a := auth.Actor{ID: "automation:" + service, OrganizationID: org, Kind: "automation"}
	if _, err = tx.Exec(ctx, `INSERT INTO identities(id,organization_id,kind,issuer,subject,name) VALUES($1,$2,'automation','internal',$1,'Collection worker') ON CONFLICT(id) DO NOTHING`, a.ID, org); err != nil {
		return CollectionPlan{}, err
	}
	for _, action := range []string{"read", "edit", "check"} {
		if _, err = tx.Exec(ctx, `INSERT INTO permissions(identity_id,service_id,action) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, a.ID, service, action); err != nil {
			return CollectionPlan{}, err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO collection_schedules(service_id,actor_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, service, a.ID); err != nil {
		return CollectionPlan{}, err
	}
	plan := CollectionPlan{ServiceID: service, ScopeRevision: sr.Revision, ProfileRevision: pr.Revision, ConfigurationRevision: configRevision, TemplateVersion: TemplateVersion, Entries: []CollectionEntry{}}
	if err = tx.QueryRow(ctx, `SELECT coalesce(max(revision),0)+1 FROM collection_plans WHERE service_id=$1`, service).Scan(&plan.Revision); err != nil {
		return plan, err
	}
	for _, req := range resolved {
		state := "unsupported_automation"
		if req.Applicability == "not_applicable" {
			state = "not_applicable"
		} else if req.Applicability == "unresolved" {
			state = "unresolved_applicability"
		} else if strings.HasPrefix(req.Requirement.ID, "AFC-CSO-") {
			state = "manual_evidence"
		}
		matched := false
		if state == "unsupported_automation" && p.CatalogID == MappedCatalog {
			for _, template := range CollectionTemplates() {
				for _, rid := range template.RequirementIDs {
					if rid != req.Requirement.ID {
						continue
					}
					matched = true
					id := uuid.NewSHA1(uuid.NameSpaceURL, []byte(service+"/"+template.ID+"/"+rid)).String()
					mr, e := latest(ctx, tx, service, "measure", id)
					revision := 1
					m := Measure{Name: template.ID, Rationale: template.Rationale, CheckID: template.ID, Expected: template.Expected, FreshnessHours: freshness, ScopeRevision: sr.Revision, Policy: policy, ConfigurationRevision: configRevision}
					if e == nil {
						revision = mr.Revision
						old, e := Decode[Measure](mr)
						if e != nil {
							return plan, e
						}
						var overridden bool
						if err = tx.QueryRow(ctx, `SELECT coalesce((SELECT overridden FROM managed_measures WHERE service_id=$1 AND measure_id=$2),false)`, service, id).Scan(&overridden); err != nil {
							return plan, err
						}
						if overridden {
							m = old
							if m.Policy == nil {
								m.Policy = policy
							}
						}
						m.ScopeRevision = sr.Revision
						m.ConfigurationRevision = configRevision
						if string(data(old)) != string(data(m)) {
							revision++
						}
					} else if !errors.Is(e, ErrNotFound) {
						return plan, e
					}
					if mr.Revision != revision {
						if _, err = tx.Exec(ctx, `INSERT INTO records(service_id,kind,id,revision,requirement_id,actor_id,data) VALUES($1,'measure',$2,$3,$4,$5,$6)`, service, id, revision, rid, a.ID, data(m)); err != nil {
							return plan, err
						}
						if err = audit(ctx, tx, a, service, "measure.generated", id, revision); err != nil {
							return plan, err
						}
					}
					if _, err = tx.Exec(ctx, `INSERT INTO managed_measures(service_id,measure_id,template_id,template_version) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, service, id, template.ID, TemplateVersion); err != nil {
						return plan, err
					}
					entryState := "runnable"
					conditions := []string{}
					policyInputs := template.PolicyInputs
					if template.ID == "aws.rds.configuration" {
						if rid == "KSI-SVC-SIN" {
							conditions = []string{"storage_encryption"}
							policyInputs = nil
						} else {
							conditions = []string{"backup_retention"}
						}
					}
					for _, key := range policyInputs {
						if _, ok := m.Policy[key]; !ok {
							entryState = "policy_required"
						}
					}
					plan.Entries = append(plan.Entries, CollectionEntry{ID: id, RequirementID: rid, CheckID: template.ID, Measure: Ref{id, revision}, State: entryState, Rationale: template.Rationale, PolicyInputs: policyInputs, Conditions: conditions})
				}
			}
		}
		if !matched {
			plan.Entries = append(plan.Entries, CollectionEntry{ID: req.Requirement.ID, RequirementID: req.Requirement.ID, State: state, Rationale: req.Reason})
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO collection_plans(service_id,revision,scope_revision,profile_revision,template_version,data,configuration_revision) VALUES($1,$2,$3,$4,$5,$6,$7)`, service, plan.Revision, sr.Revision, pr.Revision, TemplateVersion, data(plan), configRevision); err != nil {
		return plan, err
	}
	for _, entry := range plan.Entries {
		if _, err = tx.Exec(ctx, `INSERT INTO collection_entries(service_id,plan_revision,id,data) VALUES($1,$2,$3,$4)`, service, plan.Revision, entry.ID, data(entry)); err != nil {
			return plan, err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE collection_schedules SET next_run_at=now() WHERE service_id=$1`, service); err != nil {
		return plan, err
	}
	if err = audit(ctx, tx, a, service, "collection.reconciled", service, plan.Revision); err != nil {
		return plan, err
	}
	return plan, tx.Commit(ctx)
}
