import { expect, type Locator, type Page } from "@playwright/test";
import { snap } from "./fixtures";
import { combatLog } from "./helpers";
import { expectNoClippedText } from "./noClip";

export type Viewport = { width: number; height: number };

/**
 * The viewports of a walk: those of `list` that have the walk's width, or the walk's own size when the
 * list has none. A walk runs every step at its one size; the lists the steps were written with name the
 * sizes the defect shows at, and the walk's size stands in when it is not among them.
 */
export function sizesAt(size: Viewport, list: Viewport[]): Viewport[] {
  const same = list.filter((v) => v.width === size.width);
  return same.length ? same : [size];
}

/** The page itself never scrolls: each column scrolls on its own. */
export async function expectNoPageScroll(page: Page) {
  const m = await page.evaluate(() => {
    const el = document.scrollingElement ?? document.documentElement;
    return {
      sw: el.scrollWidth,
      cw: el.clientWidth,
      sh: el.scrollHeight,
      ch: el.clientHeight,
    };
  });
  expect(m.sw, "horizontal page scroll").toBeLessThanOrEqual(m.cw);
  expect(m.sh, "vertical page scroll").toBeLessThanOrEqual(m.ch);
  expect(
    await escapedAbsolutes(page),
    "absolute boxes past their scroller",
  ).toEqual([]);
}

/**
 * Absolutely positioned boxes that a scroller or clipping box around them can't hold: their
 * containing block (nearest positioned or transformed ancestor) lies outside that box, so they
 * are neither clipped nor scrolled by it and they stretch the document. A `sr-only` label in a
 * journal turn below the fold did that: with enough turns the whole page got a scrollbar. The
 * rule holds whatever the content, so a short fixture journal still proves it.
 */
export async function escapedAbsolutes(page: Page): Promise<string[]> {
  return page.evaluate(() => {
    const createsBlock = (cs: CSSStyleDeclaration) =>
      cs.position !== "static" ||
      cs.transform !== "none" ||
      cs.filter !== "none" ||
      cs.perspective !== "none" ||
      /paint|layout|strict|content/.test(cs.contain);
    const out: string[] = [];
    for (const el of document.querySelectorAll("body *")) {
      if (getComputedStyle(el).position !== "absolute") continue;
      for (let p = el.parentElement; p; p = p.parentElement) {
        const cs = getComputedStyle(p);
        if (createsBlock(cs)) break;
        if (cs.overflowX !== "visible" || cs.overflowY !== "visible") {
          const what = (e: Element) =>
            `${e.tagName.toLowerCase()}.${(e.getAttribute("class") ?? "").split(" ").slice(0, 4).join(".")}`;
          out.push(
            `${what(el)} "${el.textContent?.slice(0, 30)}" escapes ${what(p)}`,
          );
          break;
        }
      }
    }
    return [...new Set(out)];
  });
}

/** The always-visible zones (ADR 0022 step 2.1) and the focused activity's stage. */
export async function expectZones(page: Page) {
  await expect(page.locator("[data-screen=round-header]")).toBeVisible();
  await expect(
    page.getByRole("complementary", { name: "Character" }),
  ).toBeVisible();
  await expect(
    page.getByRole("navigation", { name: "Activities" }),
  ).toBeVisible();
  await expect(page.getByTestId("stage")).toBeVisible();
  await expect(page.getByTestId("dice-footer")).toBeVisible();
}

/** `inner` lies entirely within `outer` (bounding boxes, 1px tolerance for subpixel rounding). */
export async function expectInside(
  inner: Locator,
  outer: Locator,
  what: string,
) {
  const i = (await inner.boundingBox())!;
  const o = (await outer.boundingBox())!;
  expect(i.y, `${what}: top`).toBeGreaterThanOrEqual(o.y - 1);
  expect(i.y + i.height, `${what}: bottom`).toBeLessThanOrEqual(
    o.y + o.height + 1,
  );
  expect(i.x, `${what}: left`).toBeGreaterThanOrEqual(o.x - 1);
  expect(i.x + i.width, `${what}: right`).toBeLessThanOrEqual(
    o.x + o.width + 1,
  );
}

