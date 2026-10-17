import { defineConfig, devices } from "@playwright/test";

// The UI review capture (`just ui-shots`, docs/design/ui-review/): every screen in every theme at
// the two layout viewports, plus the contrast report. Not part of `just e2e` (its 5-minute budget)
// nor of `just validate`: it is a documentation tool, run by hand. It reuses the e2e stack
// (web/e2e/fixtures.ts: in-memory API on a free port, the api/testdata fixture).
export default defineConfig({
  testDir: "./ui-review",
  globalSetup: "./e2e/global-setup.ts",
  outputDir: "./e2e-results/ui-review-artifacts",
  reporter: [["list"]],
  fullyParallel: true,
  retries: 0,
  timeout: 15 * 60_000,
  use: {
    ...devices["Desktop Chrome"],
    viewport: { width: 1440, height: 900 },
    trace: "retain-on-failure",
    launchOptions: { ignoreDefaultArgs: ["--hide-scrollbars"] },
  },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
});
