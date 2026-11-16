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
  // CI: one retry for infra flakes; 5 workers on the self-hosted runner's 8 vCPU (each worker serves the built
  // bundle once, each test boots only its own API). The suite is CPU-bound: measured on 8 CPUs, 3 workers took 3.3 min,
  // 6 took 1.9, 5 took 2.2 and 8 took 1.6; 5 leaves room for the machine's other jobs, since starved tests time out.
  // Locally: no retries, default workers.
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 5 : undefined,
  // CI: the machine's other jobs can starve a test for seconds (a 5 s assertion timed out with load 13 on 8 CPUs);
  // a longer assertion timeout costs nothing when the page is ready.
  expect: { timeout: process.env.CI ? 15_000 : 5_000 },
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