/** `el` is shown in full inside the viewport. */
export async function expectOnScreen(page: Page, el: Locator, what: string) {
  await expect(el, what).toBeVisible();
  const b = (await el.boundingBox())!;
  const vp = page.viewportSize()!;
  expect(b.y, `${what}: top`).toBeGreaterThanOrEqual(0);
  expect(b.x, `${what}: left`).toBeGreaterThanOrEqual(0);
  expect(b.y + b.height, `${what}: bottom`).toBeLessThanOrEqual(vp.height);
  expect(b.x + b.width, `${what}: right`).toBeLessThanOrEqual(vp.width);
}

/** The column scrolls no content out of view. */
export async function expectNoScroll(column: Locator, what: string) {
  const m = await column.evaluate((el) => ({
    sh: el.scrollHeight,
    ch: el.clientHeight,
  }));
  expect(m.sh, `${what} scrolls`).toBeLessThanOrEqual(m.ch);
}

/**
 * ADR 0027 step 9 on the journal: a one-line entry is a 24 px row, turn blocks touch
 * (no vertical gap), the turn marker is at most 3rem wide, the entry text starts close to the left,
 * every time ends at the same right edge (9.4) and a turn's opening rows are not framed (9.6).
 * `long` holds the texts whose entries may wrap.
 */
export async function expectCompactJournal(
  page: Page,
  long: readonly string[],
  where: string,
) {
  const log = combatLog(page);
  for (const row of await log.getByRole("listitem").all()) {
    const text = (await row.textContent()) ?? "";
    if (long.some((l) => text.includes(l))) continue;
    expect(
      (await row.boundingBox())!.height,
      `one-line row, ${where}: ${text}`,
    ).toBeLessThanOrEqual(24);
  }
  const turns = await log.getByTestId("journal-round").all();
  expect(turns.length, `turn blocks, ${where}`).toBeGreaterThanOrEqual(2);
  for (let i = 1; i < turns.length; i++) {
    const [a, b] = [
      (await turns[i - 1].boundingBox())!,
      (await turns[i].boundingBox())!,
    ];
    expect(
      Math.abs(b.y - (a.y + a.height)),
      `no gap between turn blocks, ${where}`,
    ).toBeLessThanOrEqual(1);
  }
  const marker = (await log
    .getByTestId("journal-round-marker")
    .first()
    .boundingBox())!;
  expect(
    marker.width,
    `turn marker at most 3rem, ${where}`,
  ).toBeLessThanOrEqual(48);
  const logBox = (await log.boundingBox())!;
  const entryText = (await log
    .getByTestId("log-select")
    .first()
    .locator("span.break-words")
    .boundingBox())!;
  expect(
    entryText.x - logBox.x,
    `entry text near the left, ${where}`,
  ).toBeLessThan(150);
  const rightEdges = await log
    .getByTestId("log-time")
    .evaluateAll((els) => els.map((el) => el.getBoundingClientRect().right));
  expect(rightEdges.length, `times, ${where}`).toBeGreaterThanOrEqual(2);
  for (const r of rightEdges)
    expect(
      Math.abs(r - rightEdges[0]),
      `time right-aligned, ${where}`,
    ).toBeLessThanOrEqual(1);
  await expect(
    log.locator('[data-kind="round_advance"]'),
    `a turn's opening rows, ${where}`,
  ).not.toHaveCount(0);
  await expect(
    log.getByTestId("log-transaction").locator('[data-kind="round_advance"]'),
    `a turn's opening rows unframed, ${where}`,
  ).toHaveCount(0);
}

/**
 * ADR 0027 step 5 on the character card, in French at 1440x900 and 1707x1300: the card is
 * `clamp(280px, 21vw, 384px)` wide, the saves heading and every save row are one line, the HP
 * change's amount and buttons share one line and the source's placeholder fits its field.
 */
