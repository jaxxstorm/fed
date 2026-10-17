package mcp

import (
	"context"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerAgentEvidence(server *sdk.Server, store *assessment.Store, a auth.Actor) {
	add(server, "list_open_rules", "Discover outstanding evidence work for the selected profile, including documentary obligations. Follow next_cursor; restart if view_changed. No collector worker or measure setup required.", func(ctx context.Context, in assessment.OpenRulesInput) (any, error) {
		return store.ListOpenRules(ctx, a, in)
	})
	add(server, "get_rule_instructions", "Read official text, assertion IDs, current revision bindings, relevant sources, existing evidence, gaps and investigation steps. Use these bindings when submitting a result.", func(ctx context.Context, in RequirementInput) (any, error) {
		return store.RuleInstructions(ctx, a, in.ServiceID, in.RequirementID)
	})
	add(server, "list_evidence_sources", "Read the same versioned source directory configured in the UI. Access hints name client-side profiles/contexts, not credentials. Use your own tools to read these sources.", func(ctx context.Context, in ListInput) (any, error) {
		if in.Limit == 0 {
			in.Limit = 25
		}
		if in.Limit < 1 || in.Limit > 100 || in.Offset < 0 {
			return nil, assessment.ErrInvalid
		}
		if err := store.ReconcileEvidenceSources(ctx, a, in.ServiceID); err != nil {
			return nil, err
		}
		items, err := store.List(ctx, a, in.ServiceID, "evidence_source", "", in.Limit, in.Offset, in.IncludeRetired)
		if err != nil {
			return nil, err
		}
		page := Page{Items: items}
		if len(items) == in.Limit {
			next := in.Offset + in.Limit
			page.NextOffset = &next
		}
		return page, nil
	})
	add(server, "submit_evidence_artifact", "Upload reusable scoped text/plain, text/markdown or application/json evidence directly (maximum 1 MiB content). Source/version and original observation time are required. Returns immutable evidence metadata; uploading alone does not complete a rule.", func(ctx context.Context, in assessment.ArtifactSubmission) (any, error) {
		return store.SubmitEvidenceArtifact(ctx, a, in)
	})
	add(server, "submit_rule_result", "Submit per-assertion agent analysis and exact evidence citations with instruction bindings and expected result/draft revisions. Returns a completion/partial/blocked receipt and cited draft. Evidence complete is independent of compliance and human review.", func(ctx context.Context, in assessment.RuleResultSubmission) (any, error) {
		return store.SubmitRuleResult(ctx, a, in)
	})
}
