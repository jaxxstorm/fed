package mcp

import (
	"context"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/readiness"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerAgentEvidence(server *sdk.Server, store *assessment.Store, a auth.Actor) {
	add(server, "list_reporting_candidates", "List reporting candidates and owner notifications. Candidates require human review and never automatically publish. Use get_rule_instructions and get_assessment_review_target to investigate the reported evidence and review work.", func(ctx context.Context, in ListInput) (any, error) {
		if in.Limit == 0 {
			in.Limit = 25
		}
		items, err := store.ReportingRuns(ctx, a, in.ServiceID, in.Limit, in.Offset)
		page := Page{Items: items}
		if len(items) == in.Limit {
			next := in.Offset + in.Limit
			page.NextOffset = &next
		}
		return page, err
	})
	add(server, "list_assessment_releases", "List frozen assessment packages, blockers and comparisons. Preparation, human approval and publication are distinct.", func(ctx context.Context, in ListInput) (any, error) {
		if in.Limit == 0 {
			in.Limit = 25
		}
		items, err := store.ListReleases(ctx, a, in.ServiceID, in.Limit, in.Offset)
		page := Page{Items: items}
		if len(items) == in.Limit {
			next := in.Offset + in.Limit
			page.NextOffset = &next
		}
		return page, err
	})
	add(server, "get_assessment_review_target", "Read exact assessment dependencies, organizational adoption and internal/independent human review status. This tool cannot record a human decision.", func(ctx context.Context, in RequirementInput) (any, error) {
		return store.ReviewTarget(ctx, a, in.ServiceID, in.RequirementID)
	})
	add(server, "save_assessment_issue", "Track an issue, owner, milestone dates and supporting evidence. Closing requires a human review decision and reasoned disposition; this tool does not remediate infrastructure.", func(ctx context.Context, in assessment.AssessmentIssueChange) (any, error) {
		return store.SaveAssessmentIssue(ctx, a, in)
	})
	add(server, "list_assessment_issues", "List revisioned issues for a service or requirement. Follow next_offset. Planned milestones do not establish completed work.", func(ctx context.Context, in ListInput) (any, error) {
		if in.Limit == 0 {
			in.Limit = 25
		}
		items, err := store.List(ctx, a, in.ServiceID, "assessment_issue", in.RequirementID, in.Limit, in.Offset)
		page := Page{Items: items}
		if len(items) == in.Limit {
			next := in.Offset + in.Limit
			page.NextOffset = &next
		}
		return page, err
	})
	add(server, "save_evidence_request", "Create or revise an evidence request, or submit exact citations as fulfillment. Requires current instruction bindings and expected revision. Acceptance/cancellation and rejection of submitted work require human review authority.", func(ctx context.Context, in assessment.EvidenceRequestChange) (any, error) {
		return store.SaveEvidenceRequest(ctx, a, in)
	})
	add(server, "list_evidence_requests", "Discover owned or unassigned evidence requests filtered by requirement, owner or state. Follow next_offset. Revalidation-needed requests cannot be accepted against obsolete bindings.", func(ctx context.Context, in assessment.EvidenceRequestQuery) (any, error) {
		if in.Limit == 0 {
			in.Limit = 25
		}
		items, err := store.EvidenceRequests(ctx, a, in)
		page := Page{Items: items}
		if len(items) == in.Limit {
			next := in.Offset + in.Limit
			page.NextOffset = &next
		}
		return page, err
	})
	add(server, "get_evidence_request", "Read the latest immutable revision of one service-scoped evidence request.", func(ctx context.Context, in GetInput) (any, error) {
		return store.Get(ctx, a, in.ServiceID, "evidence_request", in.ID)
	})
	add(server, "submit_implementation_assessment", assessment.ImplementationInvestigationInstructions, func(ctx context.Context, in assessment.ImplementationSubmission) (any, error) {
		return store.SubmitImplementation(ctx, a, in)
	})
	add(server, "get_observation_detail", "Read one retained observation's exact target, time, configuration, coverage, source kind and artifact hash. Configuration observations do not establish tested enforcement or whole-requirement compliance.", func(ctx context.Context, in GetInput) (any, error) {
		return store.ObservationDetail(ctx, a, in.ServiceID, in.ID)
	})
	if store.Guidance != nil {
		add(server, "get_readiness_requirements", "Read a bounded, revision-bound requirement work queue grouped as evidence, review, scoping, maintain or excluded. Counts are requirements, not scoped capabilities or compliance percentages. Follow next_cursor; restart on view_changed. Human review cannot be completed by an agent or an implementation declaration.", func(ctx context.Context, in assessment.ReadinessQueueInput) (any, error) {
			return store.ReadinessQueue(ctx, a, in)
		})
		add(server, "get_readiness_capability", "Read one exact service-scoped capability instance and its dependent requirements. Uses the instance ID from the service plan. Questions are paginated; declarations are not verified support.", func(ctx context.Context, in struct {
			ServiceID string `json:"service_id"`
			ID        string `json:"id"`
			Limit     int    `json:"limit,omitempty"`
			Offset    int    `json:"offset,omitempty"`
		}) (any, error) {
			if in.Limit == 0 {
				in.Limit = 25
			}
			return store.Guidance.Capability(ctx, a, in.ServiceID, in.ID, in.Limit, in.Offset)
		})
		add(server, "get_guidance_coverage", "Read bounded published-guidance quality across the complete catalog variant denominator. Counts are content coverage, not service compliance. Compare catalog ID and release hash when combining pages.", func(ctx context.Context, in ListInput) (any, error) {
			if in.Limit == 0 {
				in.Limit = 25
			}
			return store.Guidance.Quality(ctx, a, in.ServiceID, in.Limit, in.Offset)
		})
		add(server, "get_service_readiness_plan", "Read prerequisite-first scoped capability work and explicit condition questions. Counts are capability instances, not compliance requirements. Implementation declarations, candidate evidence, triggers and assessment remain separate.", func(ctx context.Context, in ListInput) (any, error) {
			if in.Limit == 0 {
				in.Limit = 25
			}
			return store.Guidance.Plan(ctx, a, in.ServiceID, in.Limit, in.Offset)
		})
		add(server, "save_readiness_work", "Record scoped implementation declarations, ownership, alternatives, decisions and candidate evidence references. Requires exact guidance/profile/scope bindings, expected revision and idempotency key. Declarations do not establish evidence sufficiency or compliance.", func(ctx context.Context, in readiness.WorkChange) (any, error) {
			return store.Guidance.SaveWork(ctx, a, in)
		})
		add(server, "list_readiness_work", "Read service-scoped readiness declarations, preserving guidance and evidence bindings. These are declarations, not verified implementation or compliance.", func(ctx context.Context, in ListInput) (any, error) {
			if in.Limit == 0 {
				in.Limit = 25
			}
			items, err := store.Guidance.Work(ctx, a, in.ServiceID, in.Limit, in.Offset)
			page := Page{Items: items}
			if len(items) == in.Limit {
				next := in.Offset + in.Limit
				page.NextOffset = &next
			}
			return page, err
		})
		add(server, "review_requirement_guidance", "Record a completed comparison against the cited official public FedRAMP sources. Compare applicability, obligations, prerequisites, evidence and timing; record limitations and cited uncertainties. Drafting alone is not review. Requires service edit access; does not approve service evidence.", func(ctx context.Context, in GuidanceReviewInput) (any, error) {
			return store.Guidance.RecordContentReview(ctx, a, in.ServiceID, in.Package, in.PlaybookID, in.Comparison)
		})
		add(server, "publish_requirement_guidance", "Publish immutable source-reviewed guidance for this service. Every catalog variant must have a recorded source comparison. Review claims in the submitted package are ignored. Does not change compliance or human assessment review.", func(ctx context.Context, in GuidancePublishInput) (any, error) {
			hash, err := store.Guidance.Publish(ctx, a, in.ServiceID, in.Package)
			return map[string]string{"content_hash": hash}, err
		})
	}
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
	add(server, "submit_rule_result", "Submit per-assertion agent analysis and exact evidence citations with instruction bindings and expected result/draft revisions. Record unresolved gaps with kind (missing, partial, stale, access_blocked), exact scope, relevant period, reason and evidence-first next_action. An assertion with unresolved gaps cannot be supported or evidence-complete. Do not infer missing implementation from missing proof. Returns a completion/partial/blocked receipt and cited draft. Evidence complete is independent of compliance and human review.", func(ctx context.Context, in assessment.RuleResultSubmission) (any, error) {
		return store.SubmitRuleResult(ctx, a, in)
	})
}

type GuidanceReviewInput struct {
	ServiceID  string                     `json:"service_id"`
	Package    readiness.Package          `json:"package"`
	PlaybookID string                     `json:"playbook_id"`
	Comparison readiness.SourceComparison `json:"comparison"`
}

type GuidancePublishInput struct {
	ServiceID string            `json:"service_id"`
	Package   readiness.Package `json:"package"`
}
