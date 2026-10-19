package readiness

import (
	"strings"
	"testing"
)

func TestDeadlineCannotHideOutsideTimingField(t *testing.T) {
	for _, place := range []string{"purpose", "obligation", "evidence", "action"} {
		t.Run(place, func(t *testing.T) {
			p := draftFixture()
			var statement *Statement
			switch place {
			case "purpose":
				statement = &p.Playbooks[0].Purpose
			case "obligation":
				statement = &p.Playbooks[0].Assertions[0].Obligation
			case "evidence":
				statement = &p.Playbooks[0].Assertions[0].Acceptance.TimePeriod
			case "action":
				p.Capabilities[0].Setup = Steps{Items: []Step{{ID: "verify", OwnerRole: "Evidence owner", Trigger: "one_time", Action: p.Playbooks[0].Purpose, CompletionCriteria: []Statement{p.Playbooks[0].Purpose}}}}
				statement = &p.Capabilities[0].Setup.Items[0].Action
			}
			statement.Text = "Complete within 24 hours."
			statement.Basis = Official
			if err := ValidateDraft(p); err == nil || !strings.Contains(err.Error(), "uncited numeric timing") {
				t.Fatal("unsupported official deadline accepted", err)
			}
			statement.Basis = ServiceChoice
			statement.Rationale = "Explicit service-selected deadline, not a policy obligation."
			if err := ValidateDraft(p); err != nil {
				t.Fatal("labeled service deadline rejected", err)
			}
		})
	}
}
