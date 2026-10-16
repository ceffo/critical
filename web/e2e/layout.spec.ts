import { test, expect, snap, catalogueSpell } from "./fixtures";
import {
  addBuff,
  ammoChips,
  attackForm,
  chooseLanguage,
  chooseSetting,
  chooseWeapon,
  combatLog,
  nextTurn,
  openDashboard,
  openSettings,
  openWeapons,
  openWorkspace,
  recordHp,
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

// ADR 0022 step 1 (layout viewports). 1707x1300 is a third of the user's 5120x1440 table screen.
const VIEWPORTS = [
  { width: 1280, height: 800 },
  { width: 1440, height: 900 },
  { width: 1707, height: 1300 },
  { width: 1920, height: 1080 },
  { width: 2560, height: 1440 },
];
const ACTIVITIES = ["Act", "Spells", "Experience", "Inventory"] as const;
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
  await expect(last.getByTestId("last-result-text")).toContainText(
    "Long Sword +2 vs AC 5",
  );
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
    // Below 1500 px the mark shows without the wordmark (ADR 0022 step 7).
    await expect(page.getByTestId("wordmark")).toBeVisible({
      visible: vp.width >= 1500,
    });
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
        const stage = (await page.getByTestId("stage").boundingBox())!;
        expect(
          Math.abs(ledgerBox.y + ledgerBox.height - (stage.y + stage.height)),
          `ledger takes the rest of the height, ${where}`,
        ).toBeLessThanOrEqual(1);
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
      // ADR 0022 step 5.1: an entry is one line (a 28 px row); only a long one wraps.
      const row = (text: string) =>
        combatLog(page).getByRole("listitem").filter({ hasText: text });
      for (const text of ["Long Sword +2 vs AC 5", "Initiative"])
        expect(
          (await row(text).first().boundingBox())!.height,
          `one line: ${text}`,
        ).toBeLessThanOrEqual(32);
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

test("settings: language, appearance and ambiance apply live and persist across reload", async ({
  page,
}) => {
  await page.emulateMedia({ colorScheme: "light" });
  await openDashboard(page);
  expect(await isDark(page)).toBe(false); // System by default, the OS is light

  await chooseSetting(page, "Dark");
  expect(await isDark(page)).toBe(true);
  await chooseSetting(page, "Blood");
  await expect.poll(() => rootVar(page, "--tint")).toMatch(/ 15\)$/); // Blood's hue
  await chooseLanguage(page, "fr");
  await expect(
    page.getByRole("button", { name: "Tour suivant" }),
  ).toBeVisible();

  await page.reload();
  expect(await isDark(page)).toBe(true); // painted before the app mounts
  await expect(
    page.getByRole("button", { name: "Tour suivant" }),
  ).toBeVisible();
  expect(await rootVar(page, "--tint")).toMatch(/ 15\)$/);
  const popover = await openSettings(page);
  for (const name of ["Français", "Sombre", "Sang"])
    await expect(
      popover.getByRole("radio", { name, exact: true }),
    ).toHaveAttribute("aria-checked", "true");
  await snap(page, "settings-popover-fr");

  // Système follows the OS live.
  await popover.getByRole("radio", { name: "Système", exact: true }).click();
  expect(await isDark(page)).toBe(false);
  await page.emulateMedia({ colorScheme: "dark" });
  await expect.poll(() => isDark(page)).toBe(true);
});

// The visual identity (ADR 0020): the main screens in dark and light with content in the result
// card and the journal (Act/Weapons after an attack, Spells, Inventory, Experience), then every
// ambiance on the dashboard. Screenshots only: the colours themselves are unit-tested (theme.test).
test("theme: the main screens and every ambiance in dark and light", async ({
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
    await openWorkspace(page, "Act");
    for (const amb of [
      "Arcane",
      "Grove",
      "Blood",
      "Ocean",
      "Amber",
      "Obsidian",
    ]) {
      await chooseSetting(page, amb);
      await page.waitForTimeout(150); // let colour transitions settle
      await snap(page, `${m}-${amb.toLowerCase()}`);
    }
    await chooseSetting(page, "Arcane");
  }
});

/** A long weapon and a long item name: the act panel wraps them, never cuts (ADR 0022 step 1). */
const LONG_WEAPON = "Two-Handed Flamberge of the Seventh Drowned Kingdom +3";
const LONG_ITEM = "Potion of Extra-Healing brewed by the Hermit of Grey Hollow";
const ACT_TABS = ["Weapons", "Spells", "Items", "Skills"] as const;

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
  await api("PATCH", "/api/weapons/darts", { name: LONG_WEAPON });
  await api("PATCH", "/api/items/power_gem_dispel", { name: LONG_ITEM });
  await api("PUT", "/api/spells/prepared", {
    spells: [
      "magic_missile",
      "burning_hands",
      "chromatic_orb",
      "tashas_uncontrollable_hideous_laughter",
      "mirror_image",
      "fireball",
      "haste",
      "stoneskin",
      "cloudkill",
    ],
  });
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
  await api("PATCH", "/api/weapons/darts", { name: LONG_WEAPON });
  await api("PATCH", "/api/items/power_gem_dispel", { name: LONG_ITEM });
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  const character = page.getByTestId("character");
  const stage = page.getByTestId("stage");
  // The rail's buttons in order (the names are French here): Act, Spells, XP, Inventory.
  const rail = page.getByTestId("rail").getByRole("button");
  const FR_SPELL =
    "Nom de sortilège invraisemblablement long des anciens archimages";
  for (const vp of VIEWPORTS) {
    await page.setViewportSize(vp);
    const where = `fr, ${vp.width}x${vp.height}`;
    await expectNoClippedText(character, `character card, ${where}`);
    await expectWholeWords(character, `character card, ${where}`);
    await rail.nth(0).click();
    await expectNoClippedText(stage, `act, ${where}`);
    await rail.nth(1).click();
    await page.getByRole("option", { name: FR_SPELL }).click();
    await expectNoClippedText(stage, `lookup, ${where}`);
    await expectWholeWords(stage, `lookup, ${where}`);
    await rail.nth(3).click();
    await expectNoClippedText(stage, `inventory, ${where}`);
    await expectWholeWords(stage, `inventory, ${where}`);
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
