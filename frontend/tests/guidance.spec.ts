import { expect, test } from "@playwright/test";

test("required action box precedes evidence and preserves agency responsibility without guidance", async ({ page }) => {
  const obligation = "Agencies MUST complete the Authorization to Operate process for federal information systems that use FedRAMP Certified cloud service offerings.";
  await page.route("**/api/ui/v1/**", route => route.fulfill({ json: route.request().url().endsWith("bootstrap")
    ? { services: [{ id: "fixture", name: "Agency example" }], capabilities: { fixture: ["read"] } }
    : { Title: "Agency authorization", Detail: true, Sections: [{ Title: "What this requirement asks for", Text: obligation }], Forms: [], EvidenceCoverage: { assertions: [{ assertion_id: "ato", obligation, states: ["missing"], rationale: "No current cited assessment demonstrates this obligation." }], candidates: [], collection: {} } }
  }));
  await page.goto("/services/fixture/requirements/ATO");
  const box = page.getByRole("region", { name: "This is what you need to do" });
  await expect(box.getByRole("heading", { name: "What is required", exact: true })).toBeVisible();
  await expect(box.getByText(obligation, { exact: true })).toBeVisible();
  await expect(box.getByText("The party named in the requirement—not automatically the cloud provider.", { exact: false })).toBeVisible();
  await expect(box.getByRole("heading", { name: "Your next evidence action" })).toBeVisible();
  const actionPosition = await box.boundingBox();
  const evidencePosition = await page.getByRole("region", { name: "Evidence coverage", exact: true }).boundingBox();
  expect(actionPosition!.y + actionPosition!.height).toBeLessThanOrEqual(evidencePosition!.y);
});

test("agent guidance shows caveat and uncertainty without human content approval", async ({ page }) => {
  const caveat = "Checked against cited official FedRAMP sources. Interpretations may be incomplete or incorrect; this is not an official FedRAMP determination.";
  await page.route("**/api/ui/v1/**", route => route.fulfill({ json: route.request().url().endsWith("bootstrap") ? {
    services: [{ id: "fixture", name: "Guidance fixture" }], csrf: "test", capabilities: { fixture: ["read"] },
  } : {
    Title: "Requirement explanation", Detail: true, Sections: [], Forms: [],
    Guidance: {
      state: "reviewed", label: "Agent-reviewed guidance", caveat,
      guidance: {
        sources: { rules: { commit: "pinned-policy-revision" } },
        citations: [{ id: "policy", source: "rules", path: "rules.json", anchor: "/statement", excerpt: "Official requirement excerpt." }],
        playbooks: [{ id: "rule", kind: "actionable", no_prerequisites_reason: "The existing service owner can collect these records directly.", setup: { items: [], none_reason: "No additional setup is prescribed." }, operate: { items: [{ id: "event-record", action: { text: "Retain the event response record.", basis: "official", citations: ["policy"] }, owner_role: "Service owner", trigger: "event", completion_criteria: [{text: "The record identifies the event and response.", basis: "recommended"}] }] }, collect: { items: [], none_reason: "Use the retained event response record." }, review_steps: { items: [], none_reason: "Assess the cited event record against the obligation." }, purpose: { text: "Keep evidence of the operating duty." }, applicability: { text: "Applies to this profile." }, review: {
          reviewer_type: "agent", reviewed_at: "2026-10-18T12:00:00Z", comparison: {
            limitations: ["Configuration alone does not establish actual operation."],
            uncertainties: [{ finding: "The cited policy gives no numeric deadline." }],
          },
        } }],
      },
    },
  } }));
  await page.goto("/services/fixture/requirements/RULE");
  await expect(page.getByRole("heading", { name: "Agent-reviewed guidance" })).toBeVisible();
  await expect(page.getByText(caveat, { exact: true })).toBeVisible();
  await expect(page.getByText("Uncertainty: The cited policy gives no numeric deadline.")).toBeVisible();
  await expect(page.getByRole("checkbox")).toHaveCount(0);
  await expect(page.getByRole("heading", {name: "What must exist first"})).toBeVisible();
  await expect(page.getByText("Retain the event response record.", {exact: true})).toBeVisible();
  await expect(page.getByText("Establish whether the event occurred before treating this duty as due. No event is not proof of completed operation.")).toBeVisible();
  await page.getByText("Official sources and review details").click();
  await expect(page.getByText("Official requirement excerpt.")).toBeVisible();
  await expect(page.getByText("Source revision: pinned-policy-revision")).toBeVisible();
});

