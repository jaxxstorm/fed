package assessment

import (
	"github.com/jaxxstorm/fed/internal/catalog"
	"sort"
)

// ActionItem describes organizational work, not evidence collection or a claim
// that the work is already implemented. Sources refer to the pinned catalog.
type ActionItem struct {
	Title   string   `json:"title"`
	Action  string   `json:"action"`
	Owner   string   `json:"owner"`
	Basis   string   `json:"basis"`
	Sources []string `json:"sources"`
}

// Preserve official field wording and categories; artifact expectations are
// outputs to retain, not a substitute for implementing the underlying duty.
func actionExpectations(rule catalog.Resolved) []string {
	var out []string
	var appendText func(string, any)
	appendText = func(label string, value any) {
		switch v := value.(type) {
		case string:
			out = append(out, label+": "+v)
		case []any:
			for _, item := range v {
				appendText(label, item)
			}
		case map[string]any:
			keys := make([]string, 0, len(v))
			for key := range v {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				appendText(label+" ("+key+")", v[key])
			}
		}
	}
	for _, field := range []struct{ key, label string }{{"artifacts", "Expected artifact"}, {"following_information", "Specified information"}, {"following_information_bullets", "Specified information"}, {"note", "FedRAMP note"}, {"notes", "FedRAMP note"}, {"danger", "FedRAMP caution"}} {
		appendText(field.label, rule.Details[field.key])
	}
	return out
}

func organizationActions(rule catalog.Resolved, all []catalog.Resolved) []ActionItem {
	owner := "The party identified in the official requirement"
	if actors, ok := rule.Details["affects"].([]any); ok && len(actors) > 0 {
		owner = ""
		for _, actor := range actors {
			if text, ok := actor.(string); ok {
				if owner != "" {
					owner += ", "
				}
				owner += text
			}
		}
	}
	items := []ActionItem{{Title: rule.Requirement.Name, Action: rule.Statement, Owner: owner, Basis: "Official requirement", Sources: []string{rule.Requirement.ID}}}
	// Bind summaries to exact official text so catalog changes cannot silently
	// inherit an outdated interpretation. Other rules retain their selected text.
	const inbox = "Providers MUST establish and maintain an email address to receive messages from FedRAMP; this inbox is a FedRAMP Security Inbox (FSI)."
	if rule.Requirement.ID == "AFC-CSO-INB" && rule.Statement == inbox {
		items = []ActionItem{
			{Title: "Create the FedRAMP Security Inbox", Action: "Establish an email address where your organization can receive FedRAMP messages.", Owner: "Your organization's mail administrator", Basis: "Implementation of AFC-CSO-INB", Sources: []string{"AFC-CSO-INB"}},
			{Title: "Keep the inbox working", Action: "Maintain the mailbox and staff access so it continues receiving FedRAMP messages when personnel change.", Owner: "Your organization's security communications owner and mail administrator", Basis: "Implementation of AFC-CSO-INB; its personnel-change warning informs continuity", Sources: []string{"AFC-CSO-INB"}},
		}
		for _, related := range all {
			if related.Requirement.ID == "AFC-CSO-RCV" && related.Applicability == "required" && related.Statement == "Providers MUST receive and react to email messages from FedRAMP without disruption and without requiring additional actions from FedRAMP." {
				items = append(items, ActionItem{Title: "Monitor the inbox and respond", Action: "Assign security or compliance staff to monitor incoming FedRAMP messages and act on them. Ensure delivery and handling do not require FedRAMP to solve a CAPTCHA, sign into your portal, or take another service-specific action.", Owner: "Your organization's security communications owner", Basis: "Implementation of the related AFC-CSO-RCV duty; staffing is an organizational implementation choice, and no fixed monitoring interval is specified here", Sources: []string{"AFC-CSO-INB", "AFC-CSO-RCV"}})
			}
		}
	}
	return items
}
