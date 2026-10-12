// Fails with a clear message when Playwright's Chromium isn't installed.
import { chromium } from "@playwright/test";
import { existsSync } from "node:fs";

if (!existsSync(chromium.executablePath())) {
  console.error(
    "\nPlaywright Chromium is not installed. Run once:  just e2e-install\n",
  );
  process.exit(1);
}
