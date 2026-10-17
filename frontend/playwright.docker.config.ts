import { defineConfig } from "@playwright/test";
import config from "./playwright.config";

export default defineConfig({
  ...config,
  use: {
    baseURL: "http://host.docker.internal:5173",
    connectOptions: { wsEndpoint: "ws://127.0.0.1:3000/" },
  },
  webServer: {
    command: "bun run dev --host 0.0.0.0",
    url: "http://127.0.0.1:5173",
    reuseExistingServer: false,
  },
});
