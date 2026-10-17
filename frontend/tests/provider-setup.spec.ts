import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

for (const provider of ["aws", "google_workspace", "github"]) {
  test(`${provider} guided setup resumes and confirms identity`, async ({ page }, info) => {
    await page.addInitScript(() => { Object.defineProperty(navigator, "clipboard", { value: { writeText: async (text: string) => { (window as any).__copiedSetup = text; } } }); });
    let setup: any = null;
    let complete = false;
    let conflict = true;
    const account = provider === "aws" ? "123456789012" : "verified-customer";
    const role = "arn:aws:iam::123456789012:role/FedEvidence";
    await page.route("**/api/ui/v1/**", async (route) => {
      const path = new URL(route.request().url()).pathname;
      if (path.endsWith("bootstrap")) return route.fulfill({ json: { services: [{ id: "fixture", name: "Setup assessment" }], csrf: "csrf", capabilities: { fixture: ["read", "edit", "connect"] } } });
      if (path.endsWith("providers")) return route.fulfill({ json: [{ id: provider, name: provider, server_adapter_available: true, permissions: [], guide: {}, access_modes: ["agent_managed"] }] });
      if (path.includes("/readiness/")) return route.fulfill({ json: { provider, ready: true, permissions: ["Read-only metadata"], customer_actions: ["Approve read-only access to the selected provider."], operator_actions: [], missing_configuration: [] } });
      if (path.endsWith("scope")) return route.fulfill({ json: { revision: 1 } });
      if (path.includes("/setups/") && route.request().method() === "GET") return route.fulfill({ json: setup });
      if (route.request().method() === "POST") {
        expect(route.request().headers()["x-csrf-token"]).toBe("csrf");
        const body = route.request().postDataJSON();
        expect(body.service_id).toBe("fixture");
        if (path.endsWith("begin")) setup = { id: "setup", service_id: "fixture", provider, step: "prerequisites", revision: 1, input: body.input };
        if (path.endsWith("update")) {
          if (conflict) { conflict = false; return route.fulfill({ status: 409, json: { code: "revision_conflict", title: "Setup changed", explanation: "Review and retry; entries are preserved." } }); }
          setup = { ...setup, input: body.input, revision: setup.revision + 1, step: "configure" };
        }
        if (path.endsWith("aws-material")) return route.fulfill({ json: { version: "v1", role_arn: role, cli: "#!/bin/bash\necho 'Create the read-only role'", trust_policy: { Statement: [] }, read_policy: { Statement: [] }, instructions: ["Create the role with the supplied trust and read policy."] } });
        if (path.endsWith("aws-verify")) setup = { ...setup, step: "confirm", verified_account_id: account, revision: setup.revision + 1 };
        if (path.endsWith("authorize")) return route.fulfill({ json: { url: new URL("/connections/setup/mock", route.request().url()).href, operation_id: "setup", expires_at: "2099-01-01T00:00:00Z" } });
        if (path.endsWith("complete")) { complete = true; setup = { ...setup, step: "complete", connection_id: "connection" }; }
        return route.fulfill({ json: setup });
      }
      if (path.endsWith("inventory")) return route.fulfill({ json: { resources: [] } });
      return route.fulfill({ json: complete ? [{ connection: { id: "connection", provider, name: "Assessment access", configuration: { account_id: account }, verified_account_id: account, access_mode: "server_managed", lifecycle: "active", revision: 1, generation: 1 }, access: { state: "verified", limitations: [] }, discovered_count: 0, selected_count: 0 }] : [] });
    });
    await page.route("**/connections/setup/mock", async (route) => {
      setup = { ...setup, step: "confirm", verified_account_id: account, revision: setup.revision + 1 };
      await route.fulfill({ status: 302, headers: { location: `/services/fixture?page=Connections&provider=${provider}&setup=setup` } });
    });
    await page.goto(`/services/fixture?page=Connections&provider=${provider}`);
    await page.getByLabel("Connection name").fill("Assessment access");
    if (provider === "aws") await page.getByLabel("Customer AWS account ID").fill(account);
    const action = provider === "aws" ? "Generate role setup" : provider === "github" ? "Connect with GitHub App" : "Connect with Google";
    await page.getByRole("button", { name: action, exact: true }).click();
    await expect(page.getByRole("alert")).toContainText("Setup changed");
    await expect(page.getByLabel("Connection name")).toHaveValue("Assessment access");
    await page.getByRole("button", { name: action, exact: true }).click();
    if (provider === "aws") {
      await expect(page.getByLabel("Customer role ARN")).toHaveValue(role);
      await page.getByRole("button", { name: "Copy CLI script" }).click();
      expect(await page.evaluate(() => (window as any).__copiedSetup)).toContain("Create the read-only role");
      const download = page.waitForEvent("download");
      await page.getByRole("button", { name: "Download script" }).click();
      expect((await download).suggestedFilename()).toBe("fed-aws-setup.sh");
      await page.getByLabel("Setup method").selectOption("manual");
      await expect(page.getByRole("heading", { name: "Trust policy" })).toBeVisible();
      await page.screenshot({ path: info.outputPath("aws-manual-setup.png"), fullPage: true });
      await page.getByRole("button", { name: "Verify role access" }).click();
    }
    await expect(page.getByRole("heading", { name: "Confirm verified identity" })).toBeVisible();
    await expect(page.getByText(account, { exact: true }).last()).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    const accessibility = await new AxeBuilder({ page }).analyze();
    expect(accessibility.violations.filter(v => ["critical", "serious"].includes(v.impact || ""))).toEqual([]);
    await page.screenshot({ path: info.outputPath(`${provider}-confirm.png`), fullPage: true });
    await page.getByRole("button", { name: "Confirm connection", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Assessment access", exact: true })).toBeVisible();
    expect(await page.evaluate(() => JSON.stringify(localStorage))).not.toContain("access_token");
  });
}

test("missing registration and cancelled consent give resumable actions", async ({ page }) => {
  await page.route("**/api/ui/v1/**", route => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("bootstrap")) return route.fulfill({ json: { services: [{ id: "fixture", name: "Setup assessment" }], csrf: "csrf", capabilities: { fixture: ["read", "connect"] } } });
    if (path.endsWith("providers")) return route.fulfill({ json: [{ id: "google_workspace", name: "Google Workspace", guide: {} }] });
    if (path.includes("readiness")) return route.fulfill({ json: { ready: false, missing_configuration: ["KSI_GOOGLE_CLIENT_ID"], operator_actions: ["Register the Fed OAuth client."], customer_actions: [], permissions: [] } });
    if (path.endsWith("scope")) return route.fulfill({ json: { revision: 1 } });
    return route.fulfill({ json: [] });
  });
  await page.goto("/services/fixture?page=Connections&provider=google_workspace&setup_error=authorization_cancelled");
  await expect(page.getByRole("heading", { name: "Operator setup required" })).toBeVisible();
  await expect(page.getByRole("alert")).toContainText("Your setup is saved");
  await expect(page.getByRole("button", { name: "Connect with Google", exact: true })).toBeDisabled();
});
