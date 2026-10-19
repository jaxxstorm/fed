import process from "node:process";
import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./tests",
  testMatch: [
    "gallery.spec.ts",
    "workflows.spec.ts",
    "connections.spec.ts",
    "connection-handoff.spec.ts",
    "provider-setup.spec.ts",
    "organization.spec.ts",
    "guidance.spec.ts",
    "readiness.spec.ts",
    "readiness-journeys.spec.ts",
  ],
  use: {
    baseURL: "http://127.0.0.1:5173",
    channel: process.env.PLAYWRIGHT_CHANNEL,
  },
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
