import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests/ui",
  fullyParallel: true,
  use: {
    baseURL: "http://localhost:5187",
    launchOptions: process.env.SPORTSMEET_BROWSER_PATH ? { executablePath: process.env.SPORTSMEET_BROWSER_PATH } : {},
    screenshot: "only-on-failure",
  },
  webServer: { command: "bunx vite --port 5187", url: "http://localhost:5187", reuseExistingServer: !process.env.CI },
});
