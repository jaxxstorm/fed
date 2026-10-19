package observation

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/jaxxstorm/fed/internal/storage"
)

const FindingExplanationVersion = "finding-explanations/v1"

type FindingExplanation struct {
	Version     string `json:"version"`
	Title       string `json:"title"`
	Expected    string `json:"expected"`
	Observed    string `json:"observed"`
	Limitations string `json:"limitations"`
	NextAction  string `json:"next_action"`
	OwnerRole   string `json:"owner_role"`
	Known       bool   `json:"known"`
}

// ExplainFinding interprets retained configuration facts, never the positive
// predicate text as a negative result. Unknown schemas/codes retain raw details.
func ExplainFinding(f Finding) FindingExplanation {
	e := FindingExplanation{Version: FindingExplanationVersion, Title: "Finding needs interpretation", Expected: "No reviewed explanation is available for this diagnostic.", Observed: "The retained check reported " + f.Outcome + ". Inspect the original diagnostic and evidence.", Limitations: "No cause, exposure or enforcement conclusion is inferred from an unknown diagnostic.", NextAction: "Have the evidence owner inspect the cited observation and explain the condition before assessing it.", OwnerRole: "Evidence owner"}
	if f.Coverage == nil {
		return e
	}
	if f.Coverage.Error == "no_clusters_observed" && f.ArtifactSchema == "aws.eks.control-plane-logging/v1" {
		e.Title = "EKS cluster inventory is unresolved"
		e.Expected = "Identify the in-scope EKS clusters before comparing their enabled control-plane log types."
		e.Observed = "No EKS clusters were reported in this collection partition. Logging coverage remains unknown."
		e.Limitations = "This result does not establish logging for standalone Kubernetes clusters or prove that no clusters exist elsewhere."
		e.OwnerRole = "Cluster owner"
		e.NextAction = "Confirm the saved account, region and EKS inventory with the cluster owner; collect logging records for the actual in-scope clusters."
		e.Known = true
		return e
	}
	if f.Coverage.Error == "no_detector_observed" && f.ArtifactSchema == "aws.guardduty.configuration/v1" {
		e.Title = "GuardDuty detector inventory is unresolved"
		e.Expected = "Identify the detector and compare its configured features with the saved measure."
		e.Observed = "No GuardDuty detector was reported in this collection partition. The configured-feature check remains unknown."
		e.Limitations = "This does not establish the absence of other monitoring capabilities or detector coverage in other accounts or regions."
		e.OwnerRole = "Monitoring owner"
		e.NextAction = "Confirm the saved account and region, then obtain the detector configuration or document the actual monitoring arrangement."
		e.Known = true
		return e
	}
	if f.Coverage.State == "no_resources" {
		e.Title = "No resource targets reported"
		e.Expected = "Reconcile the collected inventory with the saved service boundary."
		e.Observed = "The collector reported an empty target inventory for this partition; no resource-level control was evaluated."
		e.Limitations = "An empty inventory is not a passing control or proof that the entire service has no relevant resources."
		e.NextAction = "Ask the resource owner to confirm this partition's inventory and collect any missing in-scope targets."
		return e
	}
	if f.Coverage.Error != "" || f.Coverage.State == "error" {
		e.Title = "Evidence collection did not complete"
		e.Observed = "The collector could not read the evidence needed to evaluate this condition."
		e.Limitations = "Failed collection does not establish that a control or policy is absent."
		e.NextAction = "Check the retained access error, obtain authorized read access or equivalent records, then repeat collection."
		return e
	}
	if !f.Coverage.Complete || f.Coverage.State != "observed" {
		e.Title = "Evidence coverage is incomplete"
		e.Observed = "The observation does not establish complete coverage of this condition."
		e.NextAction = "Collect the missing targets or pages before drawing a scope-wide conclusion."
		return e
	}
	if f.Condition == "collector_result" {
		if legacy, ok := explainLegacyCheck(f); ok {
			return legacy
		}
	}
	boolean := func(key string) (bool, bool) {
		var value bool
		raw, ok := f.Configuration[key]
		if !ok || string(raw) == "null" || json.Unmarshal(raw, &value) != nil {
			return false, false
		}
		return value, true
	}
	var field string
	switch f.ArtifactSchema + "/" + f.Condition {
	case "kubernetes.network-isolation/v1/ingress_isolation":
		field = "ingress_isolated"
		e.Title = "Incoming workload traffic policy"
		e.Expected = "A collected NetworkPolicy selects this workload for ingress isolation (incoming traffic rules)."
		e.Observed = "No collected NetworkPolicy selects this workload for ingress isolation."
		e.Limitations = "Policy selection is configuration evidence; it does not establish public exposure or tested traffic enforcement."
		e.NextAction = "Ask the cluster owner for the workload labels, namespace policies and intended incoming-traffic rules. Collect the matching policy or record the uncovered workload; obtain traffic-test evidence separately."
		e.OwnerRole = "Cluster owner"
	case "kubernetes.network-isolation/v1/egress_isolation":
		field = "egress_isolated"
		e.Title = "Outgoing workload traffic policy"
		e.Expected = "A collected NetworkPolicy selects this workload for egress isolation (outgoing traffic rules)."
		e.Observed = "No collected NetworkPolicy selects this workload for egress isolation."
		e.Limitations = "Policy selection does not prove that outgoing connections are blocked or that enforcement was tested."
		e.NextAction = "Ask the cluster owner for intended outgoing connections, workload labels and namespace policies. Collect matching configuration and separate enforcement-test records."
		e.OwnerRole = "Cluster owner"
	case "aws.rds/v1/storage_encryption":
		field = "storage_encrypted"
		e.Title = "Database storage encryption"
		e.Expected = "The database configuration reports storage encryption enabled."
		e.Observed = "The database configuration reports storage encryption disabled."
		e.Limitations = "This snapshot does not establish encryption of every copy, key-access restrictions, or application authorization."
		e.NextAction = "Ask the database owner to confirm the resource identity and collect its encryption configuration and relevant key-access records. Keep the disabled-encryption observation as contradictory evidence."
		e.OwnerRole = "Database owner"
	case "aws.elasticache/v1/transit_encryption":
		field = "transit_encryption_enabled"
		e.Title = "Cache connection encryption"
		e.Expected = "The cache configuration requires encrypted connections."
		e.Observed = "The cache configuration reports transit encryption disabled."
		e.Limitations = "Configuration does not demonstrate client certificate verification or observed connection behavior."
		e.NextAction = "Ask the cache owner for the encryption mode, client connection settings and connection-test records; document any permitted plaintext connections."
		e.OwnerRole = "Cache owner"
		var mode string
		_ = json.Unmarshal(f.Configuration["transit_encryption_mode"], &mode)
		if mode == "preferred" {
			e.Observed = "Preferred encryption mode still permits plaintext clients."
			e.Known = true
			return e
		}
	case "aws.rds/v1/backup_retention":
		e.Title = "Database backup retention"
		e.Expected = "Retention on the controlling resource is compared with the service's documented recovery objectives."
		e.Limitations = "Retention configuration alone does not prove that restores succeed or that recovery objectives are met."
		e.NextAction = "Ask the recovery owner for the controlling resource's backup settings, approved recovery objectives and dated restore results."
		e.OwnerRole = "Recovery owner"
		var days int
		var owner string
		if json.Unmarshal(f.Configuration["backup_retention_days"], &days) == nil && string(f.Configuration["backup_retention_days"]) != "null" && json.Unmarshal(f.Configuration["backup_owner"], &owner) == nil && owner == "self" && days == 0 {
			e.Observed = "Automated backup retention is disabled on the controlling database resource."
			e.Known = true
			return e
		}
		e.Observed = "The retained configuration requires comparison with the controlling resource and recovery objectives."
		e.Known = true
		return e
	case "aws.eks.endpoint/v1/public_endpoint":
		e.Title = "Kubernetes administration endpoint access"
		e.Expected = "Public endpoint source ranges are compared with approved administrative and deployment access."
		e.Limitations = "Allowed source addresses do not establish successful authentication, exploitation or workload exposure."
		e.NextAction = "Ask the cluster owner for approved administrative ranges and the retained endpoint configuration; document the comparison without changing endpoint access."
		e.OwnerRole = "Cluster owner"
		public, ok := boolean("public_access")
		var ranges []string
		if ok && !public {
			e.Observed = "The public administration endpoint is disabled."
			e.Known = true
			return e
		}
		if ok && public && json.Unmarshal(f.Configuration["public_cidrs"], &ranges) == nil {
			for _, cidr := range ranges {
				if cidr == "0.0.0.0/0" || cidr == "::/0" {
					e.Observed = "The public administration endpoint configuration allows an unrestricted source-address range."
					e.Known = true
					return e
				}
			}
			e.Observed = "The public administration endpoint has configured source ranges that require comparison with approved ranges."
			e.Known = true
			return e
		}
	case "kubernetes.network-isolation/v1/network_policy_enforcement":
		e.Title = "Network policy enforcement evidence"
		e.Expected = "Evidence demonstrates that the cluster actually enforces the intended traffic rules."
		e.Observed = "This configuration observation does not establish actual traffic enforcement."
		e.Limitations = "A policy manifest or supported network-plugin setting is not an end-to-end traffic test."
		e.NextAction = "Ask the cluster owner for dated permitted/denied traffic-test results and the relevant network-plugin configuration."
		e.OwnerRole = "Cluster owner"
		e.Known = true
		return e
	default:
		return e
	}
	if field != "" {
		if value, ok := boolean(field); ok {
			if value {
				e.Observed = map[string]string{
					"ingress_isolated":           "A collected NetworkPolicy selects this workload for incoming-traffic isolation.",
					"egress_isolated":            "A collected NetworkPolicy selects this workload for outgoing-traffic isolation.",
					"storage_encrypted":          "The database configuration reports storage encryption enabled.",
					"transit_encryption_enabled": "The cache configuration reports transit encryption enabled.",
				}[field]
				e.NextAction = map[string]string{
					"ingress_isolated":           "Retain the selecting policy and workload labels. Ask the cluster owner for separate permitted/denied incoming-traffic test records.",
					"egress_isolated":            "Retain the selecting policy and workload labels. Ask the cluster owner for separate permitted/denied outgoing-connection test records.",
					"storage_encrypted":          "Retain the dated encryption configuration and ask the database owner for relevant key-access and copy-coverage records.",
					"transit_encryption_enabled": "Retain the encryption mode and ask the cache owner for client verification settings and dated connection-test records.",
				}[field]
			}
			e.Known = true
			return e
		}
	}
	e.Observed = "The retained configuration does not expose the field needed to establish this condition."
	return e
}

