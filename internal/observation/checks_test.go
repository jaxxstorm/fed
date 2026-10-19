package observation

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCheckRowsKeepPassingConfigurationSeparateFromEnforcement(t *testing.T) {
	now := time.Now().UTC()
	in := Submission{Target: Target{Kind: "kubernetes", ClusterID: "production", ResourceID: "pod"}, Coverage: Coverage{Complete: true, State: "observed", Expected: 1, Observed: 1}, Artifact: Artifact{Schema: "kubernetes.network-isolation/v1", Configuration: map[string]json.RawMessage{"ingress_isolated": json.RawMessage("true")}}, Evaluations: []Evaluation{{Condition: "ingress_isolation", Outcome: "pass"}, {Condition: "network_policy_enforcement", Outcome: "unknown"}}}
	rows := CheckRows(in, Link{}, "collector_observation", "observation", now, 24, now)
	if len(rows) != 2 || rows[0].Outcome != "pass" || rows[1].Outcome != "unknown" || rows[0].ObservationID != "observation" {
		t.Fatal("lost check results or provenance", rows)
	}
	e := ExplainFinding(rows[0])
	if !strings.Contains(e.Observed, "selects this workload") || !strings.Contains(e.Limitations, "tested traffic enforcement") {
		t.Fatal("passing configuration overclaimed", e)
	}
	stale := CheckRows(in, Link{}, "collector_observation", "older", now.Add(-48*time.Hour), 24, now)
	if stale[0].Freshness != "stale" || len(GroupFindings([]Finding{rows[0], stale[0]})) != 2 {
		t.Fatal("stale passing observation merged with current")
	}
	in.Coverage.Error = "access_denied"
	in.Coverage.State = "error"
	failed := CheckRows(in, Link{Conditions: []string{"ingress_isolation"}}, "collector_observation", "failed", now, 24, now)
	if len(failed) != 1 || failed[0].Outcome != "error" || !strings.Contains(ExplainFinding(failed[0]).Observed, "could not read") {
		t.Fatal("failed collection became passed control", failed)
	}
}

func TestLegacyChecksExplainOnlyRetainedOutcomeSemantics(t *testing.T) {
	for _, schema := range []string{"aws.guardduty.configuration/v1", "aws.eks.control-plane-logging/v1", "github.pulumi.workflow-evidence/v1"} {
		for _, outcome := range []string{"pass", "fail", "unknown"} {
			f := Finding{ArtifactSchema: schema, Condition: "collector_result", Outcome: outcome, Coverage: &Coverage{Complete: true, State: "observed"}}
			e := ExplainFinding(f)
			if !e.Known || e.Expected == "" || e.Observed == "" || e.Limitations == "" || e.NextAction == "" {
				t.Fatal("registered legacy condition lacks interpretation", schema, outcome, e)
			}
			if schema == "github.pulumi.workflow-evidence/v1" && !strings.Contains(e.Limitations, "do not parse workflow behavior") {
				t.Fatal("text matching became execution proof", e)
			}
			f.Coverage = &Coverage{State: "error", Error: "access_denied"}
			if e = ExplainFinding(f); !strings.Contains(e.Observed, "could not read") {
				t.Fatal("access failure interpreted as configuration", e)
			}
		}
	}
	f := Finding{ArtifactSchema: "aws.eks.control-plane-logging/v1", Condition: "collector_result", Outcome: "unknown", Coverage: &Coverage{State: "error", Error: "no_clusters_observed"}}
	e := ExplainFinding(f)
	if !strings.Contains(e.Observed, "No EKS clusters were reported") || !strings.Contains(e.Limitations, "standalone") {
		t.Fatal("empty EKS inventory mistaken for access failure or standalone coverage", e)
	}
	f.Coverage = &Coverage{Complete: true, State: "no_resources"}
	if e = ExplainFinding(f); !strings.Contains(e.Observed, "no resource-level control was evaluated") {
		t.Fatal("empty inventory became passing control", e)
	}
}