export async function expectCharacterCardLines(page: Page, where: string) {
  const character = page.getByTestId("character");
  const vw = page.viewportSize()!.width;
  expect(
    Math.round((await character.boundingBox())!.width),
    `card width, ${where}`,
  ).toBe(Math.round(Math.min(384, Math.max(280, vw * 0.21))));
  const heading = character.getByRole("heading", {
    name: "Jets de sauvegarde",
  });
  // One 16 px line plus the sub-band's 4 px padding above and below (ADR 0026 step 6.3).
  expect(
    (await heading.boundingBox())!.height,
    `saves heading on one line, ${where}`,
  ).toBeLessThanOrEqual(25);
  // The label line with its rare controls (mind box, spell button) is one line too (ADR 0027 step 1).
  expect(
    (await character.getByTestId("saves-head").boundingBox())!.height,
    `saves label line on one line, ${where}`,
  ).toBeLessThanOrEqual(25);
  for (const row of await character.getByTestId("save-row").all())
    expect(
      (await row.boundingBox())!.height,
      `${await row.getAttribute("data-save")} on one line, ${where}`,
    ).toBeLessThanOrEqual(29);
  // The identity: the class under the name, the level right-aligned on the name's line.
  const name = (await character.getByTestId("details-trigger").boundingBox())!;
  const klass = (await character.getByTestId("class").boundingBox())!;
  const level = (await character
    .getByRole("button", { name: /^Niveau/ })
    .boundingBox())!;
  expect(klass.y, `class under the name, ${where}`).toBeGreaterThanOrEqual(
    name.y + name.height - 1,
  );
  expect(
    Math.abs(level.y + level.height / 2 - (name.y + 14)),
    `level on the name's line, ${where}`,
  ).toBeLessThanOrEqual(14);
  const hp = character.getByRole("group", { name: "Modifier les PV" });
  await expect(
    hp.getByText("Montant"),
    `no visible amount label, ${where}`,
  ).toHaveCount(0);
  const amount = (await hp.getByLabel("Montant").boundingBox())!;
  for (const name of ["Dégâts", "Soins"]) {
    const b = (await hp.getByRole("button", { name }).boundingBox())!;
    expect(
      Math.abs(b.y - amount.y),
      `${name} beside the amount, ${where}`,
    ).toBeLessThanOrEqual(1);
  }
  const placeholder = await hp.getByLabel("Source").evaluate((el) => {
    const input = el as HTMLInputElement;
    const style = getComputedStyle(input);
    const ctx = document.createElement("canvas").getContext("2d")!;
    ctx.font = style.font;
    const room =
      input.clientWidth -
      parseFloat(style.paddingLeft) -
      parseFloat(style.paddingRight);
    return { text: ctx.measureText(input.placeholder).width, room };
  });
  expect(
    placeholder.text,
    `source placeholder fits, ${where}`,
  ).toBeLessThanOrEqual(placeholder.room);
}

/**
 * ADR 0027 step 4 on the header, in French: the turn label over its number, the initiative under
 * "Tour suivant" and larger than the turn, five turn columns, the brand (mark and wordmark) over the
 * last result in one right-aligned column 6 px apart whose right edge lines up with the result card,
 * at most 120 px at rest. At the design's two viewports every effect name and the last result are
 * one line.
 */
export async function expectHeaderLayout(page: Page, where: string) {
  const header = page.locator("[data-screen=round-header]");
  const box = async (l: Locator) => (await l.boundingBox())!;
  const h = await box(header);
  expect(h.height, `header at rest, ${where}`).toBeLessThanOrEqual(120);
  const label = await box(
    header.getByTestId("round").getByText("Tour", { exact: true }),
  );
  const turn = await box(page.getByTestId("round-value"));
  expect(
    label.y + label.height,
    `turn label over its number, ${where}`,
  ).toBeLessThanOrEqual(turn.y + 1);
  const next = await box(page.getByRole("button", { name: "Tour suivant" }));
  // Next turn is an `lg` primary action, 32 px (ADR 0017 step 5, ADR 0027 step 1.2).
  expect(Math.round(next.height), `Tour suivant height, ${where}`).toBe(32);
  const init = page.getByTestId("initiative-value");
  expect(
    (await box(init)).y,
    `initiative under Tour suivant, ${where}`,
  ).toBeGreaterThan(next.y + next.height - 1);
  const fontSize = (l: Locator) =>
    l.evaluate((el) => parseFloat(getComputedStyle(el).fontSize));
  expect(
    await fontSize(init),
    `initiative larger than the turn, ${where}`,
  ).toBeGreaterThan(await fontSize(page.getByTestId("round-value")));
  await expect(page.getByTestId("effects-track")).toHaveAttribute(
    "data-columns",
    "5",
  );
  await expect(page.getByTestId("wordmark")).toBeVisible();
  const brand = await box(page.getByTestId("brand"));
  const last = await box(page.getByTestId("last-result"));
  const right = (b: { x: number; width: number }) => b.x + b.width;
  expect(
    Math.abs(right(brand) - right(last)),
    `brand and last result right-aligned, ${where}`,
  ).toBeLessThanOrEqual(1);
  expect(
    Math.abs(last.y - (brand.y + brand.height) - 6),
    `6 px under the brand, ${where}`,
  ).toBeLessThanOrEqual(1);
  // The column sits at the header's right edge (its padding and outline in), as the result card.
  const card = await box(page.getByTestId("now-band"));
  expect(
    Math.abs(right(h) - right(card)),
    `header and result card share their right edge, ${where}`,
  ).toBeLessThanOrEqual(1);
  expect(
    right(h) - right(last),
    `right column docked, ${where}`,
  ).toBeLessThanOrEqual(18);
  if (SHOTS.has(where.replace(/^.*?(\d+x\d+)$/, "$1"))) {
    for (const name of await page.getByTestId("effect-name").all())
      expect(
        await name.evaluate(lineCount),
        `effect name on one line, ${where}: ${await name.textContent()}`,
      ).toBe(1);
    expect(
      await page.getByTestId("last-result").locator("p").evaluate(lineCount),
      `last result on one line, ${where}`,
    ).toBe(1);
  }
}

