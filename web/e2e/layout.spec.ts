import { test, expect, snap, catalogueSpell } from "./fixtures";
import {
  addBuff,
  ammoChips,
  attackForm,
  chooseLanguage,
  chooseSetting,
  chooseTheme,
  chooseWeapon,
  combatLog,
  nextTurn,
  openDashboard,
  openSettings,
  openWeapons,
  openWorkspace,
  recordHp,
  openFold,
} from "./helpers";
import { test as base, type Locator, type Page } from "@playwright/test";
import {
  brokenWords,
  clippedText,
  expectNoClippedText,
  expectWholeWords,
} from "./noClip";

/** The page itself never scrolls: each column scrolls on its own. */
async function expectNoPageScroll(page: Page) {
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
}

/** The always-visible zones (ADR 0022 step 2.1) and the focused activity's stage. */
async function expectZones(page: Page) {
  await expect(page.locator("[data-screen=turn-header]")).toBeVisible();
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
async function expectInside(inner: Locator, outer: Locator, what: string) {
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
async function expectOnScreen(page: Page, el: Locator, what: string) {
  await expect(el, what).toBeVisible();
  const b = (await el.boundingBox())!;
  const vp = page.viewportSize()!;
  expect(b.y, `${what}: top`).toBeGreaterThanOrEqual(0);
  expect(b.x, `${what}: left`).toBeGreaterThanOrEqual(0);
  expect(b.y + b.height, `${what}: bottom`).toBeLessThanOrEqual(vp.height);
  expect(b.x + b.width, `${what}: right`).toBeLessThanOrEqual(vp.width);
}

/** The column scrolls no content out of view. */
async function expectNoScroll(column: Locator, what: string) {
  const m = await column.evaluate((el) => ({
    sh: el.scrollHeight,
    ch: el.clientHeight,
  }));
  expect(m.sh, `${what} scrolls`).toBeLessThanOrEqual(m.ch);
}

/**
 * ADR 0027 step 9 on the journal: a one-line entry is a 22 px row (at most 24), turn blocks touch
 * (no vertical gap), the turn marker is at most 3rem wide, the entry text starts close to the left,
 * every time ends at the same right edge (9.4) and a turn's opening rows are not framed (9.6).
 * `long` holds the texts whose entries may wrap.
 */
async function expectCompactJournal(
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
  const turns = await log.getByTestId("journal-turn").all();
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
    .getByTestId("journal-turn-marker")
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
async function expectCharacterCardLines(page: Page, where: string) {
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
async function expectHeaderLayout(page: Page, where: string) {
  const header = page.locator("[data-screen=turn-header]");
  const box = async (l: Locator) => (await l.boundingBox())!;
  const h = await box(header);
  expect(h.height, `header at rest, ${where}`).toBeLessThanOrEqual(120);
  const label = await box(
    header.getByTestId("turn").getByText("Tour", { exact: true }),
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

/** The lines `el`'s text lays out on: its text boxes grouped by vertical overlap. */
function lineCount(el: Element): number {
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
const VIEWPORTS = [
  { width: 1280, height: 800 },
  { width: 1440, height: 900 },
  { width: 1707, height: 1300 },
  { width: 1920, height: 1080 },
  { width: 2560, height: 1440 },
];
const ACTIVITIES = ["Act", "Spells", "Inventory", "Experience"] as const;
/** Activities whose screenshots are kept, at the two viewports of the design (ADR 0022). */
const SHOTS = new Set(["1440x900", "1707x1300"]);
/** A long effect name: it wraps in the expanded header, never cut. */
const LONG_EFFECT = "Protection From Evil, 10' Radius (from the temple priest)";
/** A long journal entry: it wraps inside its row, never cut (ADR 0022 step 5.1). */
const LONG_SOURCE =
  "Fire breath of the young red dragon perched on the ruined bell tower, half damage behind the shield wall";

/** A long XP ledger reason: it wraps between words in its row, never cut (ADR 0022 step 1). */
const LONG_REASON =
  "Drove the hobgoblin warband out of the flooded silver mine below Thornwood and freed the captured dwarven miners";

/** A long spell name and stat text: the lookup's list and detail wrap them, never cut (step 1). */
const LONG_SPELL =
  "Comprehensively Unreasonably Long Spell Name Of The Ancient Archmages";
test.use({
  extraSpells: {
    rows: [
      catalogueSpell({
        id: "long_spell",
        name: LONG_SPELL,
        names: {
          fr: "Nom de sortilège invraisemblablement long des anciens archimages",
        },
        class: "Wizard",
        level: 1,
        school: "Divination",
        range: "Special, see the description of the ritual circle",
        area_of_effect:
          "10-ft. cube per level, up to one creature/level of the caster",
      }),
    ],
  },
});

// ADR 0022 steps 2.1, 2.2, 2.8 and 3.1: in every activity at every layout viewport, the header card
// (at most 120 px at rest with 3 effect rows), the character card and the dice footer stay put and
// show the last result, HP and the turn with its effects; the page never scrolls and no text in the
// shell's zones is cut (step 1). Screenshots of every activity at 1440x900 and 1707x1300.
test("shell: always-visible zones in every activity, header budget, no cut text", async ({
  page,
}) => {
  test.setTimeout(90_000);
  await page.setViewportSize({ width: 1024, height: 768 });
  await openDashboard(page);
  await expectZones(page);
  await expectNoPageScroll(page);

  // The header is as tall as its effects need, up to the turn row + 3 effect rows; the rest sits
  // behind the expander.
  const header = page.locator("[data-screen=turn-header]");
  const heightOf = async () => (await header.boundingBox())!.height;
  const empty = await heightOf();
  await addBuff(page, { name: "Aid", rounds: 6 });
  expect(await heightOf(), "one effect fits the turn cells' height").toBe(
    empty,
  );
  for (const name of ["Bless", "Shield"])
    await addBuff(page, { name, rounds: 6 });
  const three = await heightOf();
  await expect(page.getByTestId("effects-expander")).toHaveCount(0);
  // Past three rows the header keeps its height; the long name shows (wrapped) once expanded.
  for (const name of ["Haste", LONG_EFFECT])
    await addBuff(page, { name, rounds: 6 });
  const expander = page.getByTestId("effects-expander");
  await expander.click(); // addBuff left it open
  await expect(expander).toHaveAttribute("aria-expanded", "false");
  await expect(page.getByTestId("buff")).toHaveCount(3);
  expect(await heightOf(), "five effects at rest").toBe(three);

  // A last result to show: an attack.
  await openWeapons(page);
  await chooseWeapon(page, "Long Sword +2");
  await attackForm(page).getByLabel("Target AC").fill("5");
  await attackForm(page)
    .getByRole("button", { name: "Attack", exact: true })
    .click();
  const last = page.getByTestId("last-result");
  await expect(last.getByTestId("last-result-text")).toContainText("Attack");
  // The journal: a long entry, and a second turn so a past turn sits under the current one.
  await nextTurn(page);
  await recordHp(page, "damage", 3, LONG_SOURCE);
  await expect(
    combatLog(page).getByRole("listitem").filter({ hasText: LONG_SOURCE }),
  ).toBeVisible();

  // The lookup with the long spell selected in a full list.
  await openWorkspace(page, "Spells");
  const lookupList = page.getByRole("listbox", { name: "Spells" });
  const spellDetail = page.getByTestId("spell-detail");
  await lookupList.getByRole("option", { name: LONG_SPELL }).click();
  await expect(spellDetail.getByRole("heading").first()).toHaveText(LONG_SPELL);

  // The XP ledger with a long reason and a session.
  await openWorkspace(page, "Experience");
  const xp = page.getByRole("region", { name: "Experience", exact: true });
  const addXp = xp.getByRole("region", { name: "Add XP" });
  await addXp.getByLabel("Amount").fill("2500");
  await addXp.getByLabel("Reason").fill(LONG_REASON);
  await addXp.getByLabel("Session").fill("S41");
  await addXp.getByLabel("Reason").press("Enter");
  const xpLedger = xp.getByRole("region", { name: "Ledger" });
  const longXpRow = xpLedger.getByRole("listitem", { name: LONG_REASON });
  await expect(longXpRow).toBeVisible();

  const rail = page.getByRole("navigation", { name: "Activities" });
  const character = page.getByRole("complementary", { name: "Character" });
  for (const vp of VIEWPORTS) {
    await page.setViewportSize(vp);
    const size = `${vp.width}x${vp.height}`;
    expect(await heightOf(), `header at rest, ${size}`).toBeLessThanOrEqual(
      120,
    );
    // The full brand at every width (ADR 0027 step 4.6).
    await expect(page.getByTestId("wordmark")).toBeVisible();
    for (const activity of ACTIVITIES) {
      await openWorkspace(page, activity);
      await expect(
        rail.getByRole("button", { name: activity }),
      ).toHaveAttribute("aria-current", "page");
      await expectZones(page);
      await expectNoPageScroll(page);
      const where = `${activity}, ${size}`;
      await expectOnScreen(page, last, `last result, ${where}`);
      await expectOnScreen(page, page.getByTestId("hp-value"), `HP, ${where}`);
      await expectOnScreen(
        page,
        page.getByTestId("round-value"),
        `turn, ${where}`,
      );
      await expectOnScreen(
        page,
        page.getByRole("group", { name: "Effects" }),
        `effects, ${where}`,
      );
      await expectInside(page.getByTestId("hp-value"), character, where);
      await expectNoClippedText(header, `header, ${where}`);
      await expectNoClippedText(rail, `rail, ${where}`);
      // The character card (ADR 0022 step 2.3), with the long source as the newest HP change.
      await expectNoClippedText(character, `character card, ${where}`);
      await expectWholeWords(character, `character card, ${where}`);
      await expectNoClippedText(
        page.getByTestId("dice-footer"),
        `dice footer, ${where}`,
      );
      if (activity === "Act")
        await expectNoClippedText(combatLog(page), `journal, ${where}`);
      if (activity === "Spells") {
        // The spell lookup (ADR 0022 step 2.5): the list and the detail, the long name selected.
        await expectNoClippedText(lookupList, `spell list, ${where}`);
        await expectNoClippedText(spellDetail, `spell detail, ${where}`);
      }
      if (activity === "Experience") {
        // ADR 0022 step 2.5: progress beside the Add XP form in one card, the ledger card below
        // with the rest of the stage's height; the long reason wraps between words (step 1).
        await expectNoClippedText(xp, `experience, ${where}`);
        await expectWholeWords(xpLedger, `XP ledger, ${where}`);
        const top = (await xp.getByTestId("xp-top").boundingBox())!;
        const progress = (await xp
          .getByRole("region", { name: "Progress" })
          .boundingBox())!;
        const form = (await addXp.boundingBox())!;
        expect(Math.round(form.width), `Add XP is 24rem, ${where}`).toBe(384);
        expect(form.x, `form beside progress, ${where}`).toBeGreaterThan(
          progress.x + progress.width,
        );
        const ledgerBox = (await xpLedger.boundingBox())!;
        expect(ledgerBox.y, `ledger below, ${where}`).toBeGreaterThan(
          top.y + top.height,
        );
        // As tall as its rows, bounded by the stage (ADR 0027 step 14.4).
        await expectInside(
          xpLedger,
          page.getByTestId("stage"),
          `ledger, ${where}`,
        );
        expect(
          (await longXpRow.boundingBox())!.height,
          `long reason wraps, ${where}`,
        ).toBeGreaterThan(40);
      }
      if (SHOTS.has(size))
        await snap(page, `${activity.toLowerCase()}-${size}`);
    }
    await expectNoScroll(
      page.getByTestId("effects-track").locator(".."),
      "track",
    );
    if (vp.width === 1440) {
      // ADR 0022 step 2.8: the character card shows everything from the name to the fifth saving
      // throw without scrolling, and the weapon in hand is fully inside the act panel.
      await expectNoScroll(character, "character card");
      await expect(character.getByTestId("save-row")).toHaveCount(5);
      for (const [what, el] of [
        ["name", character.getByRole("heading", { level: 1 })],
        ["HP change", character.getByRole("group", { name: "HP change" })],
        ["fifth save", character.getByTestId("save-row").last()],
      ] as const)
        await expectInside(el, character, what);
      await openWorkspace(page, "Act");
      // ADR 0022 step 5.1, ADR 0027 step 9: an entry is one 22 px line; only a long one wraps.
      const row = (text: string) =>
        combatLog(page).getByRole("listitem").filter({ hasText: text });
      await expectCompactJournal(page, [LONG_SOURCE], size);
      await expectNoClippedText(row(LONG_SOURCE), "long journal entry");
      await expectInside(
        page.getByRole("region", { name: "Weapon in hand" }),
        page.getByRole("region", { name: "Actions" }),
        "weapon in hand",
      );
    }
    if (vp.width === 1280 || vp.width === 1707) {
      await openWorkspace(page, "Act");
      await expander.click();
      await expect(expander).toHaveAttribute("aria-expanded", "true");
      await expect(page.getByTestId("buff")).toHaveCount(5);
      expect(await heightOf(), "expanded header").toBeGreaterThan(three);
      await expectNoPageScroll(page);
      await expectNoClippedText(header, `expanded header, ${size}`);
      await snap(page, `act-${size}-effects-expanded`);
      await expander.click();
      await expect(page.getByTestId("buff")).toHaveCount(3);
    }
  }
});

// ADR 0016 step 13 and ADR 0022 step 1: an item power that is on (the Power Gem's hammer at STR 18)
// is a row of the effects header; its name, its effects and its bar caption are shown whole, never
// cut, at the smallest and the largest layout viewport.
test.describe("with the sheet's STR 12", () => {
  test.use({ character: { str: 12 } });

  test("item power row: name, effects and bar caption uncut, 1280 to 2560", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await openDashboard(page);
    await addBuff(page, {
      name: "e2e Strength",
      rounds: 6,
      effects: [
        ["str_bonus", 6],
        ["str_max", 18],
      ],
    });
    const header = page.locator("[data-screen=turn-header]");
    const hammer = page.locator(
      '[data-testid="item-power"][data-item="power_gem_dispel"]',
    );
    for (const vp of [
      { width: 1280, height: 800 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      const size = `${vp.width}x${vp.height}`;
      await expect(hammer).toContainText("Power Gem");
      await expectOnScreen(
        page,
        hammer.getByTestId("power-effects"),
        `power effects, ${size}`,
      );
      const caption = hammer
        .getByTestId("effect-bar")
        .getByText("while STR ≥ 18");
      await expectOnScreen(page, caption, `power bar caption, ${size}`);
      await expectNoClippedText(header, `header with the power, ${size}`);
      await expectWholeWords(header, `header with the power, ${size}`);
      await snap(page, `power-row-${size}`);
    }
  });
});

// ADR 0022 step 5.4: the handle between the now band and the journal resizes the journal by drag
// (mouse) and arrows (keyboard); a click (or Enter) on it folds the journal into a one-line bar and
// back, a drag of 4 px or more only resizes; both are kept per device
// across a reload. What only a browser proves: pointer drag, layout heights and localStorage.
test("journal split: drag resizes, click toggles, kept across reload", async ({
  page,
}) => {
  await openDashboard(page);
  await recordHp(page, "damage", 3, "Split goblin");
  const slot = page.getByTestId("journal-slot");
  const band = page.getByTestId("now-band");
  const handle = page.getByTestId("journal-handle");
  const heightOf = async (l: Locator) => (await l.boundingBox())!.height;
  // Default: the band takes at most 60 % of the stage, the journal at least 14rem.
  const stage = await heightOf(page.getByTestId("act-stage"));
  expect(await heightOf(band)).toBeLessThanOrEqual(stage * 0.6 + 1);
  const before = await heightOf(slot);
  expect(before).toBeGreaterThanOrEqual(224);

  // Drag the handle up by 120 px: the journal grows by as much, the band shrinks.
  const h = (await handle.boundingBox())!;
  await page.mouse.move(h.x + h.width / 2, h.y + h.height / 2);
  await page.mouse.down();
  await page.mouse.move(h.x + h.width / 2, h.y + h.height / 2 - 60, {
    steps: 4,
  });
  await page.mouse.move(h.x + h.width / 2, h.y + h.height / 2 - 120, {
    steps: 4,
  });
  await page.mouse.up();
  const dragged = await heightOf(slot);
  expect(Math.abs(dragged - (before + 120))).toBeLessThanOrEqual(2);
  // Keyboard: ArrowDown shrinks it by one step.
  await handle.focus();
  await page.keyboard.press("ArrowDown");
  await expect.poll(() => heightOf(slot)).toBeCloseTo(dragged - 16, 0);
  await snap(page, "act-journal-dragged");

  // A drag never toggles: still expanded, handle states it.
  await expect(handle).toHaveAttribute("aria-valuetext", "Expanded");
  await expect(
    page.getByRole("button", { name: /Collapse|Expand/ }),
  ).toHaveCount(0);

  // A click on the handle collapses: one line with the newest entry, the count, Undo / Redo.
  await handle.click();
  await expect(handle).toHaveAttribute("aria-valuetext", "Collapsed");
  const bar = page.getByTestId("journal-bar");
  await expect(bar).toContainText("Split goblin");
  await expect(bar.getByTestId("undo")).toBeVisible();
  await expectNoClippedText(bar, "collapsed journal");
  expect(await heightOf(slot)).toBeLessThan(80);
  await snap(page, "act-journal-collapsed");

  // Kept across a reload; A click brings back the dragged height.
  await page.reload();
  await expect(page.getByTestId("journal-bar")).toContainText("Split goblin");
  await page.getByTestId("journal-handle").click();
  await expect(combatLog(page).getByRole("listitem").first()).toBeVisible();
  await expect.poll(() => heightOf(slot)).toBeCloseTo(dragged - 16, 0);
});

// The audit itself: it reports a cut line and a box hiding tall text, and passes text that wraps or
// scrolls. A plain page, no stack.
base(
  "expectNoClippedText reports cut text and passes wrapped or scrolling text",
  async ({ page }) => {
    await page.setContent(
      `<div id="cut" style="width:50px;overflow:hidden;white-space:nowrap">a very long line of text</div>
<div id="tall" style="height:20px;overflow:hidden"><p style="margin:0">one</p><p style="margin:0">two</p></div>
<div id="ok" style="width:50px">a very long line of text that wraps</div>
<div id="scrolls" style="height:20px;overflow-y:auto"><p>one</p><p>two</p></div>
<span id="sr" style="position:absolute;width:1px;height:1px;overflow:hidden;white-space:nowrap">screen reader text</span>`,
    );
    const cut = await page.locator("body").evaluate(clippedText);
    expect(cut).toHaveLength(2);
    expect(cut[0]).toContain("wide");
    expect(cut[1]).toContain("tall");
    for (const id of ["ok", "scrolls", "sr"])
      await expectNoClippedText(page.locator(`#${id}`), id);
  },
);

// A word split across two lines counts as cut too; words wrapped whole pass.
base("expectWholeWords reports a word broken mid-word", async ({ page }) => {
  await page.setContent(
    `<div id="split" style="width:40px;overflow-wrap:anywhere">Thornwoodsilvermine</div>
<div id="whole" style="width:60px">a few short words wrap</div>`,
  );
  expect(await page.locator("#split").evaluate(brokenWords)).toEqual([
    "Thornwoodsilvermine",
  ]);
  await expectWholeWords(page.locator("#whole"), "whole");
});

// ADR 0022 step 2.4: a click focuses an activity; each mounts on its first visit and keeps its state
// while hidden.
test("rail: focuses activities and each keeps its state", async ({ page }) => {
  await openDashboard(page);
  const rail = page.getByRole("navigation", { name: "Activities" });
  await expect(rail.getByRole("button", { name: "Act" })).toHaveAttribute(
    "aria-current",
    "page",
  );

  // Act: pick the Spells tab.
  await page.getByRole("tab", { name: "Spells" }).click();
  await expect(page.getByRole("tab", { name: "Spells" })).toHaveAttribute(
    "aria-selected",
    "true",
  );

  // Inventory: items and weapons; open the add-item form.
  await openWorkspace(page, "Inventory");
  await expect(rail.getByRole("button", { name: "Inventory" })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await expect(page.getByRole("region", { name: "Actions" })).toBeHidden();
  const inventory = page.getByRole("region", { name: "Inventory" });
  await expect(
    inventory.getByRole("button", { name: "Add weapon" }),
  ).toBeVisible();
  await inventory.getByRole("button", { name: "Add item" }).click();
  await expect(inventory.getByTestId("item-form")).toBeVisible();

  // Spells: the spell lookup; type a search.
  await openWorkspace(page, "Spells");
  const search = page.getByRole("combobox", { name: "Search spells..." });
  await search.fill("fire");
  await expect(page.getByTestId("spell-row").first()).toBeVisible();

  // Back: every activity is as it was left; the journal is the Act activity's.
  await openWorkspace(page, "Act");
  await expect(page.getByRole("tab", { name: "Spells" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await openWorkspace(page, "Inventory");
  await expect(inventory.getByTestId("item-form")).toBeVisible();
  await openWorkspace(page, "Spells");
  await expect(search).toHaveValue("fire");
  await expect(
    page.getByRole("region", { name: "Journal", exact: true }),
  ).toBeHidden();
  await expect(page.getByTestId("hp-value")).toBeVisible();
});

const isDark = (page: Page) =>
  page.evaluate(() => document.documentElement.classList.contains("dark"));
const rootVar = (page: Page, name: string) =>
  page.evaluate(
    (n) =>
      getComputedStyle(document.documentElement).getPropertyValue(n).trim(),
    name,
  );

// ADR 0026 step 4: a dark and a light theme per device, the mode picks which applies, "system"
// switches live with the OS; both choices survive a reload and the inline script paints the
// effective one before the app runs.
test("settings: language, appearance and both themes apply live and persist across reload", async ({
  page,
}) => {
  const DEEP = / 232\)$/; // Deep's tint hue
  const PARCHMENT = / 262\)$/; // Parchment's lapis
  await page.emulateMedia({ colorScheme: "light" });
  await openDashboard(page);
  expect(await isDark(page)).toBe(false); // System by default, the OS is light

  await chooseTheme(page, "Parchment");
  await expect.poll(() => rootVar(page, "--tint")).toMatch(PARCHMENT);
  // The list follows the appearance: light themes in Light, dark themes in Dark.
  let popover = await openSettings(page);
  const list = popover.getByTestId("settings-theme");
  await expect(list).toHaveAttribute("data-mode", "light");
  await expect(list.getByRole("radio")).toHaveText([
    /Arcane/,
    /Parchment/,
    /Plain/,
    /Forest/,
  ]);
  await page.keyboard.press("Escape");
  await chooseSetting(page, "Dark");
  expect(await isDark(page)).toBe(true);
  await openSettings(page);
  await expect(list).toHaveAttribute("data-mode", "dark");
  await expect(list.getByRole("radio")).toHaveText([
    /Arcane/,
    /Deep/,
    /Plain/,
    /Forest/,
    /Ember/,
    /Amber/,
  ]);
  await page.keyboard.press("Escape");
  await chooseTheme(page, "Deep");
  await expect.poll(() => rootVar(page, "--tint")).toMatch(DEEP);
  await chooseLanguage(page, "fr");
  await expect(
    page.getByRole("button", { name: "Tour suivant" }),
  ).toBeVisible();

  // First paint, the app's script blocked: index.html alone paints Deep in dark.
  const appScript = /\/assets\/.*\.js$/;
  await page.route(appScript, (route) => route.abort());
  await page.reload();
  expect(await isDark(page)).toBe(true);
  expect(await rootVar(page, "--tint")).toMatch(DEEP);
  await page.unroute(appScript);

  await page.reload();
  await expect(
    page.getByRole("button", { name: "Tour suivant" }),
  ).toBeVisible();
  expect(await rootVar(page, "--tint")).toMatch(DEEP);
  popover = await openSettings(page);
  for (const name of ["Français", "Sombre"])
    await expect(
      popover.getByRole("radio", { name, exact: true }),
    ).toHaveAttribute("aria-checked", "true");
  await expect(
    popover.getByTestId("settings-theme").getByRole("radio", {
      name: "Abysses",
      exact: true,
    }),
  ).toHaveAttribute("aria-checked", "true");
  await expectNoClippedText(popover, "settings popover, fr");
  await expectWholeWords(popover, "settings popover, fr");
  await snap(page, "settings-popover-fr");

  // Système follows the OS live, between the two chosen themes.
  await popover.getByRole("radio", { name: "Système", exact: true }).click();
  expect(await isDark(page)).toBe(false);
  await expect.poll(() => rootVar(page, "--tint")).toMatch(PARCHMENT);
  await page.emulateMedia({ colorScheme: "dark" });
  await expect.poll(() => isDark(page)).toBe(true);
  await expect.poll(() => rootVar(page, "--tint")).toMatch(DEEP);

  // Dark -> Light -> Dark: each appearance keeps its own pick, and the list switches with it.
  await popover.getByRole("radio", { name: "Clair", exact: true }).click();
  await expect(popover.getByTestId("settings-theme")).toHaveAttribute(
    "data-mode",
    "light",
  );
  await expect(
    popover
      .getByTestId("settings-theme")
      .getByRole("radio", { name: "Parchemin", exact: true }),
  ).toHaveAttribute("aria-checked", "true");
  await expect.poll(() => rootVar(page, "--tint")).toMatch(PARCHMENT);
  await popover.getByRole("radio", { name: "Sombre", exact: true }).click();
  await expect(
    popover
      .getByTestId("settings-theme")
      .getByRole("radio", { name: "Abysses", exact: true }),
  ).toHaveAttribute("aria-checked", "true");
  await expect.poll(() => rootVar(page, "--tint")).toMatch(DEEP);
});

// The visual identity (ADR 0020, ADR 0026): the main screens in dark and light with content in the
// result card and the journal (Act/Weapons after an attack, Spells, Inventory, Experience), then
// every theme in each mode it exists in, in French (the layout reference), with the no-cut audit.
// Screenshots: the colours themselves are unit-tested (theme.test).
test("theme: the main screens in dark and light, every theme in each of its modes", async ({
  page,
}) => {
  test.setTimeout(90_000);
  await openDashboard(page);
  await expect(page.getByTestId("brand")).toHaveText("Critical!");
  await expect(page).toHaveTitle("Critical!");
  await openWeapons(page);
  await chooseWeapon(page, "Long Sword +2");
  const form = attackForm(page);
  await form.getByLabel("Target AC").fill("5");
  await form.getByRole("button", { name: "Attack", exact: true }).click();
  await expect(combatLog(page).getByRole("listitem").first()).toBeVisible();
  await recordHp(page, "damage", 7, "Orc axe");
  await addBuff(page, { name: "Bless", rounds: 6 });
  await page.keyboard.press("Escape");
  for (const mode of ["Dark", "Light"]) {
    const m = mode.toLowerCase();
    await chooseSetting(page, mode);
    await page.keyboard.press("Escape");
    await openWorkspace(page, "Act");
    await snap(page, `${m}-act-weapons`);
    await openWorkspace(page, "Spells");
    await page.getByRole("combobox", { name: "Search spells..." }).fill("fire");
    await expect(
      page.getByTestId("spell-detail").getByRole("heading"),
    ).toBeVisible();
    await snap(page, `${m}-spells`);
    // A long description fills the detail to its bottom edge at full width and scrolls inside.
    await page
      .getByRole("combobox", { name: "Search spells..." })
      .fill("magic jar");
    const detail = page.getByTestId("spell-detail");
    await expect(detail.getByRole("heading")).toContainText("Magic Jar");
    const text = detail.getByTestId("spell-description-text");
    const [box, panel] = [await text.boundingBox(), await detail.boundingBox()];
    expect(panel!.y + panel!.height - (box!.y + box!.height)).toBeLessThan(16);
    // The card's 16 px padding each side and its outline (ADR 0022 step 3.2).
    expect(panel!.width - box!.width).toBeLessThanOrEqual(36);
    // The text scrolls, not the whole detail.
    expect(
      await detail.evaluate((el) => el.scrollHeight - el.clientHeight),
    ).toBeLessThanOrEqual(1);
    // In two text columns at 1440 (ADR 0022 step 2.5): a paragraph starts in the right half.
    expect(
      await text.evaluate((el) => {
        const mid = el.getBoundingClientRect().left + el.clientWidth / 2;
        return [...el.querySelectorAll("p")].some(
          (p) => p.getBoundingClientRect().left > mid,
        );
      }),
    ).toBe(true);
    await snap(page, `${m}-spells-long`);
    await text.evaluate((el) => el.scrollTo({ top: el.scrollHeight / 3 }));
    await snap(page, `${m}-spells-scrolled`);
    await openWorkspace(page, "Inventory");
    await snap(page, `${m}-inventory`);
    await openWorkspace(page, "Experience");
    await expect(page.getByTestId("xp-total")).toHaveText(/\d/);
    await snap(page, `${m}-experience`);
  }
  await openWorkspace(page, "Act");
  await chooseLanguage(page, "fr");
  for (const [mode, which, themes] of [
    [
      "Sombre",
      "dark",
      ["Arcane", "Abysses", "Sobre", "Forêt", "Braise", "Ambre"],
    ],
    ["Clair", "light", ["Arcane", "Parchemin", "Sobre", "Forêt"]],
  ] as const) {
    await chooseSetting(page, mode);
    for (const name of themes) {
      await chooseTheme(page, name);
      await page.waitForTimeout(150); // let colour transitions settle
      await expectNoClippedText(page.locator("body"), `${which} ${name}, fr`);
      // Parchment's own touches (ADR 0026 step 7): Cinzel section labels, a gilt latest outline;
      // every other theme keeps the body face and its tint outline.
      const touches = await page.evaluate(() => {
        const colour = (v: string) => {
          const probe = document.createElement("div");
          probe.style.outlineColor = `var(${v})`;
          probe.style.outlineStyle = "solid";
          document.body.append(probe);
          const c = getComputedStyle(probe).outlineColor;
          probe.remove();
          return c;
        };
        const label = document.querySelector(".section-label:not(dt)");
        const latest = document.querySelector(".latest");
        return {
          face: label ? getComputedStyle(label).fontFamily : "",
          outline: latest ? getComputedStyle(latest).outlineColor : "",
          gilt: colour("--highlight"),
          tint: colour("--tint"),
        };
      });
      expect(touches.face.includes("Cinzel")).toBe(name === "Parchemin");
      expect(touches.outline).toBe(
        name === "Parchemin" ? touches.gilt : touches.tint,
      );
      await snap(page, `fr-${which}-${name.toLowerCase()}`);
    }
  }
});

/** A long weapon and a long item name: the act panel wraps them, never cuts (ADR 0022 step 1). */
const LONG_WEAPON = "Two-Handed Flamberge of the Seventh Drowned Kingdom +3";
const LONG_ITEM = "Potion of Extra-Healing brewed by the Hermit of Grey Hollow";
const ACT_TABS = ["Weapons", "Spells", "Items", "Skills"] as const;
/** The Spells tab's preparation in the layout audits; Rary's is cast in French (blank extra slots). */
const PREPARED = [
  "rarys_mnemonic_enhancer",
  "magic_missile",
  "burning_hands",
  "chromatic_orb",
  "tashas_uncontrollable_hideous_laughter",
  "mirror_image",
  "fireball",
  "haste",
  "stoneskin",
  "cloudkill",
];

// ADR 0022 steps 1, 2.5, 3.3 and 4: each tab of the act panel at every layout viewport, with long
// weapon, item and spell names and a bow's ammunition, shows no cut text and no page scroll; the
// weapon in hand is one row whose Attack is the 44 px control with its icon centred in a 44 x 44
// cell, and the act panel is as tall as its content (no empty band under the weapon). Screenshots
// of every tab at 1440x900 and 1707x1300.
test("act panel: every tab from 1280 to 2560, long names, no cut text", async ({
  page,
  stack,
}) => {
  test.setTimeout(60_000);
  const api = (method: string, url: string, data: unknown) =>
    fetch(`${stack.apiUrl}${url}`, {
      method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    }).then((r) => expect(r.ok, `${method} ${url}`).toBe(true));
  await api("PATCH", "/api/weapons/darts", {
    name: { en: LONG_WEAPON, fr: LONG_WEAPON },
  });
  await api("PATCH", "/api/items/power_gem_dispel", {
    name: { en: LONG_ITEM, fr: LONG_ITEM },
  });
  await api("PUT", "/api/spells/prepared", { spells: PREPARED });
  await openDashboard(page);
  await openWeapons(page);
  await chooseWeapon(page, "Long Sword +2");
  await chooseWeapon(page, "Short Bow +2");
  await expect(ammoChips(page)).toBeVisible();
  const actions = page.getByRole("region", { name: "Actions" });
  const panel = actions.getByRole("tabpanel");
  for (const vp of VIEWPORTS) {
    await page.setViewportSize(vp);
    const size = `${vp.width}x${vp.height}`;
    for (const tab of ACT_TABS) {
      await actions.getByRole("tab", { name: tab }).click();
      await expect(panel).toHaveAttribute("id", `panel-${tab.toLowerCase()}`);
      await expect(panel).not.toBeEmpty();
      if (tab === "Spells")
        await expect(panel).toContainText("Hideous Laughter");
      await expectNoPageScroll(page);
      await expectNoClippedText(actions, `${tab} tab, ${size}`);
      if (SHOTS.has(size)) await snap(page, `act-${tab.toLowerCase()}-${size}`);
      if (tab === "Spells") {
        // ADR 0027 step 11: the preparation editor in the tab's place, in English.
        await actions.getByRole("button", { name: "Prepare spells" }).click();
        await expect(
          actions.getByRole("form", { name: "Prepare today's spells" }),
        ).toContainText("Hideous Laughter");
        await expectNoPageScroll(page);
        await expectNoClippedText(actions, `preparation, ${size}`);
        await actions.getByRole("button", { name: "Cancel" }).click();
      }
    }
    await actions.getByRole("tab", { name: "Weapons" }).click();
    // The weapon row: Attack is 44 px tall, its sword centred in its own 44 x 44 cell (step 3.3).
    const attack = attackForm(page).getByRole("button", {
      name: "Attack",
      exact: true,
    });
    const btn = (await attack.boundingBox())!;
    expect(Math.round(btn.height), `Attack height, ${size}`).toBe(44);
    const icon = (await attack.locator("svg").boundingBox())!;
    expect(
      Math.abs(icon.y + icon.height / 2 - (btn.y + btn.height / 2)),
    ).toBeLessThanOrEqual(1);
    expect(
      Math.abs(icon.x + icon.width / 2 - (btn.x + 22)),
    ).toBeLessThanOrEqual(1);
    // No empty band: the act panel ends under its content (the crit table line) plus its padding,
    // unless the result panel beside it is taller.
    const content = (await page.getByTestId("crit-table").boundingBox())!;
    const box = (await actions.boundingBox())!;
    const result = (await page
      .getByRole("region", { name: "Detail", exact: true })
      .boundingBox())!;
    if (result.height < box.height - 1)
      expect(
        box.y + box.height - (content.y + content.height),
        `space under the weapons, ${size}`,
      ).toBeLessThanOrEqual(24);
  }
  // The long weapon in hand wraps inside its row at the narrowest viewport.
  await page.setViewportSize(VIEWPORTS[0]);
  await chooseWeapon(page, LONG_WEAPON);
  await expectNoClippedText(actions, "long weapon in hand, 1280x800");
  await expectInside(
    page.getByRole("region", { name: "Weapon in hand" }),
    actions,
    "long weapon in hand",
  );
  await snap(page, "act-weapons-long-1280x800");
  // Items: "Manage the inventory" focuses the Inventory activity.
  await actions.getByRole("tab", { name: "Items" }).click();
  await page.getByRole("button", { name: "Manage the inventory" }).click();
  await expect(
    page
      .getByRole("navigation", { name: "Activities" })
      .getByRole("button", { name: "Inventory" }),
  ).toHaveAttribute("aria-current", "page");
});

// ADR 0022 step 1 in French (the longest texts: "Bâton/Sceptre/Baguette", "Métamorphose") and with
// long inventory names: at every layout viewport the character card, the act panel, the lookup (the
// long spell selected, no automatic hyphenation) and the Inventory stage show no cut text and no
// word broken mid-word. Only a browser lays text out, so no Go or unit test can hold this.
test("no cut text in French and with long inventory names, 1280 to 2560", async ({
  page,
  stack,
}) => {
  test.setTimeout(90_000);
  const api = (method: string, url: string, data: unknown) =>
    fetch(`${stack.apiUrl}${url}`, {
      method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    }).then((r) => expect(r.ok, `${method} ${url}`).toBe(true));
  await api("PATCH", "/api/weapons/darts", {
    name: { en: LONG_WEAPON, fr: LONG_WEAPON },
  });
  await api("PATCH", "/api/items/power_gem_dispel", {
    name: { en: LONG_ITEM, fr: LONG_ITEM },
  });
  // The Spells tab with the long Tasha's name, a used slot and blank extra slots (ADR 0027 step 10).
  await api("PUT", "/api/spells/prepared", { spells: PREPARED });
  const slots = (await (
    await fetch(`${stack.apiUrl}/api/spells/slots`)
  ).json()) as { prepared_spell_id: string }[];
  const rary = slots.findIndex(
    (sl) => sl.prepared_spell_id === "rarys_mnemonic_enhancer",
  );
  await api("POST", `/api/spells/${rary}/cast`, {});
  // A known AC and both rare modifiers set: the target bar at its widest (ADR 0027 step 7.2).
  await api("PATCH", "/api/attack/form", {
    targetAc: -10,
    targetSize: "l",
    situational: { value: -2, label: "couvert" },
  });
  // The header's effects (ADR 0027 step 4): the brand round's table, Protection From Evil the
  // longest name, two of them behind the expander (the name column is sized on them too).
  for (const buff of [
    { name: "Bless", duration: 6 },
    { name: "Haste", duration: 3 },
    { name: "Stoneskin", charges: 7 },
    { name: "Protection From Evil", duration: 10 },
    { name: "Mirror Image", duration: 4 },
  ])
    await api("POST", "/api/buffs", { effects: {}, ...buff });
  // The journal with two turns, a transaction, a dice total and an undone row (ADR 0027 step 9).
  await api("POST", "/api/hp/event", { delta: -3, source: LONG_SOURCE });
  await api("POST", "/api/round/advance", {});
  await api("POST", "/api/roll/dice", { formula: "2d6+1" });
  await api("POST", "/api/hp/event", { delta: 2, source: "Potion de soins" });
  const log = (await (await fetch(`${stack.apiUrl}/api/log`)).json()) as {
    topId: number;
  };
  await api("POST", "/api/log/undo", { expectedTopId: log.topId });
  // Two XP entries, one with a long reason and a session (ADR 0027 step 14.4).
  await api("POST", "/api/xp", {
    base: 2500,
    bonus: true,
    reason: LONG_REASON,
    session: "S41",
  });
  await api("POST", "/api/xp", { base: 300, bonus: false, reason: "Troll" });
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  const character = page.getByTestId("character");
  const stage = page.getByTestId("stage");
  // The rail's buttons in order (the names are French here): Act, Spells, XP, Inventory.
  const rail = page.getByTestId("rail").getByRole("button");
  const FR_SPELL =
    "Nom de sortilège invraisemblablement long des anciens archimages";
  // The last result: an attack, in French.
  await page.getByRole("button", { name: /^Attaquer/ }).click();
  await expect(page.getByTestId("last-result-text")).toContainText("Attaque");
  // An effect's actions show on the row's hover and on keyboard focus only (ADR 0027 step 4.4).
  const firstBuff = page.getByTestId("buff").first();
  const buffActions = firstBuff.getByTestId("buff-actions");
  await page.mouse.move(0, 0);
  await expect(buffActions).toHaveCSS("opacity", "0");
  await firstBuff.getByTestId("effect-bar").hover();
  await expect(buffActions).toHaveCSS("opacity", "1");
  await page.mouse.move(0, 0);
  await expect(buffActions).toHaveCSS("opacity", "0");
  await buffActions.getByRole("button").first().focus();
  await expect(buffActions).toHaveCSS("opacity", "1");
  await buffActions.getByRole("button").first().blur();
  for (const vp of VIEWPORTS) {
    await page.setViewportSize(vp);
    const where = `fr, ${vp.width}x${vp.height}`;
    // The whole screen (header, rail, journal, dice footer and the shared pieces in every zone).
    await expectNoClippedText(page.locator("body"), `screen, ${where}`);
    await rail.nth(0).click();
    await expectHeaderLayout(page, where);
    await expectNoClippedText(character, `character card, ${where}`);
    await expectWholeWords(character, `character card, ${where}`);
    if (SHOTS.has(`${vp.width}x${vp.height}`))
      await expectCharacterCardLines(page, where);
    if (vp.width === 1440) {
      // ADR 0022 step 2.8 in French (ADR 0027 step 5.5): the name to the fifth save, no scroll.
      await expectNoScroll(character, `character card, ${where}`);
      await expectInside(
        character.getByTestId("save-row").last(),
        character,
        `fifth save, ${where}`,
      );
    }
    await rail.nth(0).click();
    await expectNoClippedText(stage, `act, ${where}`);
    {
      await snap(page, `target-bar-fr-${vp.width}x${vp.height}`);
      // ADR 0027 steps 1.5 and 7.2: AC, Inconnue, the size chips and the toggle on one line; the tag may wrap.
      const bar = page.getByRole("group", { name: "Cible" });
      const middle = async (l: Locator) => {
        const b = (await l.boundingBox())!;
        return b.y + b.height / 2;
      };
      const ac = await middle(bar.getByLabel("CA de la cible"));
      for (const el of [
        bar.getByRole("button", { name: "Inconnue" }),
        bar.getByRole("radiogroup", { name: "Taille de la cible" }),
        bar.getByRole("button", { name: "Modificateur situationnel" }),
      ])
        expect(
          Math.abs((await middle(el)) - ac),
          `target bar on one line, ${where}`,
        ).toBeLessThanOrEqual(2);
      await expect(bar.getByTestId("modifier-tag")).toHaveCount(1);
    }
    if (vp.width === 1440) {
      // ADR 0027 step 6: the now band is as tall as the Weapons tab, which does not scroll, and the
      // journal (at least 14rem) takes the rest.
      const weapons = page.getByRole("tabpanel");
      await expect(weapons).toHaveAttribute("id", "panel-weapons");
      await expectNoScroll(weapons, `Weapons tab, ${where}`);
      expect(
        (await page.getByTestId("journal-slot").boundingBox())!.height,
        `journal height, ${where}`,
      ).toBeGreaterThanOrEqual(224);
    }
    // The preparation's entry lists every prepared spell: it wraps.
    await expectCompactJournal(
      page,
      [
        LONG_SOURCE,
        LONG_WEAPON,
        LONG_ITEM,
        "Tasha's Uncontrollable",
        LONG_REASON,
      ],
      where,
    );
    await expect(
      combatLog(page).locator('[data-status="voided"]'),
    ).toContainText("Annulé");
    if (SHOTS.has(`${vp.width}x${vp.height}`))
      await snap(page, `act-weapons-fr-${vp.width}x${vp.height}`);
    // ADR 0027 step 10: the Spells tab's rows, dots and split buttons, and the details popup.
    const actions = page.getByRole("region", { name: "Actions" });
    await actions.getByRole("tab", { name: "Sorts" }).click();
    await expect(actions).toContainText("Hideous Laughter");
    await expectNoClippedText(actions, `spells tab, ${where}`);
    await expectWholeWords(actions, `spells tab, ${where}`);
    if (SHOTS.has(`${vp.width}x${vp.height}`)) {
      await snap(page, `act-spells-fr-${vp.width}x${vp.height}`);
      await actions
        .getByRole("button", {
          name: "Tasha's Uncontrollable Hideous Laughter",
          exact: true,
        })
        .click();
      const details = page.getByRole("dialog");
      await expect(details.getByTestId("spell-meta")).toBeVisible();
      await expectNoClippedText(details, `spell details, ${where}`);
      await expectWholeWords(details, `spell details, ${where}`);
      await snap(page, `spell-details-fr-${vp.width}x${vp.height}`);
      await page.keyboard.press("Escape");
      await expect(details).toHaveCount(0);
    }
    // ADR 0027 step 11: the preparation editor in place, a level over budget (its warning shown).
    await actions.getByRole("button", { name: "Préparer les sorts" }).click();
    const editor = actions.getByRole("form", {
      name: "Préparer les sorts du jour",
    });
    await editor
      .getByTestId("draft-level-1")
      .getByRole("button", { name: /^Une copie de plus/ })
      .first()
      .click();
    await expect(editor.getByTestId("over-budget")).toBeVisible();
    await expectNoPageScroll(page);
    await expectNoClippedText(actions, `preparation, ${where}`);
    await expectWholeWords(actions, `preparation, ${where}`);
    if (SHOTS.has(`${vp.width}x${vp.height}`))
      await snap(page, `act-prepare-fr-${vp.width}x${vp.height}`);
    await editor.getByRole("button", { name: "Annuler" }).click();
    // ADR 0027 step 12: the Skills tab in French; at 1440x900 both blocks side by side in the
    // panel, which does not scroll.
    await actions.getByRole("tab", { name: "Compétences" }).click();
    const skills = actions.getByRole("tabpanel");
    await expect(skills).toHaveAttribute("id", "panel-skills");
    await expect(skills.getByTestId("skill-modifier").first()).toBeVisible();
    await expectNoClippedText(actions, `skills tab, ${where}`);
    await expectWholeWords(actions, `skills tab, ${where}`);
    if (vp.width === 1440) {
      await expectNoScroll(skills, `Skills tab, ${where}`);
      // The bard's block holds its skills, the armour and the Spend button (ADR 0030 step 18).
      const [proficiencies, bard] = await skills
        .getByTestId("skills-blocks")
        .locator(":scope > *")
        .all();
      for (const [block, what] of [
        [proficiencies, "proficiencies"],
        [bard, "bard skills"],
      ] as const)
        await expectInside(block, skills, `${what}, ${where}`);
      const a = (await proficiencies.boundingBox())!;
      const b = (await bard.boundingBox())!;
      expect(b.y, `skills blocks side by side, ${where}`).toBe(a.y);
      // A hovered row's modifier chip stays one tone off the row (the chip piece, ADR 0026 step
      // 5.3): the row lights to the hover tone, the chip steps back to the raised one.
      const chip = skills.getByTestId("skill-modifier").first();
      const row = chip.locator("xpath=ancestor::button[1]");
      await row.hover();
      await row.evaluate((el) =>
        Promise.all(el.getAnimations().map((a) => a.finished)),
      );
      const bg = (l: Locator) =>
        l.evaluate((el) => getComputedStyle(el).backgroundColor);
      expect(await bg(chip), `hovered row's chip, ${where}`).not.toBe(
        await bg(row),
      );
    }
    if (SHOTS.has(`${vp.width}x${vp.height}`))
      await snap(page, `act-skills-fr-${vp.width}x${vp.height}`);
    await actions.getByRole("tab", { name: "Armes" }).click();
    await rail.nth(1).click();
    await page.getByRole("option", { name: FR_SPELL }).click();
    await expectNoClippedText(stage, `lookup, ${where}`);
    await expectWholeWords(stage, `lookup, ${where}`);
    // A spell that adds a buff: the cast box's "Ajouter comme effet" (ADR 0004 step 19).
    await page.getByRole("option", { name: "Image miroir" }).click();
    await expect(stage.getByTestId("spell-add-effect")).toBeVisible();
    await expectNoClippedText(stage, `lookup, effect, ${where}`);
    await expectWholeWords(stage, `lookup, effect, ${where}`);
    // ADR 0027 step 13.4: every stat tile's label on one line in French ("Incantation").
    if (SHOTS.has(`${vp.width}x${vp.height}`)) {
      for (const label of await stage
        .getByTestId("spell-detail")
        .locator("dt")
        .all()) {
        const lines = await label.evaluate((el) => {
          const lh = parseFloat(getComputedStyle(el).lineHeight);
          return Math.round(el.getBoundingClientRect().height / lh);
        });
        expect(
          lines,
          `stat label "${await label.textContent()}", ${where}`,
        ).toBe(1);
      }
      // ADR 0027 steps 13.1 and 1.5: the school behind "Filtres", a tag on the count line when set.
      await stage.getByRole("button", { name: "Filtrer par école" }).click();
      await page
        .getByRole("menuitemradio", { name: "Illusion/Fantasme", exact: true })
        .click();
      await expect(stage.getByTestId("lookup-group-tag")).toBeVisible();
      // The filtered rows are in (the count drops to the Illusion/Phantasm wizard spells).
      await expect(stage.getByRole("option", { name: "Armor" })).toHaveCount(0);
      await page.getByRole("option", { name: "Image miroir" }).click();
      await expectNoClippedText(stage, `lookup, school set, ${where}`);
      await snap(page, `lookup-fr-${vp.width}x${vp.height}`);
      await stage
        .getByRole("button", { name: "Retirer le filtre Illusion/Fantasme" })
        .click();
      await expect(stage.getByTestId("lookup-group-tag")).toHaveCount(0);
    }
    // ADR 0027 step 14.4: the ledger card is as tall as its rows (two entries and the opening row),
    // not the rest of the stage.
    await rail.nth(3).click();
    const ledger = stage.getByTestId("xp-ledger");
    await expect(ledger.getByTestId("xp-row")).toHaveCount(2);
    await expectNoClippedText(stage, `experience, ${where}`);
    await expectWholeWords(stage, `experience, ${where}`);
    const ledgerBox = (await ledger.boundingBox())!;
    const opening = (await ledger.getByTestId("xp-opening").boundingBox())!;
    expect(
      ledgerBox.y + ledgerBox.height - (opening.y + opening.height),
      `ledger card ends under its last row, ${where}`,
    ).toBeLessThanOrEqual(24);
    if (SHOTS.has(`${vp.width}x${vp.height}`))
      await snap(page, `experience-fr-${vp.width}x${vp.height}`);
    await rail.nth(2).click();
    await expectNoClippedText(stage, `inventory, ${where}`);
    await expectWholeWords(stage, `inventory, ${where}`);
    if (vp.width === 1707) {
      // ADR 0027 step 14.3: from a 1200 px stage, items and weapons side by side, both whole.
      const [items, weapons] = await stage
        .getByTestId("inventory-cards")
        .locator(":scope > section")
        .all();
      for (const [card, what] of [
        [items, "items"],
        [weapons, "weapons"],
      ] as const) {
        await expectInside(card, stage, `${what} card, ${where}`);
        await expectNoScroll(
          card.locator(":scope > div").last(),
          `${what} card, ${where}`,
        );
      }
      const a = (await items.boundingBox())!;
      const b = (await weapons.boundingBox())!;
      expect(b.y, `cards side by side, ${where}`).toBe(a.y);
      expect(b.x, `weapons right of items, ${where}`).toBeGreaterThan(
        a.x + a.width,
      );
    }
    if (SHOTS.has(`${vp.width}x${vp.height}`))
      await snap(page, `inventory-long-fr-${vp.width}x${vp.height}`);
  }
});

// What only a browser proves: the Inventaire dice chip and the Sorts "To the dice" fill the composer
// (a client store shared across workspaces), and Roll on a spell's dice reaches the dice endpoint
// and lands in the card and the journal. The formula wording is a unit test (spellValues), the
// values at level are Go tests (spellstats). Screens of both workspaces at the four viewports.
test.describe("workspaces to the dice composer", () => {
  test.use({
    extraSpells: {
      rows: [
        catalogueSpell({
          id: "quixotic_stillness",
          name: "Quixotic Stillness",
          class: "Wizard",
          level: 3,
          school: "Alteration",
          casting_time: "3",
          range: "Touch",
          duration: "8 rds. + 1d8 rds.",
          area_of_effect: "The caster",
        }),
      ],
    },
  });

  test("Inventaire dice chip and Sorts 'To the dice' fill the composer; Roll rolls on the server", async ({
    page,
  }) => {
    await openDashboard(page);
    const formula = page
      .getByRole("form", { name: "Dice" })
      .getByLabel("Formula");

    await openWorkspace(page, "Inventory");
    await page
      .getByTestId("item-fire_flask")
      .getByRole("button", { name: "Send 5d6 to the dice" })
      .click();
    await expect(formula).toHaveValue("5d6");
    for (const vp of VIEWPORTS) {
      await page.setViewportSize(vp);
      await expectNoPageScroll(page);
      await snap(page, `inventory-${vp.width}x${vp.height}`);
    }
    await page.setViewportSize({ width: 1440, height: 900 });

    await openWorkspace(page, "Spells");
    await page
      .getByRole("combobox", { name: "Search spells..." })
      .fill("quixotic still");
    const detail = page.getByTestId("spell-detail");
    await expect(detail.getByRole("heading")).toHaveText("Quixotic Stillness");
    await snap(page, "spells-short");
    const dice = detail.getByTestId("spell-dice-duration");
    await expect(dice.getByTestId("spell-dice-formula")).toHaveText("1d8+8");
    await dice.getByRole("button", { name: "Send 1d8+8 to the dice" }).click();
    await expect(formula).toHaveValue("1d8+8");

    await formula.fill("");
    await dice.getByRole("button", { name: "Roll 1d8+8" }).click();
    // The roll shows at once in the header's last result, from any activity (ADR 0022 step 2.2).
    const last = page.getByTestId("last-result");
    await expect(last.getByTestId("last-result-text")).toContainText("d8+8");
    const total = Number(
      await last.getByTestId("last-result-chip").textContent(),
    );
    expect(total).toBeGreaterThanOrEqual(9);
    expect(total).toBeLessThanOrEqual(16);
    await expect(formula).toHaveValue("d8+8");
    // In the Act activity the journal has the entry and the result panel shows it.
    await openWorkspace(page, "Act");
    await combatLog(page)
      .getByRole("listitem")
      .filter({ hasText: `Dice d8+8 = ${total}` })
      .getByTestId("log-select")
      .click();
    const result = page
      .getByRole("region", { name: "Detail", exact: true })
      .getByTestId("dice-result");
    // The server answers with its normal form (a single die has no count).
    await expect(result.getByTestId("dice-formula")).toHaveText("d8+8");
    await expect(result.getByTestId("dice-total")).toHaveText(String(total));
    await openWorkspace(page, "Spells");

    // The damage row: Fireball at the Bard's level, to the composer and rolled on the server.
    await page
      .getByRole("combobox", { name: "Search spells..." })
      .fill("fireball");
    await expect(detail.getByRole("heading")).toContainText("Fireball");
    const damage = detail.getByTestId("spell-dice-damage");
    await expect(damage.getByTestId("spell-dice-formula")).toHaveText(
      "10d6 fire",
    );
    await damage.getByRole("button", { name: "Send 10d6 to the dice" }).click();
    await expect(formula).toHaveValue("10d6");
    await damage.getByRole("button", { name: "Roll 10d6" }).click();
    await expect(last.getByTestId("last-result-text")).toContainText("10d6");
    const fire = Number(
      await last.getByTestId("last-result-chip").textContent(),
    );
    expect(fire).toBeGreaterThanOrEqual(10);
    expect(fire).toBeLessThanOrEqual(60);

    for (const vp of VIEWPORTS) {
      await page.setViewportSize(vp);
      await expectNoPageScroll(page);
      await snap(page, `spells-lookup-${vp.width}x${vp.height}`);
    }
  });
});

// ADR 0006 step 5 on the save card: with an adjustment it shows four facts (d20, target, adjusted
// target, margin) and the adjustment line; nothing is cut from 1280x800 to 2560x1440, en and fr.
test("adjusted save card: four facts and a long reason uncut, en and fr, 1280 to 2560", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await openDashboard(page);
  await page.getByRole("button", { name: "Roll save vs Spells" }).click();
  const card = page.getByTestId("save-result");
  await expect(card).toBeVisible();
  const form = await openFold(
    card,
    "Adjust after the roll",
    "Adjust this save",
  );
  const harder = form.getByRole("button", { name: "Decrease adjustment" });
  await harder.click();
  await harder.click();
  await form
    .getByLabel("Reason (optional)")
    .fill("Cursed by the lich's lingering aura");
  await form.getByRole("button", { name: "Apply" }).click();
  await expect(card.getByTestId("save-adjustment-line")).toContainText("−2");

  for (const lang of ["en", "fr"] as const) {
    if (lang === "fr") await chooseLanguage(page, "fr");
    await expect(card).toContainText(
      lang === "fr" ? "Seuil ajusté" : "Adjusted target",
    );
    for (const vp of [
      { width: 1280, height: 800 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      await expectNoClippedText(card, `adjusted save card, ${where}`);
      await expectWholeWords(card, `adjusted save card, ${where}`);
      await snap(page, `save-adjusted-${lang}-${vp.width}x${vp.height}`);
    }
  }
  await expect(
    combatLog(page)
      .getByRole("listitem")
      .filter({ hasText: "Sauvegarde contre Sorts" }),
  ).toContainText("(seuil ");
});

// ADR 0028 on the result card, French first: an ability check rolled from the character card's
// DEX tile, adjusted after the roll with a long reason; the card and the stat tiles are never cut,
// fr and en, 1280x800 to 2560x1440.
test("ability check card: adjusted with a long reason, uncut, fr and en, 1280 to 2560", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  await page.getByRole("button", { name: "Test de Dextérité (17)" }).click();
  const card = page.getByTestId("ability-result");
  await expect(card).toBeVisible();
  const form = await openFold(card, "Ajuster après le jet", "Ajuster ce test");
  const harder = form.getByRole("button", { name: "Diminuer l'ajustement" });
  await harder.click();
  await harder.click();
  await form
    .getByLabel("Raison (facultatif)")
    .fill("Corniche glissante sous la pluie battante");
  await form.getByRole("button", { name: "Appliquer" }).click();
  await expect(card.getByTestId("skill-adjustment-line")).toContainText("−2");

  for (const lang of ["fr", "en"] as const) {
    await chooseLanguage(page, lang);
    await expect(card).toContainText(
      lang === "fr" ? "Seuil ajusté" : "Adjusted target",
    );
    for (const vp of [
      { width: 1280, height: 800 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      await expectNoClippedText(card, `ability check card, ${where}`);
      await expectWholeWords(card, `ability check card, ${where}`);
      await expectNoClippedText(
        page.getByRole("group", {
          name: lang === "fr" ? "Caractéristiques" : "Ability scores",
        }),
        `ability tiles, ${where}`,
      );
      await snap(page, `ability-check-${lang}-${vp.width}x${vp.height}`);
    }
  }
});

// ADR 0027 step 8 on the attack card, French first: the verdict pill right after the result it
// judges, the stat tiles (d20, target AC, margin) and the AC correction opened from its one line;
// nothing is cut at any layout viewport, in French and English.
test("attack card: verdict beside the result, tiles and the open AC correction uncut, fr and en, 1280 to 2560", async ({
  page,
  stack,
}) => {
  const res = await fetch(`${stack.apiUrl}/api/attack/form`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ targetAc: 5 }),
  });
  expect(res.ok).toBe(true);
  await openDashboard(page);
  await attackForm(page)
    .getByRole("button", { name: "Attack", exact: true })
    .click();
  const card = page.getByTestId("attack-result");
  await expect(card.getByTestId("attack-tiles")).toBeVisible();
  await openFold(card, "Correct the target AC");

  for (const lang of ["fr", "en"] as const) {
    await chooseLanguage(page, lang);
    await expect(card.getByTestId("attack-tiles")).toContainText(
      lang === "fr" ? "CA cible" : "Target AC",
    );
    for (const vp of VIEWPORTS) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      // Step 8.1: the pill follows the result on its line, not at the card's far edge.
      const result = (await card.getByTestId("hits-ac").boundingBox())!;
      const pill = (await card.getByTestId("verdict").boundingBox())!;
      expect(
        pill.x - (result.x + result.width),
        `pill gap, ${where}`,
      ).toBeLessThan(16);
      expect(
        Math.abs(pill.y + pill.height / 2 - (result.y + result.height / 2)),
        `pill on the result's line, ${where}`,
      ).toBeLessThan(4);
      await expectNoClippedText(card, `attack card, ${where}`);
      await expectWholeWords(card, `attack card, ${where}`);
      if (SHOTS.has(`${vp.width}x${vp.height}`))
        await snap(page, `attack-card-${lang}-${vp.width}x${vp.height}`);
    }
  }
});

test("the last result is one short line, at most a sixth of the viewport, the full wording in its tooltip (French)", async ({
  page,
  stack,
}) => {
  const api = (method: string, url: string, data: unknown) =>
    fetch(`${stack.apiUrl}${url}`, {
      method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    }).then((r) => expect(r.ok, `${method} ${url}`).toBe(true));
  await api("PUT", "/api/spells/prepared", { spells: PREPARED });
  const slots = (await (
    await fetch(`${stack.apiUrl}/api/spells/slots`)
  ).json()) as { prepared_spell_id: string }[];
  const rary = slots.findIndex(
    (sl) => sl.prepared_spell_id === "rarys_mnemonic_enhancer",
  );
  await api("POST", `/api/spells/${rary}/cast`, {});
  await api("POST", "/api/buffs", {
    effects: {},
    name: "Protection From Evil",
    duration: 10,
  });
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  const last = page.getByTestId("last-result");
  await expect(last.getByTestId("last-result-text")).toContainText("Rary");
  for (const vp of [
    { width: 1280, height: 800 },
    { width: 1920, height: 1080 },
  ]) {
    await page.setViewportSize(vp);
    expect(
      await last.locator("p").evaluate(lineCount),
      `one line, ${vp.width}`,
    ).toBe(1);
    const b = (await last.boundingBox())!;
    expect(b.width, `at most a sixth, ${vp.width}`).toBeLessThanOrEqual(
      vp.width / 6 + 1,
    );
    // The full wording (journal text, round and time) is the tooltip.
    await expect(last.locator("p")).toHaveAttribute("title", /Rary/);
    await snap(page, `last-result-long-spell-fr-${vp.width}x${vp.height}`);
  }
});

// ADR 0026 step 6.7: every in-card group heading (the spell list's and the preparation's levels,
// the skills' groups, the inventory's column heads, the character card's groups) is the one shared
// heading band, in French, in a dark and a light theme. Only a browser paints the tones, so the
// screenshots (and the no-cut audit on them) are the check.
test("group headings: one shared band, French, dark and light", async ({
  page,
  stack,
}) => {
  test.setTimeout(60_000);
  const put = await fetch(`${stack.apiUrl}/api/spells/prepared`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ spells: PREPARED }),
  });
  expect(put.ok).toBe(true);
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  const rail = page.getByRole("navigation", { name: "Activités" });
  const actions = page.getByRole("region", { name: "Actions" });
  for (const [mode, which, theme] of [
    ["Sombre", "dark", "Arcane"],
    ["Clair", "light", "Parchemin"],
  ] as const) {
    await chooseSetting(page, mode);
    await chooseTheme(page, theme);
    await page.waitForTimeout(150); // let colour transitions settle
    const tag = `${which}-${theme.toLowerCase()}`;
    await rail.getByRole("button", { name: "Agir", exact: true }).click();
    await actions.getByRole("tab", { name: "Sorts" }).click();
    await expect(actions.getByRole("tabpanel")).toContainText("Niveau 1");
    // Each level's head is the shared band, not the card's tone.
    await expect(
      actions.locator(".group-heading", { hasText: "Niveau 1" }).first(),
    ).toBeVisible();
    await expectNoClippedText(actions, `spell levels, ${tag}`);
    await snap(page, `fr-${tag}-spells`);
    await actions.getByRole("button", { name: "Préparer les sorts" }).click();
    const form = actions.getByRole("form", {
      name: "Préparer les sorts du jour",
    });
    await expect(form.getByTestId("grimoire-level-1")).toBeVisible();
    await expectNoClippedText(actions, `preparation, ${tag}`);
    await snap(page, `fr-${tag}-preparation`);
    await actions.getByRole("button", { name: "Annuler" }).click();
    await actions.getByRole("tab", { name: "Compétences" }).click();
    await expect(actions.locator(".group-heading").first()).toBeVisible();
    await expectNoClippedText(actions, `skills, ${tag}`);
    await snap(page, `fr-${tag}-skills`);
    await rail.getByRole("button", { name: "Inventaire", exact: true }).click();
    await expectNoClippedText(page.locator("main"), `inventory, ${tag}`);
    await snap(page, `fr-${tag}-inventory`);
  }
});

// The spells tab in French at 1280x800 and 1920x1080: the Cast buttons of the spell list, then the
// preparation editor, whose two columns take the act panel's whole height side by side and each
// scroll inside themselves. Only a browser lays out the heights, so this is the check.
test("spells tab: Cast buttons and full-height preparation columns, French, 1280 and 1920", async ({
  page,
  stack,
}) => {
  test.setTimeout(60_000);
  const put = await fetch(`${stack.apiUrl}/api/spells/prepared`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ spells: PREPARED }),
  });
  expect(put.ok).toBe(true);
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  const actions = page.getByRole("region", { name: "Actions" });
  const panel = actions.getByRole("tabpanel");
  for (const vp of [
    { width: 1280, height: 800 },
    { width: 1920, height: 1080 },
  ]) {
    await page.setViewportSize(vp);
    const size = `${vp.width}x${vp.height}`;
    await actions.getByRole("tab", { name: "Sorts" }).click();
    await expect(panel).toContainText("Niveau 1");
    await expectNoClippedText(actions, `spell list, ${size}`);
    await snap(page, `fr-spells-cast-${size}`);
    await actions.getByRole("button", { name: "Préparer les sorts" }).click();
    const form = actions.getByRole("form", {
      name: "Préparer les sorts du jour",
    });
    await expect(form.getByTestId("grimoire-level-1")).toBeVisible();
    await expectNoClippedText(actions, `preparation, ${size}`);
    // The grimoire search's placeholder fits its field (the audit does not see placeholders).
    const search = form.getByRole("textbox", {
      name: "Chercher dans le grimoire",
    });
    const fit = await search.evaluate((el) => {
      const input = el as HTMLInputElement;
      const s = getComputedStyle(input);
      const ctx = document.createElement("canvas").getContext("2d")!;
      ctx.font = `${s.fontWeight} ${s.fontSize} ${s.fontFamily}`;
      const room =
        input.clientWidth -
        parseFloat(s.paddingLeft) -
        parseFloat(s.paddingRight);
      return { text: ctx.measureText(input.placeholder).width, room };
    });
    expect(fit.text, `search placeholder fits, ${size}`).toBeLessThanOrEqual(
      fit.room,
    );
    await snap(page, `fr-preparation-${size}`);
    // Both columns end at the panel's content bottom (its 14 px padding), nothing scrolls but them.
    const inner = (await panel.boundingBox())!;
    for (const name of ["Préparation du jour", "Grimoire"]) {
      const col = (await form.getByRole("region", { name }).boundingBox())!;
      expect(
        Math.abs(inner.y + inner.height - 14 - (col.y + col.height)),
        `${name} bottom, ${size}`,
      ).toBeLessThanOrEqual(2);
    }
    expect(
      await panel.evaluate((el) => el.scrollHeight - el.clientHeight),
      `act panel scroll, ${size}`,
    ).toBeLessThanOrEqual(1);
    await actions.getByRole("button", { name: "Annuler" }).click();
  }
});

// The login page (ADR 0022 step 1, ADR 0027 step 2): with both login buttons and the longest
// refusal message, no text is cut in French and English at every layout viewport; screenshots in
// French. The e2e API has no OAuth settings, so its login options are answered here with both.
test("login page: both buttons and a refusal, uncut, fr and en, 1280 to 2560", async ({
  page,
  context,
}) => {
  await context.clearCookies();
  await page.route("**/api/auth/options", (r) =>
    r.fulfill({ json: { google: true, devLogin: true } }),
  );
  await page.goto("/login?error=email_unverified");
  const card = page.getByTestId("login-card");
  for (const lang of ["fr", "en"] as const) {
    await page
      .getByRole("radio", { name: lang === "fr" ? "Français" : "English" })
      .click();
    await expect(card.getByTestId("login-error")).toBeVisible();
    await expect(card.getByRole("link")).toBeVisible();
    for (const vp of VIEWPORTS) {
      await page.setViewportSize(vp);
      await expectNoClippedText(card, `login page ${lang} ${vp.width}`);
      await expectWholeWords(card, `login page ${lang} ${vp.width}`);
      if (lang === "fr" && (vp.width === 1440 || vp.width === 1707))
        await snap(page, `login-fr-${vp.width}x${vp.height}`);
    }
  }
  // The dark themes (ADR 0026 step 4): the card and the refusal banner, in French, for review.
  await page.setViewportSize({ width: 1440, height: 900 });
  for (const dark of ["deep", "arcane"]) {
    await page.evaluate((darkTheme) => {
      localStorage.setItem(
        "critical.settings.v1",
        JSON.stringify({
          lang: "fr",
          mode: "dark",
          darkTheme,
          lightTheme: "arcane",
        }),
      );
    }, dark);
    await page.reload();
    await expect(page.locator("html")).toHaveClass(/dark/);
    await expect(card.getByTestId("login-error")).toBeVisible();
    await expectNoClippedText(card, `login page dark ${dark}`);
    await snap(page, `login-fr-dark-${dark}`);
  }
});

// ADR 0022 step 1 for the database banner (ADR 0025 step 10.4), French first: with both of its
// lines (a write whose outcome is unknown), it wraps and is never cut, fr and en, 1280x800 to
// 2560x1440.
test("database banner: both lines uncut, fr and en, 1280 to 2560", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await openDashboard(page);
  const unavailable = {
    status: 503,
    contentType: "application/json",
    body: JSON.stringify({
      code: "database_unavailable",
      error: "database unavailable",
    }),
  };
  await page.route("**/api/hp/event", (route) => route.fulfill(unavailable));
  await page.route("**/api/character", (route) => route.fulfill(unavailable));
  await recordHp(page, "damage", 4, "e2e lost blow");
  const banner = page.getByTestId("database-banner");
  await expect(banner).toContainText(
    "Your last action may not have been saved",
  );
  for (const lang of ["fr", "en"] as const) {
    await chooseLanguage(page, lang);
    for (const vp of [
      { width: 1280, height: 800 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      await expectNoClippedText(banner, `database banner, ${where}`);
      if (lang === "fr")
        await snap(page, `database-banner-${where.replace(", ", "-")}`);
    }
  }
});

// The admin section (ADR 0029 step 1.3), French first: a player with a long email and two
// characters' worth of names, a disabled user and a long pending invite; the invites and users cards
// are never cut, fr and en, 1280x800 to 2560x1440.
test("admin section: users and invites uncut, fr and en, 1280 to 2560", async ({
  page,
  context,
  stack,
}) => {
  test.setTimeout(60_000);
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  const api = (method: string, url: string, body: unknown) =>
    fetch(`${stack.apiUrl}${url}`, {
      method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    }).then(async (r) => {
      expect(r.ok, `${method} ${url}: ${r.status}`).toBe(true);
      return (await r.json()) as { id: string };
    });
  // The dev login creates admins: one stays an admin, the others become players, one disabled.
  await api("POST", "/api/auth/dev", {
    email: "aurelie.de-la-tour-dauvergne@compagnie-des-lames.example.com",
  });
  const bob = await api("POST", "/api/auth/dev", { email: "bob@example.com" });
  const cid = await api("POST", "/api/auth/dev", { email: "cid@example.com" });
  await api("PATCH", `/api/admin/users/${bob.id}`, { role: "player" });
  await api("PATCH", `/api/admin/users/${cid.id}`, {
    role: "player",
    disabled: true,
  });
  await api("POST", "/api/admin/invites", {
    email:
      "maximilien.vandenbroucke-lefebvre@guilde-des-arcanistes.example.org",
  });

  await page.setViewportSize({ width: 1280, height: 800 });
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  await page.goto("/admin");
  const invites = page.getByTestId("admin-invites");
  const users = page.getByTestId("admin-users");
  await expect(users.getByTestId("admin-user")).toHaveCount(4);
  await expect(invites.getByRole("listitem")).toHaveCount(1);
  await invites.getByRole("button", { name: /^Copier le lien/ }).click();
  await expect(invites.getByRole("status")).toHaveText("Lien copié");

  for (const lang of ["fr", "en"] as const) {
    if (lang === "en") {
      await page.goto("/");
      await chooseLanguage(page, "en");
      await page.goto("/admin");
      await expect(users.getByTestId("admin-user")).toHaveCount(4);
    }
    await expect(users).toContainText(
      lang === "fr" ? "Administrateur principal" : "Root admin",
    );
    for (const vp of [
      { width: 1280, height: 800 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      await expectNoClippedText(invites, `admin invites, ${where}`);
      await expectNoClippedText(users, `admin users, ${where}`);
      await snap(page, `admin-${lang}-${vp.width}x${vp.height}`);
    }
  }
});

// The character dialogs of the cog menu and the page of a user with no character (ADR 0029 step
// 7.2), French first: the creation form with its details, the rename dialog and the switch list (a
// long name) and the creation page are never cut, fr and en, 1280x800 and 2560x1440.
test("character dialogs and the creation page uncut, fr and en, 1280 and 2560", async ({
  page,
}) => {
  test.setTimeout(60_000);
  await page.setViewportSize({ width: 1280, height: 800 });
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  const call = (method: string, url: string, body: unknown) =>
    page.evaluate(
      async ([m, u, b]) => {
        const r = await fetch(u as string, {
          method: m as string,
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(b),
        });
        return {
          ok: r.ok,
          body: (await r.json()) as { characters: { id: number }[] },
        };
      },
      [method, url, body] as const,
    );
  const created = await call("POST", "/api/characters", {
    name: "Aldéric de la Tour-d'Auvergne-Lesparre",
    class: "bard",
    raceCode: "half_elf",
    details: { alignment: "neutral_good", sex: "male" },
    str: 10,
    dex: 15,
    con: 12,
    int: 14,
    wis: 9,
    cha: 16,
    hpMax: 6,
    xp: 0,
    skillArmor: "leather",
  });
  expect(created.ok).toBe(true);
  await page.reload();
  const dialog = page.getByTestId("character-dialog");

  for (const lang of ["fr", "en"] as const) {
    if (lang === "en") await chooseLanguage(page, "en");
    const [switchHint, newHint, renameHint] =
      lang === "fr"
        ? [
            /^Changer de personnage/,
            "Créer un nouveau personnage",
            "Renommer le personnage joué",
          ]
        : [
            /^Switch character/,
            "Create a new character",
            "Rename the character you play",
          ];
    for (const vp of [
      { width: 1280, height: 800 },
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      let settings = await openSettings(page);
      await expectNoClippedText(
        settings.getByTestId("settings-character"),
        `cog menu characters, ${where}`,
      );
      await settings.getByRole("button", { name: newHint }).click();
      await expect(dialog.getByTestId("character-form")).toBeVisible();
      await expectNoClippedText(dialog, `new character form, ${where}`);
      await expectWholeWords(dialog, `new character form, ${where}`);
      await snap(page, `character-form-${lang}-${vp.width}x${vp.height}`);
      await page.keyboard.press("Escape");
      await expect(dialog).toBeHidden();

      settings = await openSettings(page);
      await settings.getByRole("button", { name: renameHint }).click();
      await expect(dialog.getByTestId("rename-form")).toBeVisible();
      await expectNoClippedText(dialog, `rename dialog, ${where}`);
      await snap(page, `character-rename-${lang}-${vp.width}x${vp.height}`);
      await page.keyboard.press("Escape");
      await expect(dialog).toBeHidden();

      settings = await openSettings(page);
      await settings.getByRole("button", { name: switchHint }).click();
      await expect(
        dialog.getByTestId("character-list").getByRole("listitem"),
      ).toHaveCount(2);
      await expectNoClippedText(dialog, `switch list, ${where}`);
      await snap(page, `character-switch-${lang}-${vp.width}x${vp.height}`);
      await page.keyboard.press("Escape");
      await expect(dialog).toBeHidden();
    }
  }

  // Both characters retired: the creation page, the first one's lead (no character left).
  const list = await call("GET", "/api/characters", undefined);
  for (const c of list.body.characters)
    expect(
      (await call("PATCH", `/api/characters/${c.id}`, { retired: true })).ok,
    ).toBe(true);
  for (const lang of ["fr", "en"] as const) {
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/");
    const card = page.getByTestId("create-card");
    await expect(card.getByTestId("character-form")).toBeVisible();
    if (lang === "fr")
      await page.getByRole("radio", { name: "Français", exact: true }).click();
    else
      await page.getByRole("radio", { name: "English", exact: true }).click();
    for (const vp of [
      { width: 1280, height: 800 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      await expectNoClippedText(
        page.locator("body"),
        `creation page, ${where}`,
      );
      await snap(page, `create-page-${lang}-${vp.width}x${vp.height}`);
    }
    // A window shorter than the form: the page itself still never scrolls, the form's card does.
    await page.setViewportSize({ width: 1280, height: 600 });
    await expectNoPageScroll(page);
    await expect(card.getByTestId("character-form")).toBeVisible();
  }
});

// ADR 0032 step 4 and ADR 0022 step 1, judged in French (ADR 0027 step 2): the card's languages, in
// the edit (the catalogue's checkboxes, the race's list first, the count over the limit) and in the
// view (the names and the warning), are never cut from 1280 to 2560.
test("details card: the languages picker, its count and the warning uncut in French, 1280 to 2560", async ({
  page,
}) => {
  test.setTimeout(60_000);
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  await page.getByTestId("details-trigger").click();
  const pop = page.getByTestId("details-popover");
  await pop.getByRole("button", { name: "Modifier" }).click();
  for (const name of ["Elfique", "Gnome", "Halfelin", "Gobelin", "Orque"])
    await pop.getByRole("checkbox", { name, exact: true }).check();
  await expect(pop.getByTestId("language-count")).toContainText(
    "Hors langue natale : 5 sur 4",
  );
  for (const vp of [
    { width: 1280, height: 800 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expectNoClippedText(pop, `details card languages edit, ${where}`);
    await expectWholeWords(pop, `details card languages edit, ${where}`);
    await snap(page, `details-languages-edit-fr-${where}`);
  }
  await pop.getByRole("button", { name: "Enregistrer" }).click();
  await expect(pop.getByTestId("details-languages-warning")).toBeVisible();
  for (const vp of [
    { width: 1280, height: 800 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expectNoClippedText(pop, `details card languages view, ${where}`);
    await snap(page, `details-languages-view-fr-${where}`);
  }
});

// ADR 0030 step 7 and ADR 0022 step 1, judged in French (ADR 0027 step 2): the level-up prompt on
// the character card, at its widest (a hit die to roll, a second level owed, the manual entry
// open), is never cut from 1280 to 2560, and each button says one word (ADR 0027 step 3).
test("level-up prompt: the hit die and its manual entry uncut in French, 1280 to 2560", async ({
  page,
}) => {
  await openDashboard(page);
  const call = (method: string, url: string, body: unknown) =>
    page.evaluate(
      async ([m, u, b]) =>
        (
          await fetch(u as string, {
            method: m as string,
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(b),
          })
        ).ok,
      [method, url, body] as const,
    );
  // A level 1 bard (made current), then 2,500 XP: levels 2 and 3 owe a d6 each (Table 25).
  expect(
    await call("POST", "/api/characters", {
      name: "Aldéric de la Tour-d'Auvergne-Lesparre",
      class: "bard",
      raceCode: "half_elf",
      details: { alignment: "neutral_good", sex: "male" },
      str: 10,
      dex: 15,
      con: 12,
      int: 14,
      wis: 9,
      cha: 16,
      hpMax: 6,
      xp: 0,
      skillArmor: "leather",
    }),
  ).toBe(true);
  expect(await call("POST", "/api/xp", { base: 2500, bonus: false })).toBe(
    true,
  );
  await page.reload();
  await chooseLanguage(page, "fr");
  const prompt = page.getByTestId("level-up");
  await expect(prompt.getByTestId("level-up-title")).toHaveText(
    "Niveau 2 atteint",
  );
  await expect(prompt.getByTestId("level-up-more")).toBeVisible();
  for (const vp of VIEWPORTS) {
    await page.setViewportSize(vp);
    const where = `fr, ${vp.width}x${vp.height}`;
    await expectNoClippedText(prompt, `level-up prompt, ${where}`);
    await expectWholeWords(prompt, `level-up prompt, ${where}`);
    await prompt
      .getByRole("button", { name: "Saisir le dé lancé à la table" })
      .click();
    await expect(prompt.getByTestId("level-up-die")).toBeVisible();
    await expectNoClippedText(prompt, `level-up manual entry, ${where}`);
    await expectWholeWords(prompt, `level-up manual entry, ${where}`);
    if (SHOTS.has(`${vp.width}x${vp.height}`))
      await snap(page, `level-up-fr-${vp.width}x${vp.height}`);
    await prompt.getByRole("button", { name: "Fermer", exact: true }).click();
  }
});

// ADR 0030 steps 9, 11 and 12 in French at 1280x800 and 2560x1440: the 18/50 Strength tile, the
// attack card's rate, "spécialisée" tag and point-blank checkbox, and the weapon editor's Catégorie
// and Spécialisée controls, none of them cut. Screenshot of the weapon editor at both sizes.
test("fighter: 18/50 tile, attack card line and weapon editor uncut in French, 1280 to 2560", async ({
  page,
}) => {
  test.setTimeout(60_000);
  await page.setViewportSize({ width: 1280, height: 800 });
  await openDashboard(page);
  const call = (method: string, url: string, body: unknown) =>
    page.evaluate(
      async ([m, u, b]) => {
        const r = await fetch(u as string, {
          method: m as string,
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(b),
        });
        return r.ok || `${r.status} ${await r.text()}`;
      },
      [method, url, body] as const,
    );
  expect(
    await call("POST", "/api/characters", {
      name: "Borin Gueule-de-Fer",
      class: "fighter",
      raceCode: "human",
      details: { alignment: "neutral_good", sex: "male" },
      str: 18,
      strExceptional: 50,
      dex: 14,
      con: 15,
      int: 10,
      wis: 10,
      cha: 9,
      hpMax: 10,
      xp: 0,
    }),
  ).toBe(true);
  for (const w of [
    {
      name: { en: "Long Sword", fr: "Épée longue" },
      damage_sm: "1d8",
      damage_l: "1d12",
      kind: "melee",
      weapon_type: "long_sword",
      specialized: true,
    },
    {
      name: { en: "Heavy Crossbow", fr: "Arbalète lourde" },
      damage_sm: "1d4",
      damage_l: "1d6",
      kind: "missile",
      ammo_kind: "bolt",
      weapon_type: "heavy_crossbow",
      specialized: true,
    },
  ])
    expect(await call("POST", "/api/weapons", w), w.name.en).toBe(true);
  await page.reload();
  await expect(page.getByTestId("str-value")).toHaveText("18/50");

  await openWorkspace(page, "Act");
  await openWeapons(page);
  const f = page.getByRole("form", { name: "Attaque" });
  for (const weapon of ["Long Sword", "Heavy Crossbow"]) {
    await chooseLanguage(page, "en");
    await chooseWeapon(page, weapon);
    await chooseLanguage(page, "fr");
    await expect(f.getByTestId("tile-specialized")).toHaveText("spécialisée");
    await expect(f.getByTestId("tile-rate")).toContainText("att.");
    if (weapon === "Heavy Crossbow")
      await expect(f.getByTestId("point-blank")).toBeVisible();
    for (const vp of [
      { width: 1280, height: 800 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      const where = `${weapon}, fr, ${vp.width}x${vp.height}`;
      await expectNoClippedText(
        page.getByTestId("character"),
        `character card, ${where}`,
      );
      await expectNoClippedText(
        page.getByTestId("str-value"),
        `Strength tile, ${where}`,
      );
      await expectNoClippedText(f, `attack card, ${where}`);
      if (weapon === "Heavy Crossbow")
        await snap(page, `fighter-crossbow-fr-${vp.width}x${vp.height}`);
    }
    await page.setViewportSize({ width: 1280, height: 800 });
  }

  await chooseLanguage(page, "en");
  await openWorkspace(page, "Inventory");
  await chooseLanguage(page, "fr");
  for (const vp of [
    { width: 1280, height: 800 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    await page
      .getByTestId("weapon-long_sword")
      .getByRole("button", { name: "Modifier Épée longue" })
      .click();
    const wf = page.getByTestId("weapon-form");
    await expect(wf.getByLabel("Catégorie")).toBeVisible();
    await expect(wf.getByLabel("Spécialisée")).toBeChecked();
    const where = `weapon editor, fr, ${vp.width}x${vp.height}`;
    await expectNoClippedText(wf, where);
    await snap(page, `fighter-weapon-editor-fr-${vp.width}x${vp.height}`);
    await wf.getByRole("button", { name: "Annuler" }).click();
    await expect(wf).toBeHidden();
  }
});

// The mage's Learn control (ADR 0030 step 20) in the lookup's detail: the button, then the
// roll's answer (and "in the spellbook" once learned), uncut in French at 1280x800 and 2560x1440.
test("mage: the lookup's Learn button and its answer uncut in French, 1280 to 2560", async ({
  page,
}) => {
  test.setTimeout(60_000);
  await page.setViewportSize({ width: 1280, height: 800 });
  await openDashboard(page);
  const created = await page.evaluate(async () => {
    const r = await fetch("/api/characters", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        name: "Mordenkainen l'Archimage de Greyhawk",
        class: "mage",
        raceCode: "human",
        details: { alignment: "neutral_good", sex: "male" },
        str: 9,
        dex: 12,
        con: 11,
        int: 19,
        wis: 12,
        cha: 10,
        hpMax: 4,
        xp: 0,
      }),
    });
    return r.ok || `${r.status} ${await r.text()}`;
  });
  expect(created).toBe(true);
  await page.reload();
  await openWorkspace(page, "Spells");
  await chooseLanguage(page, "fr");
  await page
    .getByRole("combobox", { name: "Rechercher des sorts..." })
    .fill("Sleep");
  const detail = page.getByTestId("spell-detail");
  const learn = detail.getByTestId("learn-spell");
  // The search settles on Sleep before the click (the first row is selected meanwhile).
  await expect(detail.getByRole("heading")).toHaveText("Sommeil");
  await expect(learn.getByRole("button", { name: /^Apprendre/ })).toBeVisible();
  for (const vp of [
    { width: 1280, height: 800 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    await expectNoClippedText(
      learn,
      `Learn button, fr, ${vp.width}x${vp.height}`,
    );
  }
  await page.setViewportSize({ width: 1280, height: 800 });
  // The failure's answer, the longest one (with the server's retry level): the dice are real, so
  // the response is fixed here; the server's own failure is a Go test (learnsvc).
  await page.route("**/api/character/spellbook/learn", (route) =>
    route.fulfill({
      json: {
        spellId: "sleep",
        spellLevel: 1,
        roll: 97,
        chance: 95,
        learned: false,
        characterLevel: 1,
        retryLevel: 2,
      },
    }),
  );
  await learn.getByRole("button", { name: /^Apprendre/ }).click();
  await expect(learn.getByTestId("learn-result")).toContainText("niveau 2");
  for (const vp of [
    { width: 1280, height: 800 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expectNoClippedText(learn, `Learn failure, fr, ${where}`);
    await snap(page, `mage-learn-failed-fr-${where}`);
  }
  await page.unroute("**/api/character/spellbook/learn");
  await page.setViewportSize({ width: 1280, height: 800 });
  const answered = page.waitForResponse("**/api/character/spellbook/learn");
  await learn.getByRole("button", { name: /^Apprendre/ }).click();
  const real = (await (await answered).json()) as { roll: number };
  await expect(learn.getByTestId("learn-result")).toContainText(
    String(real.roll),
  );
  for (const vp of [
    { width: 1280, height: 800 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expectNoClippedText(learn, `Learn answer, fr, ${where}`);
    await expectNoClippedText(
      page.getByTestId("character"),
      `mage character card, fr, ${where}`,
    );
    await snap(page, `mage-learn-fr-${where}`);
  }
});

// The cleric's screens (ADR 0030 steps 14 and 16) uncut in French at 1280x800 and 2560x1440: the
// Turning tab (Table 61 at the realized level, the longest outcome labels), the turning result card
// and its journal row, and a cast's spell failure on the result card. Wisdom 9 gives a spell
// failure chance; the failed cast is a fixed response (the dice are real; the failure itself is a
// Go test, castsvc).
test("cleric: turning panel, turning result and spell failure uncut in French, 1280 to 2560", async ({
  page,
}) => {
  test.setTimeout(60_000);
  await page.setViewportSize({ width: 1280, height: 800 });
  await openDashboard(page);
  const created = await page.evaluate(async () => {
    const r = await fetch("/api/characters", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        name: "Frère Anselme de la Haute-Marche",
        class: "cleric",
        raceCode: "human",
        details: { alignment: "neutral_good", sex: "male" },
        str: 12,
        dex: 10,
        con: 13,
        int: 10,
        wis: 9,
        cha: 11,
        hpMax: 8,
        xp: 0,
      }),
    });
    if (!r.ok) return `${r.status} ${await r.text()}`;
    const p = await fetch("/api/spells/prepared", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ spells: ["bless"] }),
    });
    return p.ok || `${p.status} ${await p.text()}`;
  });
  expect(created).toBe(true);
  await page.reload();
  await openWorkspace(page, "Act");
  await chooseLanguage(page, "fr");
  await page.getByRole("tab", { name: "Renvoi" }).click();
  const turning = page.getByTestId("turn-undead");
  await expect(turning.getByTestId("turn-cell-skeleton")).toBeVisible();
  for (const vp of [
    { width: 1280, height: 800 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    await expectNoClippedText(
      turning,
      `turning panel, fr, ${vp.width}x${vp.height}`,
    );
  }
  await page.setViewportSize({ width: 1280, height: 800 });
  await turning.getByRole("button", { name: "Repousser : Squelette" }).click();
  const turned = page.getByTestId("turn-result");
  await expect(turned).toContainText("Clerc de niveau 1");
  for (const vp of [
    { width: 1280, height: 800 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expectNoClippedText(turned, `turning result, fr, ${where}`);
    await expectNoClippedText(
      page.getByTestId("card-header"),
      `turning card header, fr, ${where}`,
    );
    await expectNoClippedText(combatLog(page), `turning journal, fr, ${where}`);
    await snap(page, `cleric-turn-fr-${where}`);
  }

  await page.setViewportSize({ width: 1280, height: 800 });
  await page.route("**/api/spells/*/cast", (route) =>
    route.fulfill({
      json: {
        entryId: 999_999,
        source: "cast",
        spellId: "bless",
        name: "Bless",
        level: 1,
        casterLevel: 1,
        slotIndex: 0,
        untracked: [],
        failure: { d100: 12, chance: 20, failed: true },
      },
    }),
  );
  await page.getByRole("tab", { name: "Sorts" }).click();
  await page.getByRole("button", { name: "Lancer Bless" }).click();
  const failure = page.getByTestId("cast-failure");
  await expect(failure).toHaveAttribute("data-failed", "true");
  for (const vp of [
    { width: 1280, height: 800 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expectNoClippedText(failure, `spell failure, fr, ${where}`);
    await snap(page, `cleric-spell-failure-fr-${where}`);
  }
  await page.unroute("**/api/spells/*/cast");
});

// The thief's screens (ADR 0030 steps 1, 17 and 19) uncut in French at 1280x800 and 2560x1440: the
// creation form's skill points and armour fields, the Skills tab's thieving skills with the
// budget warnings (70 points at 1st, 40 in one skill: both breached) and the attack card's backstab
// option.
test("thief: creation fields, thieving skills with the budget warnings and backstab uncut in French, 1280 to 2560", async ({
  page,
}) => {
  test.setTimeout(60_000);
  await page.setViewportSize({ width: 1280, height: 800 });
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  const settings = await openSettings(page);
  await settings
    .getByRole("button", { name: "Créer un nouveau personnage" })
    .click();
  const form = page
    .getByTestId("character-dialog")
    .getByTestId("character-form");
  await form.getByLabel("Classe").selectOption({ label: "Voleur" });
  const entry = form.getByTestId("thief-skills-entry");
  for (const vp of [
    { width: 1280, height: 800 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expectNoClippedText(entry, `thief creation fields, fr, ${where}`);
    await snap(page, `thief-form-fr-${where}`);
  }
  await page.keyboard.press("Escape");

  await page.setViewportSize({ width: 1280, height: 800 });
  const call = (method: string, url: string, body: unknown) =>
    page.evaluate(
      async ([m, u, b]) => {
        const r = await fetch(u as string, {
          method: m as string,
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(b),
        });
        return r.ok || `${r.status} ${await r.text()}`;
      },
      [method, url, body] as const,
    );
  expect(
    await call("POST", "/api/characters", {
      name: "Vex Main-Légère de la Basse-Ville",
      class: "thief",
      raceCode: "halfling",
      details: { alignment: "neutral_good", sex: "male" },
      str: 16,
      dex: 17,
      con: 12,
      int: 11,
      wis: 9,
      cha: 10,
      hpMax: 5,
      xp: 0,
      skillArmor: "padded_studded_leather",
      skillPoints: { find_remove_traps: 40, hide_in_shadows: 30 },
    }),
  ).toBe(true);
  expect(
    await call("POST", "/api/weapons", {
      name: { en: "Dagger", fr: "Dague" },
      damage_sm: "1d4",
      damage_l: "1d3",
      kind: "melee",
      weapon_type: "dagger",
    }),
  ).toBe(true);
  await page.reload();
  await chooseLanguage(page, "en");
  await openWorkspace(page, "Act");
  await chooseLanguage(page, "fr");
  await page.getByRole("tab", { name: "Compétences" }).click();
  const skills = page.getByTestId("skills-blocks");
  await expect(page.getByTestId("skill-budget")).toContainText("70 au total");
  for (const vp of [
    { width: 1280, height: 800 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expectNoClippedText(skills, `thieving skills, fr, ${where}`);
    // The armour select (its longest choice) and its label sit uncut in the block.
    const picker = page.getByTestId("thief-armor-picker");
    await expectNoClippedText(picker, `thief armour picker, fr, ${where}`);
    const select = page.getByTestId("thief-armor");
    expect(
      await select.evaluate((el) => el.scrollWidth <= el.clientWidth),
    ).toBe(true);
    await snap(page, `thief-skills-fr-${where}`);
  }

  await page.setViewportSize({ width: 1280, height: 800 });
  await page.getByRole("tab", { name: "Armes" }).click();
  const f = page.getByRole("form", { name: "Attaque" });
  await f.getByTestId("backstab").check();
  for (const vp of [
    { width: 1280, height: 800 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expectNoClippedText(f, `backstab attack card, fr, ${where}`);
    await snap(page, `thief-backstab-card-fr-${where}`);
  }
});
