// Screenshots of the onboarding design round (docs/design/onboarding/README.md): every mock-up
// screen of web/design/onboarding/ in French and English at the two layout viewports the item asks
// (1280x800 and 2560x1440, ADR 0022 step 1), dark Arcane, into shots/<screen>--<lang>-<WxH>.png.
// Fails when any element cuts its text (wider or taller than the box that hides it) or the page
// scrolls: the no-cut rule, checked rather than eyeballed. `--self-test` proves the audit catches
// both. Run `just onboarding-shots` (needs web/node_modules and Playwright's Chromium).
import { mkdirSync } from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, "../../..");
const web = path.join(root, "web");
const { chromium } = createRequire(path.join(web, "package.json"))("@playwright/test");
const { createServer } = await import(pathToFileURL(path.join(web, "node_modules/vite/dist/node/index.js")).href);

const SCREENS = [
  "intro",
  "1-class",
  "2-race",
  "3-scores",
  "4-hp-skills",
  "5-identity",
  "6-review",
  "roster",
  "character-card",
  "cog-menu",
];
const LANGS = ["fr", "en"];
const VIEWPORTS = [
  { width: 1280, height: 800 },
  { width: 2560, height: 1440 },
];

/** Elements whose text is wider (or taller) than the box that hides it, and whether the page scrolls. */
const audit = () => {
  const clipped = [];
  const describe = (el, how) =>
    `<${el.tagName.toLowerCase()} class="${String(el.className).slice(0, 60)}"> ${how}: ${el.textContent.trim().slice(0, 60)}`;
  for (const el of document.querySelectorAll("body *")) {
    if (!el.textContent.trim() || el.tagName === "svg" || el.closest("svg")) continue;
    const s = getComputedStyle(el);
    const scrolls = (v) => v === "auto" || v === "scroll";
    if (!scrolls(s.overflowX) && el.clientWidth > 0 && el.scrollWidth > el.clientWidth + 1)
      clipped.push(describe(el, `${el.scrollWidth}>${el.clientWidth} wide`));
    if ((s.overflowY === "hidden" || s.overflowY === "clip") && el.clientHeight > 0 && el.scrollHeight > el.clientHeight + 1)
      clipped.push(describe(el, `${el.scrollHeight}>${el.clientHeight} tall`));
  }
  const over = (el) => el.scrollWidth > el.clientWidth || el.scrollHeight > el.clientHeight;
  return { clipped, pageScroll: over(document.documentElement) || over(document.body) };
};

const browser = await chromium.launch();

if (process.argv.includes("--self-test")) {
  const page = await browser.newPage({ viewport: { width: 800, height: 400 } });
  await page.setContent(
    `<style>html,body{height:100%;overflow:hidden;margin:0}</style>
<div style="width:50px;overflow:hidden;white-space:nowrap">a very long line of text</div>
<div style="height:100px;overflow:hidden"><p style="height:300px">tall text</p></div>
<div style="height:600px">spills past the viewport</div>`,
  );
  const r = await page.evaluate(audit);
  const ok = r.clipped.some((c) => c.includes("wide")) && r.clipped.some((c) => c.includes("tall")) && r.pageScroll;
  await browser.close();
  if (!ok) {
    console.error("self-test failed:", JSON.stringify(r));
    process.exit(1);
  }
  console.log("self-test ok: a cut line, hidden tall content and a page scroll are all reported");
  process.exit(0);
}

const server = await createServer({ configFile: path.join(web, "design/onboarding/vite.config.ts") });
await server.listen();
const base = server.resolvedUrls.local[0];
const outDir = path.join(here, "shots");
mkdirSync(outDir, { recursive: true });

let failed = 0;
for (const screen of SCREENS)
  for (const lang of LANGS)
    for (const vp of VIEWPORTS) {
      const page = await browser.newPage({ viewport: vp, deviceScaleFactor: 1 });
      await page.emulateMedia({ reducedMotion: "reduce" });
      await page.goto(`${base}?screen=${screen}&lang=${lang}`);
      await page.waitForSelector("body[data-ready] #root *");
      await page.evaluate(() => document.fonts.ready);
      const out = path.join(outDir, `${screen}--${lang}-${vp.width}x${vp.height}.png`);
      await page.screenshot({ path: out });
      const r = await page.evaluate(audit);
      const rel = path.relative(root, out);
      if (r.clipped.length || r.pageScroll) {
        failed++;
        console.error(`FAIL ${rel}`);
        if (r.pageScroll) console.error("  the page scrolls");
        for (const c of r.clipped) console.error(`  clipped: ${c}`);
      } else console.log(`ok   ${rel}`);
      // A step whose card scrolls at this size: a second, full-height shot shows all of it (French).
      const hidden = await page.evaluate(() =>
        Math.max(0, ...[...document.querySelectorAll("*")].map((el) =>
          getComputedStyle(el).overflowY === "auto" ? el.scrollHeight - el.clientHeight : 0)),
      );
      if (hidden > 0 && lang === "fr") {
        await page.setViewportSize({ width: vp.width, height: vp.height + hidden });
        const full = out.replace(/\.png$/, "-full.png");
        await page.screenshot({ path: full });
        console.log(`     ${path.relative(root, full)} (the card scrolls ${hidden} px at ${vp.width}x${vp.height})`);
      }
      await page.close();
    }
// The 2560 question: the creation frame capped at 1680 px and centred, beside the full-width shots.
for (const screen of ["3-scores", "5-identity"]) {
  const page = await browser.newPage({ viewport: { width: 2560, height: 1440 }, deviceScaleFactor: 1 });
  await page.goto(`${base}?screen=${screen}&lang=fr&cap=1`);
  await page.waitForSelector("body[data-ready] #root *");
  await page.evaluate(() => document.fonts.ready);
  const out = path.join(outDir, `${screen}--fr-2560x1440-capped.png`);
  await page.screenshot({ path: out });
  const r = await page.evaluate(audit);
  if (r.clipped.length || r.pageScroll) {
    failed++;
    console.error(`FAIL ${path.relative(root, out)}`, r);
  } else console.log(`ok   ${path.relative(root, out)}`);
  await page.close();
}
await browser.close();
await server.close();
if (failed) {
  console.error(`${failed} screenshot(s) with cut text or a page scroll`);
  process.exit(1);
}
