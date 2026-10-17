import { expect, test } from "@playwright/test";

test("agent handoff resumes connection setup and partial discovery stays explicit", async ({ page }, info) => {
  const detail = { connection: { id: "workspace", name: "Workspace directory", provider: "google_workspace", configuration: { account_id: "C123" }, access_mode: "server_managed", lifecycle: "active", revision: 1, generation: 1 }, access: { state: "authorization_required", limitations: [] }, discovered_count: 0, selected_count: 0 };
  await page.route("**/api/ui/v1/**", (route) => {
    const url = new URL(route.request().url());
    if (url.pathname.endsWith("bootstrap")) return route.fulfill({ json: { services: [{ id: "fixture", name: "Agent handoff" }], csrf: "csrf", capabilities: { fixture: ["read", "connect", "edit", "check"] } } });
    if (url.pathname.endsWith("providers")) return route.fulfill({ json: [{ id: "google_workspace", name: "Google Workspace", server_adapter_available: true, access_modes: ["server_managed"], permissions: ["Directory customer, org-unit and group read-only scopes"], guide: { execution: "Administrator offline consent", steps: ["Register the OAuth client.", "Grant read-only access in your browser.", "Inspect customer identity and discover resources."], configuration_names: ["KSI_GOOGLE_CLIENT_ID"] } }] });
    if (url.pathname.endsWith("scope")) return route.fulfill({ json: { revision: 1 } });
    if (url.pathname.endsWith("inventory")) return route.fulfill({ json: { resources: [] } });
    if (url.pathname.endsWith("status")) return route.fulfill({ json: { id: "agent-operation", connection_id: "workspace", state: "succeeded" } });
    if (url.pathname.endsWith("authorize")) return route.fulfill({ json: { operation_id: "agent-operation", url: `${url.origin}/connections/authorize/fixture/agent-operation` } });
    if (url.pathname.endsWith("discover")) return route.fulfill({ json: { id: "scan" } });
    if (url.pathname.endsWith("discover-page")) return route.fulfill({ json: { session_id: "scan", state: "partial", accepted: 0, next_cursor: "", coverage: [{ kind: "org_unit", error_code: "insufficient_permissions" }] } });
    return route.fulfill({ json: [detail] });
  });
  await page.route("**/connections/authorize/fixture/agent-operation", (route) => route.fulfill({ contentType: "text/html", body: '<h1>Complete agent-requested authorization</h1><p>Allow Directory read-only access for customer C123.</p><form method="post" action="/fixture-consent"><button>Allow read access</button></form>' }));
  await page.route("**/fixture-consent", (route) => { detail.access.state = "verified"; detail.connection.generation = 2; return route.fulfill({ status: 303, headers: { location: "/services/fixture?page=Connections&connection=workspace&operation=agent-operation" } }); });
  // This is the same-origin URL returned to the external agent, opened by an
  // authorized browser. Backend tests verify actor/session/token binding.
  await page.goto("/connections/authorize/fixture/agent-operation");
  await expect(page.getByRole("heading", { name: "Complete agent-requested authorization" })).toBeVisible();
  await page.getByRole("button", { name: "Allow read access" }).click();
  await expect(page.getByText("Authorization Succeeded")).toBeVisible();
  await page.getByRole("button", { name: "Discover resources", exact: true }).click();
  await expect(page.getByText(/Discovery Partial.*insufficient_permissions/)).toBeVisible();
  expect(await page.evaluate(() => JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage } }))).not.toMatch(/access_token|refresh_token|kubeconfig|client_secret/);
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({ path: info.outputPath("Handoff-partial-recovery.png"), fullPage: true });
});