// These legacy collectors retain the original result artifact but do not copy
// all configuration fields into Finding. Describe only their outcome semantics,
// never invent the particular missing feature, log type or source term.
func explainLegacyCheck(f Finding) (FindingExplanation, bool) {
	e := FindingExplanation{Version: FindingExplanationVersion, Known: true}
	var pass, fail, unknown string
	switch f.ArtifactSchema {
	case "aws.guardduty.configuration/v1":
		e.Title = "GuardDuty configured features"
		e.OwnerRole = "Monitoring owner"
		e.Expected = "The detector status and feature names selected by the saved measure report ENABLED."
		pass = "The collector reported every selected detector/feature setting enabled."
		fail = "The collector reported at least one selected detector/feature setting missing or not enabled."
		unknown = "The collector did not establish a configured-feature result; a measure with no selected features is unknown."
		e.Limitations = "Configured features do not prove complete resource coverage, successful detection, alert delivery or response. The individual feature values remain in the original result artifact."
		e.NextAction = "Ask the monitoring owner for the selected feature list and detector configuration; collect separate coverage and alert-handling records."
	case "aws.eks.control-plane-logging/v1":
		e.Title = "EKS control-plane log configuration"
		e.OwnerRole = "Cluster owner"
		e.Expected = "The saved measure's selected control-plane log types are enabled for this EKS cluster."
		pass = "The collector reported every selected log type enabled in the cluster configuration."
		fail = "The collector reported at least one selected control-plane log type not enabled."
		unknown = "The collector did not establish the log-type comparison; a measure with no selected log types is unknown."
		e.Limitations = "Enabled logging is configuration intent, not proof of log delivery, retention, review or standalone-cluster coverage."
		e.NextAction = "Ask the cluster owner for the selected log types, dated cluster configuration and separate log-delivery, retention and review records."
	case "github.pulumi.workflow-evidence/v1":
		e.Title = "Workflow text matches"
		e.OwnerRole = "Repository owner"
		e.Expected = "The selected workflow file contains the saved measure's search terms, using case-insensitive text matching."
		pass = "The collector found every selected search term in the retrieved workflow file."
		fail = "At least one selected search term was not found in the retrieved workflow file."
		unknown = "The collector did not establish the text comparison; a measure with no selected search terms is unknown."
		e.Limitations = "Text matches do not parse workflow behavior or prove execution, deployment, recurring drift checks or enforcement. A missing word does not prove a capability is absent."
		e.NextAction = "Ask the repository owner for the exact commit, file and search-term results, then collect relevant workflow-run records separately."
	default:
		return e, false
	}
	switch f.Outcome {
	case "pass":
		e.Observed = pass
	case "fail":
		e.Observed = fail
	default:
		e.Observed = unknown
	}
	return e, true
}