test("missing guidance does not claim agent review", async ({ page }) => {
  await page.route("**/api/ui/v1/**", route => route.fulfill({ json: route.request().url().endsWith("bootstrap") ? {
    services: [{ id: "fixture", name: "Guidance fixture" }], csrf: "test", capabilities: { fixture: ["read"] },
  } : { Title: "Requirement explanation", Detail: true, Sections: [], Forms: [], Guidance: { state: "missing", reason: "No published source-reviewed guidance covers this requirement variant." } } }));
  await page.goto("/services/fixture/requirements/RULE");
  await expect(page.getByText("Guidance missing.",{exact:false})).toBeVisible();
  await expect(page.getByText("Agent-reviewed guidance", { exact: true })).toHaveCount(0);
});

test("evidence coverage keeps candidate proof and grouped failures distinct", async ({ page }) => {
  await page.route("**/api/ui/v1/**", route => route.fulfill({ json: route.request().url().endsWith("bootstrap") ? {
    services: [{ id: "fixture", name: "Evidence fixture" }], csrf: "test", capabilities: { fixture: ["read"] },
  } : {
    Title: "Workload isolation", Detail: true, Sections: [], Forms: [],
    EvidenceCoverage: {
      limitations: ["Configuration does not establish tested traffic enforcement."], blockers: [],
      assertions: [{ assertion_id: "enforcement", obligation: "Demonstrate intended traffic rules are enforced", states: ["missing", "access_blocked"], rationale: "Traffic-test records have not been collected.", citations: [], gaps: [{kind:"access_blocked",scope:"Production cluster",period:"Current review period",reason:"Production traffic-test records are not accessible.",next_action:"Ask the cluster owner for authorized production test records."}] }],
      candidates: [{ evidence: { id: "configuration", revision: 3 }, description: "Production policy inventory", observed_at: "2026-10-18T12:00:00Z", state: "candidate", reason: "Available for assertion assessment; relevance alone does not establish sufficiency.", limitations: ["No traffic tests were performed."] }],
      collection: { observed: 2, expected: 2, technical: "fail", last_observed_at: "2026-10-18T12:00:00Z", recovery: [] },
      check_groups: [{id:"passing-policy",affected_targets:1,outcome:"pass",freshness:"current",last_observed_at:"2026-10-18T12:00:00Z",explanation:{title:"Selected policy configuration",expected:"A policy selects this workload.",observed:"A collected policy selects the workload.",limitations:"Policy selection is not tested traffic enforcement.",owner_role:"Cluster owner",next_action:"Collect separate permitted/denied traffic-test records."},findings:[]}],
      finding_groups: [{ id: "ingress-prod", affected_targets: 2, explanation: {
        title: "Incoming workload traffic policy", expected: "A collected NetworkPolicy selects the workload for ingress isolation (incoming traffic rules).",
        observed: "No collected NetworkPolicy selects these workloads for ingress isolation.", limitations: "Policy selection does not establish public exposure or tested traffic enforcement.", owner_role: "Cluster owner", next_action: "Collect workload labels, namespace policies and intended incoming-traffic rules.",
      }, findings: ["one", "two"].map(id => ({ id, observation_id: `observation-${id}`, observed_at: "2026-10-18T12:00:00Z", target: { cluster_id: "production", namespace: "app", resource_id: `pod-${id}` }, condition: "ingress_isolation", outcome: "fail", basis: "configuration", explanation: "Whether a policy selects this workload for ingress isolation." })) }],
    },
  } }));
  await page.goto("/services/fixture/requirements/RULE");
  await expect(page.getByRole("heading", {name: "Incoming workload traffic policy · 2 affected targets"})).toHaveCount(1);
  await expect(page.getByText("Traffic-test records have not been collected.")).toBeVisible();
  await expect(page.getByText("Access Blocked · Production cluster · Current review period")).toBeVisible();
  await expect(page.getByText("Next: Ask the cluster owner for authorized production test records.")).toBeVisible();
  await expect(page.getByText("Check result: Pass · Current",{exact:false})).toBeVisible();
  await expect(page.getByText("Available for assertion assessment; relevance alone does not establish sufficiency.")).toBeVisible();
  await expect(page.getByRole("link", { name: "Open retained artifact · revision 3" })).toHaveAttribute("href", /configuration\?revision=3$/);
  await expect(page.getByText("Diagnostic: ingress_isolation", {exact: false}).first()).not.toBeVisible();
  await page.getByRole("article").filter({has:page.getByRole("heading",{name:"Incoming workload traffic policy · 2 affected targets"})}).getByText("Resources and original observations").click();
  await expect(page.getByRole("link", { name: "Open retained observation" })).toHaveCount(2);
  await expect(page.getByText("production · app · pod-one", {exact: false})).toBeVisible();
});
