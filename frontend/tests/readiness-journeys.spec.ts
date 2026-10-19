import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { expect, test } from "@playwright/test";
import authored from "./fixtures/readiness-authored-subset.json" with { type: "json" };
import manifest from "./fixtures/readiness-authored-manifest.json" with { type: "json" };

// Exact authored draft content, separately paired with explicitly synthetic service
// evidence. Hash binding is reproducibility, not an authenticated review receipt.
const bytes = readFileSync(new URL("./fixtures/readiness-authored-subset.json", import.meta.url));
expect(createHash("sha256").update(bytes).digest("hex")).toBe(manifest.subset_sha256);
expect(authored.sources).toEqual(manifest.sources);
const human = (value: string) => value.replaceAll("_", " ").replace(/\b\w/g, c => c.toUpperCase());
const draftReason = "Authored draft subset without an authenticated review receipt; all service evidence below is synthetic.";

// Independent per-assertion evidence cases, not authored guidance or real findings.
const evidence = {
  "tested-recovery-capability": {
    have: "Synthetic development restore report", fact: "Synthetic development restore completed; production was not tested.",
    gap: "Production recovery capability remains untested in these synthetic records.",
    next: "Ask the recovery owner for authorized production test records and objective comparisons.",
  },
  "after-action-reports": {
    have: "Synthetic Incident 42 after-action report", fact: "Synthetic Incident 42 has a report; the incident inventory is absent.",
    gap: "Report coverage across the incident history cannot be established.",
    next: "Ask the incident response owner for the incident inventory and linked reports.",
  },
  "incorporated-lessons": {
    have: "Synthetic Incident 42 open lesson list", fact: "Synthetic lessons have open actions without resulting-practice records.",
    gap: "Lesson incorporation remains unsupported, separately from report generation.",
    next: "Ask the incident response owner for dated lesson dispositions and changed practices.",
  },
  "persistent-effectiveness-review": {
    have: "Synthetic undated training dashboard", fact: "The synthetic dashboard has no dated review history.",
    gap: "Repeated effectiveness review and current activity status are unsupported.",
    next: "Ask the training owner for dated review history and current status.",
  },
  "general-employee-training": {
    have: "Synthetic general employee attendance export", fact: "Synthetic employee attendance is recorded without effectiveness conclusions.",
    gap: "General employee training effectiveness review is unsupported.",
    next: "Ask the training owner for employee coverage and effectiveness conclusions.",
  },
  "high-risk-role-training": {
    have: "Synthetic generic awareness assignment", fact: "Synthetic assignments do not identify high-risk roles.",
    gap: "High-risk role-specific coverage and effectiveness review are missing.",
    next: "Ask the training owner and role leads for high-risk role mapping and review records.",
  },
  "secure-delivery-training": {
    have: "Synthetic secure-delivery course catalog", fact: "The synthetic catalog lists courses without engineering evaluations.",
    gap: "Development and engineering effectiveness conclusions are missing.",
    next: "Ask engineering role leads for secure-delivery evaluation and review records.",
  },
  "response-recovery-training": {
    have: "Synthetic restore activity log", fact: "The synthetic restore log never evaluates staff training.",
    gap: "Response and recovery training effectiveness review is unsupported.",
    next: "Ask response and recovery role leads for training effectiveness conclusions.",
  },
};

const journeys = [
  { domain: "technical", id: "KSI-RPL-TRC", count: 1 },
  { domain: "operational", id: "KSI-INR-AAR", count: 2 },
  { domain: "documentary", id: "KSI-CED-RAT", count: 5 },
];

