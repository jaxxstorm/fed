package assessment

import (
	"encoding/json"
	"github.com/jaxxstorm/fed/internal/catalog"
	"os"
	"strings"
	"testing"
)

func TestInboxActionsUseOfficialDuties(t *testing.T) {
	// Read actual pinned source statements, rather than a paraphrased fixture.
	raw, err := os.ReadFile("../../testdata/fedramp/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	var root any
	if err = json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	rules := map[string]catalog.Object{}
	var visit func(any)
	visit = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, v := range x {
				if k == "AFC-CSO-INB" || k == "AFC-CSO-RCV" {
					rules[k] = v.(map[string]any)
				}
				visit(v)
			}
		case []any:
			for _, v := range x {
				visit(v)
			}
		}
	}
	visit(root)
	var all []catalog.Resolved
	for _, id := range []string{"AFC-CSO-INB", "AFC-CSO-RCV"} {
		d := rules[id]
		all = append(all, catalog.Resolved{Requirement: catalog.Requirement{ID: id, Name: d["name"].(string)}, Statement: d["statement"].(string), Details: d, Applicability: "required"})
	}
	actions := organizationActions(all[0], all)
	if len(actions) != 3 || actions[0].Title != "Create the FedRAMP Security Inbox" || actions[2].Title != "Monitor the inbox and respond" {
		t.Fatalf("missing implementation/operating actions: %#v", actions)
	}
	if !strings.Contains(strings.Join(actionExpectations(all[0]), " "), "Email address to receive messages from FedRAMP") {
		t.Fatal("official artifact expectations omitted")
	}
	all[1].Applicability = "not_applicable"
	if len(organizationActions(all[0], all)) != 2 {
		t.Fatal("assigned excluded related duty")
	}
	changed := all[0]
	changed.Statement = "Changed official obligation"
	if got := organizationActions(changed, all); len(got) != 1 || got[0].Action != changed.Statement {
		t.Fatal("stale interpretation reused for changed source")
	}
}
