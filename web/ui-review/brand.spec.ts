import { mkdir, readFile, rm } from "node:fs/promises";
import path from "node:path";
import type { Page } from "@playwright/test";
import { test, expect } from "../e2e/fixtures";
import { chooseSetting } from "../e2e/helpers";
import { OPTIONS } from "./mockup-tokens";
import { annotate, audit, paint, seedMockTable } from "./paint";
import { focus, translator, webDir, type T } from "./table";

/**
 * The brand placement round (docs/design/brand-placement/README.md): where the mark and the
 * "Critical!" wordmark sit in the header. The real app on the mock-up round's seeded table, in
 * French, painted with the picked colours (contrast ladder + raised stage, ADR 0026), on Act ·
 * Weapons. Only the header is drawn: the decided header of ADR 0027 step 4 (turn stacked,
 * initiative under "Tour suivant", 5 turn columns, last result on one line) in each placement,
 * filled with the real header's values read from the page, styled by
 * docs/design/brand-placement/header.css. Every shot is audited for cut text (ADR 0022 step 1),
 * a page scroll, a header over 120 px (ADR 0022 step 3.1) and a wordmark not shown in full.
 *
 * Output: docs/design/brand-placement/shots/ (PNG). Run with `just ui-brand`; not part of
 * `just validate` nor of `just e2e`.
 */

const repo = path.resolve(webDir, "..");
const docs = path.join(repo, "docs", "design", "brand-placement");
const shots = path.join(docs, "shots");

export const PLACEMENTS = [
  { id: "a-left", cls: "bp-left" },
  { id: "b-column", cls: "bp-column" },
  { id: "c-right", cls: "bp-right" },
] as const;
type Placement = (typeof PLACEMENTS)[number];

/** Dark Arcane at both layout viewports; light Arcane at 1440x900 only (one light shot each). */
const RUNS = [
  {
    theme: "arcane-dark",
    viewports: [
      [1440, 900],
      [1707, 1300],
    ],
  },
  { theme: "arcane-light", viewports: [[1440, 900]] },
] as const;

/** The real header's values, read from the page so the drawing shows the seeded table. */
type HeaderData = {
  turn: string;
  initiative: string;
  hidden: string;
  effects: { name: string; turns: number; text: string; pips: number }[];
  chip: string;
  verdict: string;
  when: string;
  text: string;
  icons: Record<"next" | "menu" | "reroll" | "add" | "more", string>;
};

async function readHeader(page: Page): Promise<HeaderData> {
  return page.evaluate(() => {
    const h = document.querySelector(
      "[data-screen=turn-header]:not(.bp-header)",
    )!;
    const q = (s: string) => h.querySelector(s);
    const txt = (s: string) => q(s)?.textContent?.trim() ?? "";
    const svg = (el: Element | null | undefined) =>
      el?.querySelector("svg")?.outerHTML ?? "";
    const buttons = [...h.querySelectorAll("button")];
    const byText = (s: string) =>
      buttons.find((b) => b.textContent?.includes(s));
    const effects = [...h.querySelectorAll("[data-testid=buff]")].map((b) => {
      const rounds =
        b.querySelector("[data-testid=buff-rounds]")?.textContent ?? "";
      const charges =
        b.querySelector("[data-testid=buff-charges]")?.textContent ?? "";
      return {
        name: b.querySelector(".break-words > span")?.textContent?.trim() ?? "",
        turns: parseInt(rounds, 10) || 0,
        text: (rounds || charges).trim(),
        pips: b.querySelectorAll("[data-testid=pip-left]").length,
      };
    });
    const label = q("[data-testid=last-result] .section-label span");
    return {
      turn: txt("[data-testid=round-value]"),
      initiative: txt("[data-testid=initiative-value]"),
      hidden: txt("[data-testid=effects-expander]"),
      effects,
      chip: txt("[data-testid=last-result-chip]"),
      verdict:
        (q("[data-testid=last-result-chip]") as HTMLElement | null)?.dataset
          .mockVerdict ?? "good",
      when: label?.textContent?.replace(/^\s*·\s*/, "").trim() ?? "",
      text: txt("[data-testid=last-result-text]"),
      icons: {
        next: svg(byText("Tour suivant")),
        menu: svg(q("[aria-haspopup]") ?? q("button[aria-label^='Autres']")),
        reroll: svg(q("[data-testid=initiative-row] button")),
        add: svg(byText("Ajouter")),
        more: svg(q("[data-testid=effects-expander]")),
      },
    };
  });
}

const MARK = `<svg viewBox="0 0 32 32" aria-hidden="true"><polygon points="16,2.5 27.7,9.25 27.7,22.75 16,29.5 4.3,22.75 4.3,9.25"/><rect class="bang" x="14.5" y="8.5" width="3" height="9.5" rx="1.5"/><circle class="bang" cx="16" cy="21.9" r="1.8"/></svg>`;

const COLUMNS = 5;

