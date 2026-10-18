import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

test("admin approves signed-in users and sees revision conflicts", async ({ page }) => {
  const member = { identity_id: "pending", organization_id: "default", email: "member@example.com", name: "Member", role: "", state: "pending", revision: 1 };
  let conflict = true;
  await page.route("**/api/ui/v1/**", async route => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("bootstrap")) return route.fulfill({ json: { services: [], csrf: "csrf", capabilities: {}, can_create: true, can_manage_organization: true, role: "admin" } });
    if (path.endsWith("organization")) return route.fulfill({ json: { id: "default", name: "Example", allowed_domains: ["example.com"], revision: 1 } });
    if (route.request().method() === "POST") {
      expect(route.request().headers()["x-csrf-token"]).toBe("csrf");
      const body = route.request().postDataJSON();
      if (conflict) { conflict = false; return route.fulfill({ status: 409, json: { code: "revision_conflict", title: "Membership changed", explanation: "Refresh and review the current request." } }); }
      expect(body.operation).toBe("approve");
      expect(body.role).toBe("user");
      Object.assign(member, { role: body.role, state: "active", revision: 2 });
      return route.fulfill({ json: member });
    }
    return route.fulfill({ json: { members: [member] } });
  });
  await page.goto("/organization");
  await expect(page.getByRole("heading", { name: "Organization", exact: true })).toBeVisible();
  await expect(page.getByText(/Waiting for approval/)).toBeVisible();
  await page.getByLabel("Role for member@example.com").selectOption("user");
  await page.getByRole("button", { name: "Approve access" }).click();
  await expect(page.getByRole("heading", { name: "Membership changed" })).toBeVisible();
  await expect(page.getByLabel("Role for member@example.com")).toHaveValue("user");
  await page.getByRole("button", { name: "Approve access" }).click();
  await expect(page.getByText("Member · active · user", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Approve access" })).toHaveCount(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});

test("viewer has no management or mutation controls", async ({ page }) => {
  await page.route("**/api/ui/v1/**", route => route.fulfill({ json: route.request().url().endsWith("bootstrap") ? { services: [{ id: "fixture", name: "Example" }], csrf: "csrf", capabilities: { fixture: ["read"] }, can_create: false, role: "viewer" } : { Title: "Settings", Forms: [{ Action: "/services/fixture/actions/token", Title: "Agent access", Button: "Issue token", Fields: [] }], Links: [{ Text: "Download export", URL: "/services/fixture/exports/example/json" }] } }));
  await page.goto("/services/fixture?page=Settings");
  await expect(page.getByRole("heading", { name: "Service settings" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Organization & users" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Issue token" })).toHaveCount(0);
  await expect(page.getByRole("link", { name: "Download export" })).toHaveCount(0);
  await page.goto("/organization");
  await expect(page.getByRole("heading", { name: "Administrator access required" })).toBeVisible();
});

test("real jailed page is accessible and has no product requests", async ({ page }) => {
  test.skip(!process.env.KSI_APPROVAL_URL, "requires isolated jailed application fixture");
  const productRequests: string[] = [];
  page.on("request", request => { if (request.url().includes("/api/ui/v1/") && !request.url().includes("membership-status")) productRequests.push(request.url()); });
  await page.goto(`${process.env.KSI_APPROVAL_URL}/approval`);
  await expect(page.getByRole("heading", { name: "Waiting for approval" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Refresh status" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Sign out" })).toBeVisible();
  await page.getByRole("link", { name: "Refresh status" }).focus();
  await page.keyboard.press("Tab");
  await expect(page.getByRole("button", { name: "Sign out" })).toBeFocused();
  expect(productRequests).toEqual([]);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});

test("denied requests return to pending and self-demotion removes management", async ({ page }) => {
  const members = [
    { identity_id: "request", organization_id: "default", email: "request@example.com", name: "Request", role: "", state: "pending", revision: 1 },
    { identity_id: "self", organization_id: "default", email: "admin@example.com", name: "Admin", role: "admin", state: "active", revision: 1 },
  ];
  await page.route("**/api/ui/v1/**", async route => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("bootstrap")) return route.fulfill({ json: { services: [], csrf: "csrf", capabilities: {}, can_create: members[1].role === "admin", can_manage_organization: members[1].role === "admin", role: members[1].role } });
    if (path.endsWith("organization")) return route.fulfill({ json: { id: "default", name: "Example", allowed_domains: [], revision: 1 } });
    if (route.request().method() === "POST") {
      const body = route.request().postDataJSON();
      const member = members.find(m => m.identity_id === body.identity_id);
      if (!member) throw new Error("unknown member");
      expect(body.expected_revision).toBe(member.revision);
      member.revision++;
      if (body.operation === "disable") member.state = "disabled";
      else if (body.operation === "enable") member.state = "pending";
      else member.role = body.role;
      return route.fulfill({ json: member });
    }
    return route.fulfill({ json: { members } });
  });
  await page.goto("/organization");
  await page.getByRole("button", { name: "Deny access" }).click();
  await expect(page.getByText("Request · disabled", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Return to pending" }).click();
  await expect(page.getByRole("button", { name: "Approve access" })).toBeVisible();
  await page.getByLabel("Role for admin@example.com").selectOption("viewer");
  await page.getByRole("button", { name: "Change role" }).click();
  await expect(page.getByRole("heading", { name: "Administrator access required" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Approve access" })).toHaveCount(0);
});