for (const journey of journeys) {
  test(`${journey.domain} authored draft preserves duties and separate evidence gaps`, async ({ page }) => {
    const book = authored.playbooks.find(b => b.requirement_id === journey.id)!;
    expect(book.assertions).toHaveLength(journey.count);
    const assertions = book.assertions.map(assertion => {
      const sample = evidence[assertion.id as keyof typeof evidence];
      expect(sample).toBeDefined();
      return {
        assertion_id: assertion.id, obligation: assertion.obligation.text,
        acceptance: assertion.acceptance, states: ["partial"], rationale: sample.fact,
        gaps: [{ kind: "missing", scope: "Synthetic service boundary", period: "Synthetic review period", reason: sample.gap, next_action: sample.next }],
        citations: [{ evidence: { id: `synthetic-${assertion.id}`, revision: 3 }, start_line: 1, end_line: 1 }],
      };
    });
    const writes: string[] = [];
    await page.route("**/api/ui/v1/**", route => {
      if (route.request().method() !== "GET") writes.push(route.request().url());
      if (route.request().url().endsWith("bootstrap")) return route.fulfill({ json: {
        services: [{ id: "journey", name: "Synthetic readiness journey" }], csrf: "test", capabilities: { journey: ["read"] },
      } });
      return route.fulfill({ json: {
        Title: journey.id, Detail: true, Sections: [], Forms: [],
        Guidance: { state: "draft", reason: draftReason, guidance: { ...authored, playbooks: [book] } },
        EvidenceCoverage: {
          limitations: ["Synthetic service evidence only. Collection and declarations do not grant human approval."], blockers: [], assertions,
          candidates: book.assertions.map(assertion => ({
            evidence: { id: `synthetic-${assertion.id}`, revision: 3 },
            description: evidence[assertion.id as keyof typeof evidence].have,
            observed_at: "2026-10-18T12:00:00Z", state: "candidate",
            reason: "Synthetic candidate; relevance is not sufficiency.", limitations: [],
          })),
          collection: { observed: 0, expected: 0, technical: "unknown", recovery: [] }, check_groups: [], finding_groups: [],
        },
      } });
    });
    await page.goto(`/services/journey/requirements/${journey.id}`);
    await expect(page.getByText(`Draft guidance. ${draftReason}`, { exact: false })).toBeVisible();
    await expect(page.getByText("Agent-reviewed guidance", { exact: false })).toHaveCount(0);

    for (const dependency of book.prerequisites) {
      const capability = authored.capabilities.find(c => c.id === dependency.capability_id)!;
      await expect(page.getByRole("heading", { name: capability.name, exact: true })).toBeVisible();
      for (const step of capability.setup.items) {
        await expect(page.getByText(step.action.text, { exact: true })).toBeVisible();
        await expect(page.getByText(`${step.owner_role} · ${human(step.action.basis)} · ${human(step.trigger)}`, { exact: true }).first()).toBeVisible();
      }
    }
    const duties = page.locator("section").filter({ has: page.getByRole("heading", { name: "Operating duties", exact: true }) }).last();
    expect(book.operate.items.some(step => ["recurring", "event"].includes(step.trigger))).toBe(true);
    for (const step of book.operate.items) {
      const item = duties.locator("ol > li").filter({ has: page.getByText(step.action.text, { exact: true }) });
      await expect(item).toHaveCount(1);
      await expect(item.getByText(`${step.owner_role} · ${human(step.action.basis)} · ${human(step.trigger)}`, { exact: true })).toBeVisible();
      if (["recurring", "event"].includes(step.trigger)) await expect(item).not.toContainText("One Time");
      if (step.trigger === "event") await expect(item).toContainText("Establish whether the event occurred");
    }
    for (const step of book.review_steps.items) {
      await expect(page.getByText(step.action.text, { exact: true }).last()).toBeVisible();
      for (const criterion of step.completion_criteria) await expect(page.getByText(criterion.text, { exact: false }).last()).toBeVisible();
    }

    const coverage = page.getByRole("region", { name: "Evidence coverage", exact: true });
    await expect(coverage.locator("article > h3")).toHaveCount(journey.count);
    for (const assertion of assertions) {
      const article = coverage.locator("article").filter({ has: page.getByRole("heading", { name: assertion.obligation, exact: true }) });
      await expect(article).toHaveCount(1);
      await expect(article.getByText(assertion.rationale, { exact: true })).toBeVisible();
      await expect(article.getByText(assertion.gaps[0].reason, { exact: true })).toBeVisible();
      await expect(article.getByText(`Next: ${assertion.gaps[0].next_action}`, { exact: true })).toBeVisible();
      const proofID = `synthetic-${assertion.assertion_id}`;
      await expect(article.getByRole("link", { name: `Evidence ${proofID} · revision 3`, exact: true })).toHaveAttribute("href", new RegExp(`${proofID}\\?revision=3$`));
      await expect(article.getByText("· lines 1–1", { exact: false })).toBeVisible();
      await article.getByText("Acceptable proof and its limits", { exact: true }).click();
      await expect(article.getByText(assertion.acceptance.fact, { exact: true })).toBeVisible();
      await expect(article.getByText(`Provenance: ${assertion.acceptance.provenance.text}`, { exact: true })).toBeVisible();
      for (const insufficient of assertion.acceptance.insufficient) await expect(article.getByText(insufficient, { exact: true })).toBeVisible();
      for (const alternative of assertion.acceptance.alternatives) {
        await expect(article.getByText(`Proves: ${alternative.proves}`, { exact: true })).toBeVisible();
        await expect(article.getByText(`Does not prove: ${alternative.limitations}`, { exact: true })).toBeVisible();
      }
      await expect(coverage.getByRole("heading", { name: evidence[assertion.assertion_id as keyof typeof evidence].have, exact: true })).toBeVisible();
    }
    expect(writes, "Read-only journey must not mutate records").toEqual([]);
  });
}
