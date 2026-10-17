import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const providers = [
  { id: "github", name: "GitHub" },
  { id: "google_workspace", name: "Google Workspace" },
  { id: "aws", name: "AWS" },
  { id: "kubernetes", name: "Kubernetes" },
].map((provider) => ({ ...provider, server_adapter_available: true, access_modes: ["server_managed", "agent_managed"], resource_kinds: ["repository"], permissions: ["Read-only provider metadata"], guide: { version: "1", execution: "Credentials execute on the server; agents may use their own tools.", configuration_names: ["KSI_PUBLIC_URL"], callback_path: `/connections/callback/${provider.id}`, steps: ["Register the provider application with read-only permissions.", "Authorize access and verify the intended account.", "Discover resources, select scope and collect evidence."] } }));

test("provider catalog and named setup fields stay accessible at every viewport", async ({ page }, info) => {
  const created: object[] = [];
  await page.route("**/api/ui/v1/**", (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("bootstrap")) return route.fulfill({ json: { services: [{ id: "fixture", name: "Connection assessment" }], csrf: "csrf", capabilities: { fixture: ["read", "edit", "connect", "check"] } } });
    if (path.endsWith("providers")) return route.fulfill({ json: providers });
    if (path.endsWith("scope")) return route.fulfill({ json: { revision: 1 } });
    if (route.request().method() === "POST") {
      const body = route.request().postDataJSON();
      expect(route.request().headers()["x-csrf-token"]).toBe("csrf");
      expect(body.configuration.account_id).toBeTruthy();
      const detail = { connection: { ...body, id: body.provider, revision: 1, generation: 1, lifecycle: "active" }, access: { state: "not_configured", limitations: [] }, discovered_count: 0, selected_count: 0 };
      created.push(detail); return route.fulfill({ json: detail });
    }
    if (path.endsWith("inventory")) return route.fulfill({ json: { resources: [] } });
    return route.fulfill({ json: created });
  });
  await page.goto("/services/fixture?page=Connections");
  await expect(page.getByRole("heading", { name: "Connections", exact: true })).toBeVisible();
  await page.screenshot({ path: info.outputPath("Connection-catalog.png"), fullPage: true });
  for (const provider of providers) {
    await page.getByRole("button", { name: `${provider.name} Server integration available`, exact: true }).click();
    if (provider.id !== "kubernetes") await page.getByRole("button", { name: "Advanced / agent-managed setup" }).click();
    await expect(page.getByRole("heading", { name: `Configure ${provider.name}` })).toBeVisible();
    await page.getByLabel("Name", { exact: true }).fill(`${provider.name} production`);
    await expect(page.getByLabel("Access mode", { exact: true })).toBeVisible();
    if (provider.id === "aws") await expect(page.getByLabel("Server profile")).toBeVisible();
    if (provider.id === "kubernetes") await expect(page.getByLabel("HTTPS API server")).toBeVisible();
    await page.getByLabel(provider.id === "kubernetes" ? "Cluster identity (kube-system namespace UID)" : "Account / customer ID", { exact: true }).fill(provider.id === "aws" ? "123456789012" : "11111111-1111-4111-8111-111111111111");
    if (provider.id === "kubernetes") { await page.getByLabel("Cluster ID (same UID as above)").fill("11111111-1111-4111-8111-111111111111"); await page.getByLabel("HTTPS API server").fill("https://cluster.example"); }
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    const accessibility = await new AxeBuilder({ page }).analyze();
    expect(accessibility.violations.filter((violation) => ["critical", "serious"].includes(violation.impact || ""))).toEqual([]);
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.screenshot({ path: info.outputPath(`${provider.id}-setup.png`), fullPage: true });
    await page.getByRole("button", { name: "Save connection", exact: true }).click();
    await expect(page.getByRole("heading", { name: `${provider.name} production`, exact: true })).toBeVisible();
  }
});

