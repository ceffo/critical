import { defineConfig, devices } from "@playwright/test";

// Artifacts (all gitignored): e2e-results/{artifacts,report,screens}.
// `outputDir` is wiped on every run, so screenshots live in a sibling dir.
export default defineConfig({
  testDir: "./e2e",
  globalSetup: "./e2e/global-setup.ts",
  outputDir: "./e2e-results/artifacts",
  reporter: [
    ["list"],
    ["html", { outputFolder: "./e2e-results/report", open: "never" }],
  ],
  // Every test gets its own API + Vite stack on free ports (see e2e/fixtures.ts),
  // so tests are independent and can run in parallel.
  fullyParallel: true,
  // CI: one retry for infra flakes; 3 workers on the free runner's 4 vCPU (each worker serves the built
  // bundle once, each test boots only its own API; a 4th worker only adds contention: measured, the suite
  // is CPU-bound, and starved tests time out). Locally: no retries, default workers.
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 3 : undefined,
  use: {
    ...devices["Desktop Chrome"],
    viewport: { width: 1440, height: 900 },
    // Recording a trace costs about a quarter of a test's time: only the retry of a failure records one.
    trace: "on-first-retry",
    // Headless Chromium hides scrollbars by default; show them so the screenshots carry the app's
    // styled scrollbars and scrollers lay out as on a desktop browser.
    launchOptions: { ignoreDefaultArgs: ["--hide-scrollbars"] },
  },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
});