function headerHtml(p: Placement, d: HeaderData, t: T): string {
  const brand = `<div class="bp-brand" data-testid="bp-brand">${MARK}<span class="bp-wordmark" data-testid="bp-wordmark">Critical<b>!</b></span></div>`;
  const turn = `<div class="bp-turn"><span class="bp-label">${t("turn.title")}</span><span class="num">${d.turn}</span></div>`;
  const next = `<div class="bp-next">
    <div class="row"><span class="bp-btn">${t("turn.next")}${d.icons.next}</span><span class="bp-icon-btn">${d.icons.menu}</span></div>
    <div class="bp-init" title="${t("turn.initiativeHint")}">${t("turn.initiative")} <span class="num">${d.initiative}</span><span class="bp-icon-btn quiet">${d.icons.reroll}</span></div>
  </div>`;
  const turnNo = Number(d.turn);
  const cols = Array.from({ length: COLUMNS }, (_, i) => {
    const n = turnNo + i;
    return `<span class="col${i === 0 ? " cur" : ""}" style="grid-column:${i + 2}">${t("effects.column", { n })}</span>`;
  }).join("");
  const rows = d.effects
    .slice(0, 3)
    .map((e, i) => {
      const row = i + 2;
      const span = e.turns > 0 ? Math.min(e.turns, COLUMNS) : COLUMNS;
      const fades = e.turns === 0 || e.turns > COLUMNS;
      const body =
        e.pips > 0
          ? `<span class="pips">${"<i></i>".repeat(e.pips)} ${e.text}</span>`
          : `<span class="pill${fades ? " fade" : ""}">${e.text}</span>`;
      return `<span class="name" style="grid-row:${row}">${e.name}</span><div style="grid-column:2 / span ${span};grid-row:${row};min-width:0">${body}</div>`;
    })
    .join("");
  const effects = `<div class="bp-effects"><div class="now"></div>
    <div class="head"><span class="title">${t("effects.title")}</span><span class="quiet">${d.icons.add}${t("buffs.addShort")}</span><span class="quiet">${d.icons.more}${d.hidden}</span></div>
    ${cols}${rows}</div>`;
  const last = `<div class="bp-last" data-testid="bp-last"><span class="bp-label">${t("shell.lastResult")} <span class="when">· ${d.when}</span></span>
    <p class="line" style="margin:0"><span class="chip${d.verdict === "bad" ? " bad" : ""}">${d.chip}</span><span>${d.text}</span></p></div>`;

  const inner =
    p.id === "a-left"
      ? `${brand}${turn}${next}${effects}${last}<span class="bp-spacer"></span>`
      : p.id === "b-column"
        ? `<div class="bp-cell">${brand}${turn}</div><div class="bp-rest">${next}${effects}${last}<span class="bp-spacer"></span></div>`
        : `${turn}${next}${effects}${last}${brand}`;
  return `<header class="bp-header ${p.cls}" data-screen="turn-header">${inner}</header>`;
}

/** Put the drawn header in front of the real one (which the stylesheet hides). */
async function draw(page: Page, css: string, html: string) {
  await page.evaluate(
    ({ css, html }) => {
      let style = document.getElementById("bp-style");
      if (!style) {
        style = document.createElement("style");
        style.id = "bp-style";
      }
      document.head.appendChild(style);
      style.textContent = css;
      document.querySelector(".bp-header")?.remove();
      const real = document.querySelector(
        "[data-screen=turn-header]:not(.bp-header)",
      )!;
      real.insertAdjacentHTML("beforebegin", html);
    },
    { css, html },
  );
  await page.evaluate(() => document.fonts.ready);
}

/** The round's own checks on the drawn header: height budget, wordmark shown in full. */
async function headerChecks(page: Page): Promise<string[]> {
  return page.evaluate(() => {
    const out: string[] = [];
    const h = document.querySelector(".bp-header") as HTMLElement;
    const height = h.getBoundingClientRect().height;
    if (height > 120) out.push(`header ${height}px tall (max 120)`);
    const w = document.querySelector(
      "[data-testid=bp-wordmark]",
    ) as HTMLElement;
    const r = w.getBoundingClientRect();
    const hr = h.getBoundingClientRect();
    if (r.width < 40 || r.right > hr.right || r.left < hr.left)
      out.push(`wordmark not shown in full (${r.left}-${r.right})`);
    return out;
  });
}

test("brand placement: each placement in French, Arcane dark at both viewports, light at 1440", async ({
  page,
  stack,
}) => {
  test.setTimeout(10 * 60_000);
  await rm(shots, { recursive: true, force: true });
  await mkdir(shots, { recursive: true });
  const en = await translator("en");
  await seedMockTable(page, stack.apiUrl, en);
  await chooseSetting(page, "Français");
  const fr = await translator("fr");
  const option = OPTIONS.find((o) => o.id === "o13-raised-stage")!;
  const css = await readFile(path.join(docs, "header.css"), "utf8");
  const problems: string[] = [];

  for (const run of RUNS) {
    const theme = option.themes().find((th) => th.id === run.theme)!;
    await paint(page, fr, option, theme);
    await focus(page, fr, "act");
    await page.getByRole("tab", { name: fr("actions.tabs.weapons") }).click();
    await annotate(page);
    await page.waitForTimeout(150);
    for (const [width, height] of run.viewports) {
      await page.setViewportSize({ width, height });
      await page.waitForTimeout(100);
      const data = await readHeader(page);
      expect(data.effects.length, "the seeded table shows 3 effects").toBe(3);
      for (const p of PLACEMENTS) {
        await draw(page, css, headerHtml(p, data, fr));
        const name = `${p.id}--${theme.id}-${width}x${height}.png`;
        await page.screenshot({
          path: path.join(shots, name),
          animations: "disabled",
        });
        for (const c of [...(await audit(page)), ...(await headerChecks(page))])
          problems.push(`${name}: ${c}`);
      }
    }
  }
  expect(problems, "cut text, page scroll or header over budget").toEqual([]);
});