test("cross-page selection survives scope conflicts and separates evidence refresh from renewal", async ({ page }, info) => {
  let scopeRevision = 1;
  let attempts = 0;
  let selected = false;
  let lifecycle = "enabled";
  let sourceRevision = 1;
  let refreshes = 0;
  let renewals = 0;
  let connectionLifecycle = "active";
  let generation = 1;
  const resource = (id: string) => ({ resource: { provider: "github", tenant: "99", kind: "repository", id }, name: `example/repo-${id}`, locator: `https://github.com/example/repo-${id}`, last_seen_at: "2026-10-17T12:00:00Z", available: true, access_mode: "server_managed", actor_id: "admin", source_id: selected ? `source-${id}` : "", source_lifecycle: lifecycle, bound_connection_id: selected ? "connection" : "", source_revision: sourceRevision, binding_revision: sourceRevision });
  await page.route("**/api/ui/v1/**", async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname;
    if (path.endsWith("bootstrap")) return route.fulfill({ json: { services: [{ id: "fixture", name: "Connection assessment" }], csrf: "csrf", capabilities: { fixture: ["read", "edit", "connect", "check"] } } });
    if (path.endsWith("providers")) return route.fulfill({ json: providers });
    if (path.endsWith("scope")) return route.fulfill({ json: { revision: scopeRevision } });
    if (path.endsWith("inventory")) return route.fulfill({ json: { resources: [resource(url.searchParams.get("cursor") ? "2" : "1")], next_cursor: url.searchParams.get("cursor") ? "" : "page-two", generation } });
    if (route.request().method() === "POST") {
      expect(route.request().headers()["x-csrf-token"]).toBe("csrf");
      const body = route.request().postDataJSON();
      if (path.endsWith("select")) {
        expect(body.sources).toHaveLength(2);
        expect(body.add_to_scope).toBe(true);
        attempts++;
        if (attempts === 1) { scopeRevision = 2; return route.fulfill({ status: 409, json: { code: "revision_conflict", title: "Scope changed", explanation: "Compare scope before retrying." } }); }
        expect(body.expected_scope_revision).toBe(2);
        selected = true;
      }
      if (path.endsWith("refresh-source")) refreshes++;
      if (path.endsWith("renew")) { renewals++; generation++; }
      if (path.endsWith("retire-source")) { lifecycle = "retired"; sourceRevision++; }
      if (path.endsWith("restore-source")) { lifecycle = "enabled"; sourceRevision++; }
      if (path.endsWith("delete")) connectionLifecycle = "deleted";
      return route.fulfill({ json: { state: "succeeded" } });
    }
    return route.fulfill({ json: [{ connection: { id: "connection", name: "GitHub production", provider: "github", configuration: { account_id: "example", installation_id: "7" }, verified_account_id: "99", access_mode: "server_managed", lifecycle: connectionLifecycle, revision: 1, generation }, access: { state: "verified", checked_at: "2026-10-17T12:00:00Z", limitations: [] }, discovered_count: 2, selected_count: selected ? 2 : 0 }] });
  });
  await page.goto("/services/fixture?page=Connections&connection=connection");
  await page.getByRole("checkbox", { name: /example\/repo-1/ }).check();
  await page.getByRole("button", { name: "Next page" }).click();
  await page.getByRole("checkbox", { name: /example\/repo-2/ }).check();
  await expect(page.getByText(/2 selected across pages/)).toBeVisible();
  for (const input of await page.getByLabel("Full commit SHA").all()) await input.fill("1111111111111111111111111111111111111111");
  for (const input of await page.getByLabel("Evidence paths (one per line)").all()) await input.fill(".github/workflows/ci.yaml");
  await page.getByRole("checkbox", { name: "Explicitly add selected resources to assessment scope" }).check();
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({ path: info.outputPath("Cross-page-selection.png"), fullPage: true });
  await page.getByRole("button", { name: "Select sources", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("Scope changed");
  await expect(page.getByLabel("Full commit SHA").first()).toHaveValue("1111111111111111111111111111111111111111");
  await page.getByRole("button", { name: "Try again" }).first().click();
  await expect(page.getByText(/scope revision 2/)).toBeVisible();
  await page.getByRole("button", { name: "Select sources", exact: true }).click();
  await expect(page.getByText(/Sources selected/)).toBeVisible();
  await page.getByRole("button", { name: "Renew", exact: true }).click();
  await expect.poll(() => renewals).toBe(1); expect(refreshes).toBe(0);
  await page.getByRole("button", { name: "Refresh evidence", exact: true }).click();
  await expect.poll(() => refreshes).toBe(1);
  await page.getByRole("button", { name: "Retire source", exact: true }).click();
  await expect(page.getByRole("button", { name: "Refresh evidence", exact: true })).toBeDisabled();
  await expect(page.getByRole("link", { name: "Source history" })).toBeVisible();
  await page.getByRole("button", { name: "Delete", exact: true }).click();
  await expect(page.getByRole("button", { name: "Restore connection" })).toBeVisible();
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({ path: info.outputPath("Retired-history.png"), fullPage: true });
});
