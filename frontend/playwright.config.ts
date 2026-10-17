import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./tests",
  testMatch: [
    "gallery.spec.ts",
    "workflows.spec.ts",
    "connections.spec.ts",
    "connection-handoff.spec.ts",
    "provider-setup.spec.ts",
  ],
  use: { baseURL: "http://127.0.0.1:5173" },
  webServer: {
    command: "bun run dev",
    url: "http://127.0.0.1:5173",
    reuseExistingServer: false,
  },
  projects: [
    { name: "desktop", use: { viewport: { width: 1440, height: 1000 } } },
    { name: "tablet", use: { viewport: { width: 768, height: 1024 } } },
    { name: "mobile", use: { viewport: { width: 390, height: 844 } } },
  ],
});
