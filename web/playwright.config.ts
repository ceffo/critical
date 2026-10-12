import { defineConfig, devices } from "@playwright/test";

// Artifacts (all gitignored): e2e-results/{artifacts,report,screens}.
// `outputDir` is wiped on every run, so screenshots live in a sibling dir.
export default defineConfig({
  testDir: "./e2e",
  outputDir: "./e2e-results/artifacts",
  reporter: [
    ["list"],
    ["html", { outputFolder: "./e2e-results/report", open: "never" }],
  ],
  // Every test gets its own API + Vite stack on free ports (see e2e/fixtures.ts),
  // so tests are independent and can run in parallel.
  fullyParallel: true,
  retries: 0,
  use: {
    ...devices["Desktop Chrome"],
    viewport: { width: 1440, height: 900 },
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
});