type FindingGroup struct {
	Outcome         string             `json:"outcome"`
	Freshness       string             `json:"freshness,omitempty"`
	LastObservedAt  *time.Time         `json:"last_observed_at,omitempty"`
	ID              string             `json:"id"`
	Condition       string             `json:"condition"`
	Explanation     FindingExplanation `json:"explanation"`
	AffectedTargets int                `json:"affected_targets"`
	Findings        []Finding          `json:"findings"`
}

func GroupFindings(findings []Finding) []FindingGroup {
	groups := map[string]*FindingGroup{}
	seen := map[string]map[string]bool{}
	for _, f := range findings {
		e := ExplainFinding(f)
		// Keep unknown causes and different environments/access results separate.
		cause := ""
		if !e.Known {
			cause = f.Explanation
		}
		if f.Condition == "collector_result" {
			cause += "/" + f.ObservationID
		}
		if f.Coverage != nil {
			cause += "/" + f.Coverage.Error + "/" + f.Coverage.State
		}
		key := strings.Join([]string{f.ArtifactSchema, f.Condition, f.Outcome, f.Freshness, e.Observed, cause, f.Basis, f.Target.Kind, f.Target.AccountID, f.Target.Region, f.Target.ClusterID, f.Target.Cluster, f.Target.NamespaceUID, f.Target.Namespace, f.Target.Repository, f.Target.Commit}, "\x00")
		id := storage.Hash([]byte(key))
		g := groups[id]
		if g == nil {
			g = &FindingGroup{ID: id, Condition: f.Condition, Explanation: e, Findings: []Finding{}, Outcome: f.Outcome, Freshness: f.Freshness}
			groups[id] = g
			seen[id] = map[string]bool{}
		}
		raw, _ := json.Marshal(f.Target)
		target := string(raw)
		if !seen[id][target] {
			seen[id][target] = true
			g.AffectedTargets++
		}
		g.Findings = append(g.Findings, f)
		if !f.ObservedAt.IsZero() && (g.LastObservedAt == nil || f.ObservedAt.After(*g.LastObservedAt)) {
			at := f.ObservedAt
			g.LastObservedAt = &at
		}
	}
	out := []FindingGroup{}
	for _, g := range groups {
		sort.Slice(g.Findings, func(i, j int) bool { return g.Findings[i].ID < g.Findings[j].ID })
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
