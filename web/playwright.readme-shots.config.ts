import { defineConfig, devices } from "@playwright/test";

// The README screenshots (`just readme-shots`, docs/frontend.md "README screenshots"): the app on the
// e2e stack at 1920x1080, written into docs/images/. Not part of `just e2e` (its 5-minute budget) nor
// of `just validate` beyond lint and typecheck: it is a documentation tool, run when the UI changes.
export default defineConfig({
  testDir: "./readme-shots",
  globalSetup: "./e2e/global-setup.ts",
  outputDir: "./e2e-results/readme-shots-artifacts",
  reporter: [["list"]],
  // One stack at a time keeps the run deterministic and light.
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 3 * 60_000,
  use: {
    ...devices["Desktop Chrome"],
    viewport: { width: 1920, height: 1080 },
    trace: "retain-on-failure",
    launchOptions: { ignoreDefaultArgs: ["--hide-scrollbars"] },
    // Deterministic pictures: no CSS animation or caret blink caught mid-way.
    colorScheme: "dark",
    locale: "en-US",
  },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
});
