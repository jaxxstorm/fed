import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { components } from "./generated";
import {
  ContextGaps,
  GuidanceState,
  ScopedReadiness,
} from "./readiness-context";

describe("shared requirement readiness presentation", () => {
  it.each([
    "missing",
    "draft",
    "needs_revalidation",
  ])("keeps %s guidance distinct from reviewed content", (state) => {
    const html = renderToStaticMarkup(
      <GuidanceState state={state} reason="No current source comparison." />,
    );
    expect(html).toContain("No current source comparison.");
    expect(html).toContain(
      "Official requirements and existing assessments remain available.",
    );
    expect(html).not.toContain("Agent-reviewed");
  });

  const progress: components["schemas"]["ReadinessRequirementProgress"] = {
    binding: {
      catalog_id: "catalog",
      release_revision: 2,
      release_hash: "hash",
      profile_revision: 3,
      scope_revision: 4,
    },
    questions: [
      {
        requirement_id: "RULE",
        capability_id: "monitoring",
        question: "Did a production incident occur?",
        reason: "The operating duty is event-triggered.",
      },
    ],
    duties: [],
    capabilities: [
      {
        id: "instance",
        capability_id: "monitoring",
        name: "Production monitoring",
        scope: { kind: "account", account_id: "production" },
        requirement_ids: ["RULE"],
        prerequisite_ids: [],
        implementation: "declared_present",
        evidence_state: "available_for_assessment",
        trigger: "unknown",
        review: "not_assessed",
        blocked: true,
        needs_revalidation: false,
        steps: { items: [] },
        alternatives: [],
        next_action: "Collect dated incident records.",
        rationale: "Configuration does not prove response.",
        evidence: [
          {
            reference: { id: "artifact", revision: 2 },
            state: "available_for_assessment",
            reason: "Candidate, not assessed support.",
            limitations: ["Production only."],
          },
        ],
      },
    ],
  };

  it("shows unresolved conditions in cards and detail without inferring applicability", () => {
    for (const compact of [true, false]) {
      const html = renderToStaticMarkup(
        <ScopedReadiness progress={progress} compact={compact} />,
      );
      expect(html).toContain("Unresolved conditions");
      expect(html).toContain("Did a production incident occur?");
    }
    const detail = renderToStaticMarkup(
      <ScopedReadiness progress={progress} />,
    );
    expect(detail).toContain("Candidate, not assessed support.");
    expect(detail).toContain("artifact revision 2");
    expect(detail).toContain("Scope revision 4");
    expect(detail).toContain(
      "do not establish compliance or replace human review",
    );
  });

  it("keeps access failures distinct from absent implementation", () => {
    const html = renderToStaticMarkup(
      <ContextGaps
        gaps={[
          {
            title: "Read access denied",
            explanation: "Production records could not be read.",
            action: "Ask the source owner for a dated record.",
            targets: ["production", "production"],
          },
        ]}
      />,
    );
    expect(html).toContain("Production records could not be read.");
    expect(html).toContain("<h3>Action Items</h3>");
    expect(html).toContain("<details><summary>Affected targets (1)</summary>");
    expect(html.match(/<li>production<\/li>/g)).toHaveLength(1);
    expect(html).toContain("Ask the source owner for a dated record.");
    expect(html).toContain("does not establish that implementation is absent");
  });
});
