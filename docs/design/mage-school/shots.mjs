// Screenshots of the mage school design round (docs/design/mage-school/README.md, CRI-392): every
// mock-up screen of web/design/mage-school/ in French and English at the two layout viewports
// (1920x1080 and 2560x1440), dark Arcane (user: "no light theme please"), into
// shots/<screen>--<lang>-<WxH>.png, and the review gallery shots/index.html.
// Fails when any element cuts its text (wider or taller than the box that hides it) or the page
// scrolls: the no-cut rule, checked rather than eyeballed. `--self-test` proves the audit catches
// both. Run `just mage-school-shots` (needs web/node_modules and Playwright's Chromium).
import { execFileSync } from "node:child_process";
import { mkdirSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, "../../..");
const web = path.join(root, "web");
const { chromium } = createRequire(path.join(web, "package.json"))("@playwright/test");
const { createServer } = await import(pathToFileURL(path.join(web, "node_modules/vite/dist/node/index.js")).href);

// Each screen with the line the review gallery (shots/index.html) shows above it.
const GALLERY = [
  ["a-tag", "A · Étiquette « Choisir une école » sur la ligne de classe, tant que le magicien n'a pas d'école"],
  ["a-tag-dual", "A · La même étiquette pour un guerrier qui a pris Magicien en seconde classe"],
  ["a-tag-after", "A · Après le choix : la classe lit le spécialiste et son école, l'étiquette disparaît"],
  ["b-class-link", "B · Plus discret : le nom de la classe devient un lien (souligné pointillé, comme Niveau), avec son infobulle"],
  ["b-class-link-dual", "B · Guerrier qui a pris Magicien : le nom Magicien de la ligne des classes est le lien"],
  ["b-class-link-after", "B · Après le choix : le lien lit le spécialiste et son école, et rouvre le choix"],
  ["c-spells", "C · Dans Sorts : un bandeau « Magicien généraliste » en tête du grimoire, avec le bouton"],
  ["popover", "Commun · Le nom porte un chevron ; dans les détails, le bouton « École » est sur la ligne Classe (au lieu de « Changer »)"],
  ["picker", "Commun · Le choix montre les écoles, leurs écoles opposées, le minimum de caractéristique et l'avertissement de race"],
];
const SCREENS = GALLERY.map(([id]) => id);
// `--fr` shoots French only (the quick loop while iterating); the default shoots both.
const LANGS = process.argv.includes("--fr") ? ["fr"] : ["fr", "en"];
// `--only=a,b` shoots those screens only.
const only = process.argv.find((a) => a.startsWith("--only="))?.slice(7).split(",");
const VIEWPORTS = [
  { width: 1920, height: 1080 },
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

const browser = await chromium.launch({ ignoreDefaultArgs: ["--hide-scrollbars"] });

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

const server = await createServer({ configFile: path.join(web, "design/mage-school/vite.config.ts") });
await server.listen();
const base = server.resolvedUrls.local[0];
const outDir = path.join(here, "shots");
mkdirSync(outDir, { recursive: true });

let failed = 0;
for (const screen of SCREENS.filter((s) => !only || only.includes(s)))
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
      // A screen whose card scrolls at this size: a second, full-height shot shows all of it (French).
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
await browser.close();
await server.close();
writeGallery();
if (failed) {
  console.error(`${failed} screenshot(s) with cut text or a page scroll`);
  process.exit(1);
}

/** shots/index.html: every screen's French 1920x1080 shot under its line, with links to the
 * other language and size. Its frame takes the fleet's page theme (`fleet theme-css`). */
function writeGallery() {
  const esc = (t) => t.replace(/&/g, "&amp;").replace(/</g, "&lt;");
  let theme = "";
  try {
    theme = execFileSync(path.join(root, ".claude/bin/fleet"), ["theme-css"], { encoding: "utf8" });
  } catch {
    theme = "";
  }
  // A version stamp per run: the server sends no cache headers, so a reload would keep old shots.
  const v = Date.now().toString(36);
  const sections = GALLERY.map(([id, line]) => {
    const f = (lang, size) => `${id}--${lang}-${size}.png?v=${v}`;
    const links = [["en", "1920x1080"], ["fr", "2560x1440"], ["en", "2560x1440"]]
      .map(([lang, size]) => `<a href="${f(lang, size)}">${lang} ${size}</a>`)
      .join(" · ");
    return `<section id="${id}"><h2><a href="#${id}">${id}</a> — ${esc(line)}</h2>
<p class="links">${links}</p><a href="${f("fr", "1920x1080")}"><img src="${f("fr", "1920x1080")}" alt="${id}" loading="lazy"></a></section>`;
  }).join("\n");
  const toc = GALLERY.map(([id]) => `<a href="#${id}">${id}</a>`).join(" ");
  writeFileSync(
    path.join(outDir, "index.html"),
    `<!doctype html><meta charset="utf-8"><title>École de magie : où la choisir (CRI-392)</title>
<style>${theme}
body{max-width:1920px}img{width:100%;border:1px solid var(--border);border-radius:6px}
.links{margin:0 0 8px;font-size:13px}nav{position:sticky;top:0;background:var(--bg);padding:8px 0;font-size:13px}nav a{margin-right:10px}
.note{color:var(--muted)}</style>
<h1>École de magie : la rendre facile à trouver (CRI-392), tour 1</h1>
<p>Aujourd'hui : clic sur le nom (texte simple), puis « Changer » ; rien ne parle d'école avant la liste.
Trois emplacements proposés (A, B, C : un seul à retenir, ou A + C), plus deux changements communs (détails, choix).
Le pointillé magenta marque ce qui change ; il ne fait pas partie du design.</p>
<p class="note">Captures du jeu actuel : CRI-392-where.html (rapport du développeur).</p>
<nav>${toc}</nav>\n${sections}\n`,
  );
  console.log(`     ${path.relative(root, path.join(outDir, "index.html"))} (review gallery)`);
}
