package observation

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFindingExplanationsSeparateAbsentPolicyFromFailedRead(t *testing.T) {
	f := Finding{ArtifactSchema: "kubernetes.network-isolation/v1", Condition: "ingress_isolation", Outcome: "fail", Configuration: map[string]json.RawMessage{"ingress_isolated": json.RawMessage("false")}, Coverage: &Coverage{Complete: true, State: "observed"}}
	e := ExplainFinding(f)
	if !e.Known || !strings.Contains(e.Observed, "No collected NetworkPolicy") || !strings.Contains(e.Limitations, "does not establish public exposure") || !strings.Contains(e.NextAction, "Collect") {
		t.Fatal("negative evidence explanation missing", e)
	}
	f.Coverage = &Coverage{State: "error", Error: "policy_access_denied"}
	e = ExplainFinding(f)
	if strings.Contains(e.Observed, "No collected NetworkPolicy") || !strings.Contains(e.Observed, "could not read") {
		t.Fatal("failed read became absent policy", e)
	}
	f.Coverage = &Coverage{State: "partial"}
	if e = ExplainFinding(f); !strings.Contains(e.Observed, "complete coverage") {
		t.Fatal("partial inventory treated as complete", e)
	}
	f.Condition = "unreviewed-code"
	f.Coverage = &Coverage{Complete: true, State: "observed"}
	if e = ExplainFinding(f); e.Known || !strings.Contains(e.Expected, "No reviewed explanation") {
		t.Fatal("invented unknown-code meaning", e)
	}
}

func TestFindingGroupsKeepNamespacesCausesAndErrorsSeparate(t *testing.T) {
	base := Finding{ID: "one", ArtifactSchema: "kubernetes.network-isolation/v1", Condition: "ingress_isolation", Outcome: "fail", Configuration: map[string]json.RawMessage{"ingress_isolated": json.RawMessage("false")}, Coverage: &Coverage{Complete: true, State: "observed"}, Target: Target{Kind: "kubernetes", ClusterID: "production", NamespaceUID: "namespace-a", ResourceID: "pod-one"}}
	second := base
	second.ID = "two"
	second.Target.ResourceID = "pod-two"
	other := base
	other.ID = "three"
	other.Target.NamespaceUID = "namespace-b"
	failedRead := base
	failedRead.ID = "four"
	failedRead.Coverage = &Coverage{State: "error", Error: "access_denied"}
	groups := GroupFindings([]Finding{base, second, other, failedRead})
	if len(groups) != 3 {
		t.Fatal("distinct scopes or access errors merged", groups)
	}
	found := false
	for _, g := range groups {
		if g.AffectedTargets == 2 {
			found = true
			if len(g.Findings) != 2 {
				t.Fatal("group lost provenance")
			}
		}
	}
	if !found {
		t.Fatal("repeated findings not grouped")
	}
}

func TestPreferredTLSDoesNotClaimEncryptionDisabled(t *testing.T) {
	f := Finding{ArtifactSchema: "aws.elasticache/v1", Condition: "transit_encryption", Outcome: "fail", Coverage: &Coverage{Complete: true, State: "observed"}, Configuration: map[string]json.RawMessage{"transit_encryption_enabled": json.RawMessage("true"), "transit_encryption_mode": json.RawMessage(`"preferred"`)}}
	e := ExplainFinding(f)
	if !strings.Contains(e.Observed, "permits plaintext") || strings.Contains(e.Observed, "disabled") {
		t.Fatal("wrong TLS cause", e)
	}
}
