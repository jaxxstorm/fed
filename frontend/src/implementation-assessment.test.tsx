import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { components } from "./generated";
import { ImplementationAssessmentPanel } from "./implementation-assessment";

describe("period-bound implementation assessment", () => {
  it("does not present structural readiness as current or reviewed support", () => {
    const unknown = {
      state: "unknown",
      text: "",
      reason: "Records unavailable",
    };
    const context: components["schemas"]["ImplementationContext"] = {
      current_freshness: "stale",
      review: "pending",
      readiness: { ready: true, basis: "structural_only", blockers: [] },
      assessment: {
        version: "implementation-assessment/v1",
        period: { start: "2026-09-01T00:00:00Z", end: "2026-09-30T23:59:59Z" },
        narrative: {
          description: unknown,
          responsible_roles: unknown,
          mechanisms: unknown,
          procedures: unknown,
          frequency: unknown,
          boundary: unknown,
        },
        assertions: [],
      },
    };
    const html = renderToStaticMarkup(
      <ImplementationAssessmentPanel context={context} />,
    );
    expect(html).toContain("Structurally ready for human review");
    expect(html).toContain("not whether the evidence proves the claims");
    expect(html).toContain("stale");
    expect(html).toContain("pending");
    expect(html).toContain("2026-09-01");
    expect(html).toContain(
      "Historical support does not establish operations outside this period",
    );
  });
});
