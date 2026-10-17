import { mkdir, readFile, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";
import type { Page } from "@playwright/test";
import { test, expect } from "../e2e/fixtures";
import { attackForm, chooseSetting, logEntry } from "../e2e/helpers";
import { clippedText } from "../e2e/noClip";
import {
  OPTIONS,
  hex,
  measured,
  pairs,
  themeCss,
  wcagRatio,
  type Option,
  type Theme,
} from "./mockup-tokens";
import {
  VIEWPORTS,
  focus,
  journal,
  recordHpT,
  seedTable,
  translator,
  webDir,
  type T,
} from "./table";
import { linear } from "./wcag";

/**
 * The mock-up round of the UI review (docs/design/ui-review/mockups/README.md): each option of the
 * review's "Recommendations" painted over the real app (its tokens from mockup-tokens.ts, its
 * component rules from docs/design/ui-review/mockups/css/), on the same seeded table, so every shot
 * compares with today's on identical data. Screens: Act · Weapons after an attack, the spell lookup
 * on Fireball, the inventory; at 1707x1300 and 1440x900, dark and light. Every capture, English and
 * French, is audited for cut text (ADR 0022 step 1) and the run fails on any. Also writes each
 * option's theme sheet (swatches with measured ratios) and themes.md (the numbers).
 *
 * Output: web/e2e-results/ui-mockups/ (everything, gitignored); the English shots, the French Act
 * shots of the option that moves geometry, and the theme sheets also go to its commit/ folder,
 * which `ui-review/to-webp.mjs` converts to lossless WebP (identical pixels, 40 % of the PNG) in
 * docs/design/ui-review/mockups/shots/. Run with `just ui-mockups`; not part of `just validate` nor
 * of `just e2e`.
 */

const repo = path.resolve(webDir, "..");
const docs = path.join(repo, "docs", "design", "ui-review", "mockups");
const cssDir = path.join(docs, "css");
/** The PNGs to commit; `ui-review/to-webp.mjs` turns them into lossless WebP in docs/…/mockups/shots/. */
const committed = path.join(webDir, "e2e-results", "ui-mockups", "commit");
const scratch = path.join(webDir, "e2e-results", "ui-mockups");

const SCREENS = ["act-weapons", "lookup", "inventory"] as const;
type Screen = (typeof SCREENS)[number];
/** Options whose French Act screen is committed too: they move geometry (bands), so FR length matters. */
const FR_COMMITTED = new Set(["o13-raised-stage"]);

async function optionCss(o: Option): Promise<string> {
  const parts = await Promise.all(
    o.css.map((name) => readFile(path.join(cssDir, `${name}.css`), "utf8")),
  );
  return parts.join("\n");
}

/** Select the theme's base ambiance and mode in the app, then paint the option over it. */
async function paint(page: Page, t: T, o: Option, theme: Theme) {
  await chooseSetting(page, t(`settings.mode.${theme.mode}`));
  await chooseSetting(page, t(`settings.ambiances.${theme.base}`));
  const css = o.id === "today" ? "" : themeCss(theme) + (await optionCss(o));
  await page.evaluate(
    ({ css, id }) => {
      let el = document.getElementById("mock-overlay");
      if (!el) {
        el = document.createElement("style");
        el.id = "mock-overlay";
      }
      document.head.appendChild(el); // last, after the app's styles
      el.textContent = css;
      if (css) document.documentElement.dataset.mockTheme = id;
      else delete document.documentElement.dataset.mockTheme;
    },
    { css, id: theme.id },
  );
}

/** Mark the header's last-result chip with the verdict the result card shows (CSS can't read text). */
async function annotate(page: Page) {
  await page.evaluate(() => {
    const chip = document.querySelector("[data-testid=last-result-chip]");
    const verdict = document.querySelector("[data-testid=verdict]");
    if (!chip || !verdict) return;
    const good = verdict.classList.contains("text-success");
    const bad = verdict.classList.contains("text-danger");
    if (good || bad)
      (chip as HTMLElement).dataset.mockVerdict = good ? "good" : "bad";
  });
}

async function show(page: Page, t: T, screen: Screen) {
  if (screen === "act-weapons") {
    await focus(page, t, "act");
    await page.getByRole("tab", { name: t("actions.tabs.weapons") }).click();
  } else if (screen === "lookup") {
    await focus(page, t, "spells");
    await page.locator("#lookup-row-fireball").click();
    await expect(page.getByTestId("spell-detail").locator("h3")).toBeVisible();
  } else {
    await focus(page, t, "inventory");
  }
  await annotate(page);
  await page.waitForTimeout(150); // colour transitions
}

/** The no-cut audit of the whole page, plus "the page scrolls". */
async function audit(page: Page): Promise<string[]> {
  const cut = await page.locator("body").evaluate(clippedText);
  const scrolls = await page.evaluate(
    () =>
      document.documentElement.scrollHeight >
      document.documentElement.clientHeight + 1,
  );
  return scrolls ? [...cut, "the page scrolls"] : cut;
}

/** Damage, undone, then a fresh attack: the journal shows an undone row, the card the attack. */
async function seedMockTable(page: Page, apiUrl: string, t: T) {
  await seedTable(page, apiUrl);
  await recordHpT(page, t, "damage", 60, "Dragon breath");
  await expect(logEntry(page, "Dragon breath")).toBeVisible();
  await page.getByTestId("undo").click();
  await expect(journal(page, t).getByRole("listitem").first()).toHaveAttribute(
    "data-status",
    "voided",
  );
  await attackForm(page)
    .getByRole("button", { name: "Attack", exact: true })
    .click();
  await expect(
    journal(page, t).getByRole("listitem").first(),
  ).not.toHaveAttribute("data-status", "voided");
}

const fmt = (x: number) => x.toFixed(2);
const fmtPair = (p: ReturnType<typeof pairs>[number]) => {
  const v = measured(p);
  const shown = p.dL !== undefined ? `ΔL ${fmt(v)}` : fmt(v);
  return `${shown}${p.min && v < p.min ? " ✗" : ""}`;
};

/** One option's theme sheet: a tile per theme, in its own colours, with its measured pairs. */
function themeSheet(o: Option): string {
  const tiles = o.themes().map((th) => {
    const ps = pairs(th);
    const by = (label: string) => ps.find((p) => p.label === label)!;
    const style = Object.entries(th.vars)
      .map(([k, v]) => `${k}:${v}`)
      .concat([
        `--ink-muted:color-mix(in oklab, var(--ink) ${th.mix.muted}%, transparent)`,
        `--border:color-mix(in oklab, var(--ink) ${th.mix.border}%, transparent)`,
        `--input:color-mix(in oklab, var(--ink) ${th.mix.input}%, transparent)`,
      ])
      .join(";");
    const accent = (name: string, label: string) => {
      const p = by(`${name} on surface`);
      return `<span class="acc" style="color:var(--${name})">${label} <b>${fmtPair(p)}</b></span>`;
    };
    const apart = ps
      .filter((p) => p.kind === "apart")
      .map(
        (p) =>
          `<span class="${p.min && measured(p) < p.min ? "bad" : ""}">${p.label.replace(" off ", "/")} ${fmtPair(p)}</span>`,
      )
      .join("");
    const swatches = [
      "ground",
      "surface",
      "neu-900",
      "tint",
      "highlight",
      "magic",
      "danger",
      "success",
      "warning",
    ]
      .map(
        (k) =>
          `<span class="sw" title="${k}"><i style="background:var(--${k})"></i>${k === "neu-900" ? "raised" : k}<br><code>${hex(linearOf(th, k))}</code></span>`,
      )
      .join("");
    return `<section class="tile ${th.status}" style="${style}">
  <h2>${th.name} <small>${th.mode}</small> <em>${th.status}</em></h2>
  <p class="why">${th.why}</p>
  <div class="sws">${swatches}</div>
  <div class="card">
    <p class="ink">Ink ${fmtPair(by("ink on surface"))} · <span class="muted">muted ${fmtPair(by("muted on surface"))}</span></p>
    <p>${accent("tint", "Tint")} ${accent("highlight", "Gold")} ${accent("magic", "Magic")}</p>
    <p>${accent("danger", "Danger")} ${accent("success", "Success")} ${accent("warning", "Warning")}</p>
    <p class="fills"><span style="background:var(--tint);color:var(--on-accent)">Attack ${fmtPair(by("on-accent on tint"))}</span><span style="background:var(--danger);color:var(--on-accent)">Miss</span><span style="background:var(--success);color:var(--on-accent)">Hit</span><span style="background:var(--highlight-fill);color:var(--on-highlight)">Crit ${fmtPair(by("on-highlight on highlight-fill"))}</span></p>
    <div class="raised">Raised step ${fmtPair(by("raised on card"))} · muted on it ${fmtPair(by("muted on raised"))}</div>
    <div class="field">Field edge ${fmtPair(by("field edge on card"))}</div>
  </div>
  <p class="meta">Card on page ${fmtPair(by("card on page"))} · outline ${fmt(wcagRatio(by("card outline on page")))}</p>
  <p class="apart">${apart}</p>
</section>`;
  });
  const font = (pkg: string, file: string) =>
    pathToFileURL(
      path.join(
        webDir,
        "node_modules",
        "@fontsource-variable",
        pkg,
        "files",
        file,
      ),
    ).href;
  return `<!doctype html><html><head><meta charset="utf-8"><style>
@font-face{font-family:Inter;src:url(${font("inter", "inter-latin-wght-normal.woff2")});font-weight:100 900}
body{margin:0;padding:16px;background:#777;font:13px/18px Inter,sans-serif;font-variant-numeric:tabular-nums}
h1{font-size:16px;margin:0 0 12px;color:#fff}
.grid{display:grid;grid-template-columns:repeat(4,1fr);gap:12px}
.tile{background:var(--ground);color:var(--ink);border-radius:10px;padding:12px;display:flex;flex-direction:column;gap:8px}
.tile.drop{opacity:.55}
h2{font-size:14px;margin:0;display:flex;gap:8px;align-items:baseline}
h2 small{font-weight:400;color:var(--ink-muted)}
h2 em{margin-left:auto;font-style:normal;font-size:11px;text-transform:uppercase;letter-spacing:.08em;padding:0 6px;border-radius:6px;background:var(--neu-900)}
.drop h2 em{background:var(--danger);color:var(--on-accent)}
.new h2 em{background:var(--highlight-fill);color:var(--on-highlight)}
.why{margin:0;color:var(--ink-muted);min-height:36px}
.sws{display:grid;grid-template-columns:repeat(9,1fr);gap:2px;font-size:9px;line-height:11px;color:var(--ink-muted)}
.sw i{display:block;height:18px;border-radius:3px;border:1px solid var(--border)}
.sw code{font-size:8px}
.card{background:var(--surface);border:1px solid var(--border);border-radius:8px;padding:10px;display:flex;flex-direction:column;gap:6px}
.card p{margin:0;display:flex;flex-wrap:wrap;gap:4px 10px}
.muted{color:var(--ink-muted)}
.acc b{font-weight:600}
.fills span{border-radius:6px;padding:1px 6px;font-weight:600}
.raised{background:var(--neu-900);border-radius:6px;padding:6px 8px;color:var(--ink-muted)}
.field{border:1px solid var(--input);border-radius:6px;padding:4px 8px;color:var(--ink-muted)}
.meta,.apart{margin:0;font-size:11px;color:var(--ink-muted);display:flex;flex-wrap:wrap;gap:2px 10px}
.bad{color:var(--warning)}
</style></head><body><h1>${o.name}: theme set (WCAG 2 ratios; ✗ under the floor: text 4.5, field edge 3, accent off tint 1.8, tone steps by OKLab ΔL .10 card / .07 raised)</h1>
<div class="grid">${tiles.join("\n")}</div></body></html>`;
}

const linearOf = (th: Theme, k: string) => linear(th.vars[`--${k}`]);

/** themes.md: per option, its theme set and every measured pair. */
function themesMd(): string {
  const out: string[] = [
    "# Theme sets of the mock-up round (generated)",
    "",
    "Generated by `just ui-mockups` (`web/ui-review/mockups.spec.ts`) from",
    "`web/ui-review/mockup-tokens.ts`: the themes each option keeps, retunes, adds or drops, and",
    "the measured contrast of each. WCAG 2 ratios; `✗` under the floor: text 4.5:1, field edge and",
    "ring 3:1, an accent off the primary 1.8:1 (the review's proposed rule). Tone steps are judged by",
    "their OKLab lightness difference (ΔL), because WCAG ratios flatten dark tones (L .12 vs .235",
    "reads as a clear step at 1.2:1): card on page ΔL ≥ .10, raised block on card ΔL ≥ .07. The card",
    "outline is informational: with the tone step it no longer carries the boundary alone.",
    "",
  ];
  for (const o of OPTIONS) {
    const themes = o.themes();
    out.push(`## ${o.name}`, "");
    out.push("| Theme | Mode | Status | Why |", "|---|---|---|---|");
    for (const th of themes)
      out.push(`| ${th.name} | ${th.mode} | ${th.status} | ${th.why} |`);
    out.push("");
    const shown = themes.filter((th) => th.status !== "drop");
    out.push(
      `| Pair | floor | ${shown.map((th) => `${th.name} ${th.mode}`).join(" | ")} |`,
      `|---|---|${shown.map(() => "---").join("|")}|`,
    );
    const labels = pairs(shown[0]).map((p) => p.label);
    for (const label of labels) {
      const ps = shown.map((th) => pairs(th).find((p) => p.label === label)!);
      const floor = ps[0].min
        ? ps[0].dL !== undefined
          ? `ΔL ${ps[0].min}`
          : String(ps[0].min)
        : "–";
      out.push(`| ${label} | ${floor} | ${ps.map(fmtPair).join(" | ")} |`);
    }
    out.push("");
    const roles = [
      "ground",
      "surface",
      "neu-900",
      "ink",
      "tint",
      "highlight",
      "highlight-fill",
      "magic",
      "danger",
      "success",
      "warning",
      "amb-800",
    ];
    out.push(
      `| Token | ${shown.map((th) => `${th.name} ${th.mode}`).join(" | ")} |`,
      `|---|${shown.map(() => "---").join("|")}|`,
    );
    for (const r of roles)
      out.push(
        `| ${r} | ${shown.map((th) => `\`${th.vars[`--${r}`]}\``).join(" | ")} |`,
      );
    out.push(
      `| mixes (muted / border / input) | ${shown.map((th) => `${th.mix.muted} / ${th.mix.border} / ${th.mix.input} %`).join(" | ")} |`,
      "",
    );
  }
  return out.join("\n");
}

test("mock-ups: every option on the same table, EN and FR, both viewports, dark and light", async ({
  page,
  stack,
}) => {
  test.setTimeout(30 * 60_000);
  await rm(committed, { recursive: true, force: true });
  await mkdir(committed, { recursive: true });
  const problems: string[] = [];
  const en = await translator("en");
  await seedMockTable(page, stack.apiUrl, en);

  // Theme sheets and the numbers first: they need no app state.
  await writeFile(path.join(docs, "themes.md"), themesMd());
  for (const o of OPTIONS) {
    const file = path.join(scratch, `${o.id}--themes.html`);
    await writeFile(file, themeSheet(o));
    const sheet = await page.context().newPage();
    await sheet.setViewportSize({ width: 1440, height: 900 });
    await sheet.goto(pathToFileURL(file).href);
    await sheet.evaluate(() => document.fonts.ready);
    await sheet.screenshot({
      path: path.join(committed, `${o.id}--themes.png`),
      fullPage: true,
    });
    await sheet.close();
  }

  for (const lang of ["en", "fr"] as const) {
    const t = await translator(lang);
    if (lang === "fr") await chooseSetting(page, "Français");
    for (const vp of VIEWPORTS) {
      await page.setViewportSize(vp);
      const size = `${vp.width}x${vp.height}`;
      for (const o of OPTIONS) {
        const themes = o.themes();
        const runs: {
          theme: string;
          screens: readonly Screen[];
          only?: string;
        }[] = [
          { theme: o.capture.dark, screens: SCREENS },
          { theme: o.capture.light, screens: SCREENS },
          ...(lang === "en"
            ? (o.extra ?? []).map((x) => ({
                theme: x.theme,
                screens: [x.screen as Screen],
                only: x.viewport,
              }))
            : []),
        ];
        for (const run of runs) {
          if (run.only && run.only !== size) continue;
          const theme = themes.find((th) => th.id === run.theme)!;
          await paint(page, t, o, theme);
          for (const screen of run.screens) {
            await show(page, t, screen);
            const name = `${o.id}--${screen}--${theme.id}${lang === "fr" ? "-fr" : ""}-${size}.png`;
            await page.screenshot({
              path: path.join(scratch, name),
              animations: "disabled",
            });
            if (
              lang === "en" ||
              (FR_COMMITTED.has(o.id) && screen === "act-weapons")
            )
              await page.screenshot({
                path: path.join(committed, name),
                animations: "disabled",
              });
            for (const c of await audit(page)) problems.push(`${name}: ${c}`);
          }
        }
      }
    }
  }
  expect(problems, "cut text or page scroll in a mock-up").toEqual([]);
});
