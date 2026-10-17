import { defineConfig } from "@playwright/test";
import config from "./playwright.docker.config";
export default defineConfig({
  ...config,
  testMatch: "live.spec.ts",
  webServer: undefined,
  use: { ...config.use, baseURL: "http://host.docker.internal:8080" },
});
