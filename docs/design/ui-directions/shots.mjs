// Screenshots every design-direction page at the two viewports (1707x1300, the user's table
// screen; 1440x900) with Playwright's Chromium, into <direction>/shots/<page>-<WxH>.png, and
// fails when any element clips its text (scrollWidth > clientWidth) or the page scrolls: the
// no-cut rule, checked rather than eyeballed. Builds the pages first. Run `just design-shots`
// (needs web/node_modules: `cd web && npm ci`, and Playwright's Chromium: `just e2e-install`).
import { mkdirSync, readdirSync, existsSync } from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { build } from "./build.mjs";

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, "../../..");
const { chromium } = createRequire(path.join(root, "web/package.json"))("@playwright/test");

const VIEWPORTS = [
  { width: 1707, height: 1300 },
  { width: 1440, height: 900 },
];

/** Elements whose text is wider than the box that shows it, and whether the page scrolls. */
const audit = () => {
  const clipped = [];
  for (const el of document.querySelectorAll("body *")) {
    if (el.closest("[data-scroll-x]")) continue;
    if (el.clientWidth > 0 && el.scrollWidth > el.clientWidth + 1 && el.textContent.trim())
      clipped.push(
        `<${el.tagName.toLowerCase()} class="${el.className}"> ${el.scrollWidth}>${el.clientWidth}: ${el.textContent.trim().slice(0, 60)}`,
      );
  }
  const root = document.documentElement;
  return {
    clipped,
    pageScroll: root.scrollWidth > root.clientWidth || root.scrollHeight > root.clientHeight,
  };
};

const files = build().filter((f) => f.endsWith(".html"));
const browser = await chromium.launch();
let failed = 0;
for (const file of files) {
  const dir = path.dirname(file);
  const name = path.basename(file, ".html");
  const outDir = path.join(dir, "shots");
  mkdirSync(outDir, { recursive: true });
  for (const vp of VIEWPORTS) {
    const page = await browser.newPage({ viewport: vp, deviceScaleFactor: 1 });
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto(pathToFileURL(file).href);
    await page.evaluate(() => document.fonts.ready);
    const out = path.join(outDir, `${name}-${vp.width}x${vp.height}.png`);
    await page.screenshot({ path: out, fullPage: name === "index" });
    const r = await page.evaluate(audit);
    const rel = path.relative(root, out);
    const scrolls = r.pageScroll && name !== "index";
    if (r.clipped.length || scrolls) {
      failed++;
      console.error(`FAIL ${rel}`);
      if (scrolls) console.error("  the page scrolls");
      for (const c of r.clipped) console.error(`  clipped: ${c}`);
    } else console.log(`ok   ${rel}`);
    await page.close();
  }
}
await browser.close();
if (existsSync(path.join(here, "shared")))
  for (const f of readdirSync(path.join(here, "shared"))) if (!f.endsWith(".css")) console.warn(`stray file shared/${f}`);
if (failed) {
  console.error(`${failed} screenshot(s) with clipped text or page scroll`);
  process.exit(1);
}
