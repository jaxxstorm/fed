// Separate acceptance entry point because the default config has an explicit allowlist.
import { defineConfig } from "@playwright/test";
import base from "./playwright.config";

export default defineConfig({
  ...base,
  testMatch: ["readiness-journeys.spec.ts"],
});
