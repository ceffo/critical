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
  const describe = (el, how) =>
    `<${el.tagName.toLowerCase()} class="${el.className}"> ${how}: ${el.textContent.trim().slice(0, 60)}`;
  for (const el of document.querySelectorAll("body *")) {
    if (el.closest("[data-scroll-x]") || !el.textContent.trim()) continue;
    if (el.clientWidth > 0 && el.scrollWidth > el.clientWidth + 1)
      clipped.push(describe(el, `${el.scrollWidth}>${el.clientWidth} wide`));
    // Vertical: only boxes that hide their overflow (a scrolling box is fine, it scrolls).
    const oy = getComputedStyle(el).overflowY;
    if ((oy === "hidden" || oy === "clip") && el.clientHeight > 0 && el.scrollHeight > el.clientHeight + 1)
      clipped.push(describe(el, `${el.scrollHeight}>${el.clientHeight} tall`));
  }
  // body hides its overflow, so html's scroll size never grows: check both boxes.
  const over = (el) => el.scrollWidth > el.clientWidth || el.scrollHeight > el.clientHeight;
  return { clipped, pageScroll: over(document.documentElement) || over(document.body) };
};

const browser = await chromium.launch();

// `--self-test`: prove the audit catches what it must (a cut line, a box hiding tall content,
// a page taller than the viewport) before trusting the green run.
if (process.argv.includes("--self-test")) {
  const page = await browser.newPage({ viewport: { width: 800, height: 400 } });
  await page.setContent(
    `<style>html,body{height:100%;overflow:hidden;margin:0}</style>
<div style="width:50px;overflow:hidden;white-space:nowrap">a very long line of text</div>
<div style="height:100px;overflow:hidden"><p style="height:300px">tall text</p></div>
<div style="height:600px">spills past the viewport</div>`,
  );
  const r = await page.evaluate(audit);
  const wide = r.clipped.some((c) => c.includes("wide"));
  const tall = r.clipped.some((c) => c.includes("tall"));
  if (!wide || !tall || !r.pageScroll) {
    console.error("self-test failed:", JSON.stringify(r));
    process.exit(1);
  }
  console.log("self-test ok: cut line, hidden tall content and page overflow are all reported");
  await browser.close();
  process.exit(0);
}

const files = build().filter((f) => f.endsWith(".html"));
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