/**
 * ADR 0035 item 6.3: the initiative's breakdown is its number's tooltip, shown on hover, whole
 * and inside the screen; the "fin du tour" tag sits in the header (which stays at rest,
 * expectHeaderLayout) when the round's spell takes a round or more.
 */
export async function expectInitiativeBreakdown(page: Page, where: string) {
  await expect(page.getByTestId("initiative-end-of-round")).toHaveText(
    "fin du tour",
  );
  const value = page.getByTestId("initiative-value");
  await value.hover();
  const breakdown = page.getByTestId("initiative-breakdown");
  await expect(breakdown).toBeVisible();
  await expect(breakdown).toContainText(/^d10 \d+/);
  await expect(breakdown).toContainText("fin du tour");
  await expect(breakdown).toContainText("le plus bas agit en premier");
  await expectNoClippedText(breakdown, `initiative breakdown, ${where}`);
  await expectInside(breakdown, page.locator("body"), `breakdown, ${where}`);
  if (
    SHOTS.has(where.replace(/^.*?(\d+x\d+)$/, "$1")) ||
    /1920|2560/.test(where)
  )
    await snap(page, `initiative-breakdown-${where.replace(/[ ,]+/g, "-")}`);
  await page.mouse.move(0, 0);
  await expect(breakdown).toBeHidden();
}

/** The lines `el`'s text lays out on: its text boxes grouped by vertical overlap. */
export function lineCount(el: Element): number {
  const range = document.createRange();
  range.selectNodeContents(el);
  const rects = [...range.getClientRects()]
    .filter((r) => r.width > 0)
    .sort((a, b) => a.top - b.top);
  let lines = 0;
  let bottom = -Infinity;
  for (const r of rects) {
    if (r.top >= bottom - 1) lines++;
    bottom = r.top >= bottom - 1 ? r.bottom : Math.max(bottom, r.bottom);
  }
  return lines;
}

// ADR 0022 step 1 (layout viewports). 1707x1300 is a third of the user's 5120x1440 table screen.
export const VIEWPORTS = [
  { width: 1280, height: 800 },
  { width: 1440, height: 900 },
  { width: 1707, height: 1300 },
  { width: 1920, height: 1080 },
  { width: 2560, height: 1440 },
];
export const ACTIVITIES = ["Act", "Spells", "Inventory", "Experience"] as const;
/** Activities whose screenshots are kept, at the two viewports of the design (ADR 0022). */
export const SHOTS = new Set(["1440x900", "1707x1300"]);
/** A long effect name: it wraps in the expanded header, never cut. */
export const LONG_EFFECT =
  "Protection From Evil, 10' Radius (from the temple priest)";
/** A long journal entry: it wraps inside its row, never cut (ADR 0022 step 5.1). */
export const LONG_SOURCE =
  "Fire breath of the young red dragon perched on the ruined bell tower, half damage behind the shield wall";

/** A long XP ledger reason: it wraps between words in its row, never cut (ADR 0022 step 1). */
export const LONG_REASON =
  "Drove the hobgoblin warband out of the flooded silver mine below Thornwood and freed the captured dwarven miners";

/** A long spell name and stat text: the lookup's list and detail wrap them, never cut (step 1). */
export const LONG_SPELL =
  "Comprehensively Unreasonably Long Spell Name Of The Ancient Archmages";
