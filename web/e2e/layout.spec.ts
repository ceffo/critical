import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { test, expect, snap, catalogueSpell, type Stack } from "./fixtures";
import {
  addBuff,
  ammoChips,
  attackForm,
  chooseLanguage,
  chooseSetting,
  chooseTheme,
  chooseWeapon,
  combatLog,
  createInFlow,
  nextRound,
  openDashboard,
  openDice,
  openSettings,
  openWeapons,
  openWorkspace,
  showWizardSpells,
  recordHp,
  apiCall,
  keepATangent,
  reloadSettled,
  openFold,
  openTypedAdd,
  expectUndoOn,
  logEntry,
  openSortsPowers,
  openSpend,
} from "./helpers";
import {
  test as base,
  type BrowserContext,
  type Locator,
  type Page,
} from "@playwright/test";
import { asMultiClass, openClassDetail } from "./multiclass";
import { psionicist } from "./psionicist";
import { keepPowerUp } from "./keepPowerUp";
import {
  brokenWords,
  clippedText,
  expectNoClippedText,
  expectTextFloor,
  expectWholeWords,
} from "./noClip";

import {
  VIEWPORTS,
  ACTIVITIES,
  SHOTS,
  LONG_EFFECT,
  LONG_SOURCE,
  LONG_REASON,
  LONG_SPELL,
  sizesAt,
  expectMinTarget,
  expectOneFocusRing,
  expectNoPageScroll,
  escapedAbsolutes,
  expectZones,
  expectInside,
  expectOnScreen,
  expectNoScroll,
  expectCompactJournal,
  expectCharacterCardLines,
  expectHeaderLayout,
  expectInitiativeBreakdown,
  lineCount,
  type Viewport,
} from "./layoutWalk";

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
async function shellStep(page: Page, size: Viewport) {
  await page.setViewportSize(size);
  await openDashboard(page);
  await expectZones(page);
  await expectNoPageScroll(page);

  // The header is as tall as its effects need, up to the turn row + 3 effect rows; the rest sits
  // behind the expander.
  const header = page.locator("[data-screen=round-header]");
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
  await nextRound(page);
  await recordHp(page, "damage", 3, LONG_SOURCE);
  await expect(
    combatLog(page)
      .getByRole("listitem")
      .filter({ hasText: LONG_SOURCE })
      .first(),
  ).toBeVisible();

  // The lookup with the long spell selected in a full list.
  await openWorkspace(page, "Spells");
  await showWizardSpells(page);
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
  for (const vp of sizesAt(size, VIEWPORTS)) {
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
      // throw without scrolling, and the weapon in hand is fully inside the act panel. What
      // follows the saves (the details, below) may scroll inside the card (ADR 0017 step 7).
      await expectInside(
        character.getByTestId("save-row").last(),
        character,
        "fifth save inside the character card",
      );
      expect(
        await character.evaluate((el) => el.scrollTop),
        "character card at rest",
      ).toBe(0);
      await expect(character.getByTestId("save-row")).toHaveCount(5);
      for (const [what, el] of [
        ["name", character.getByRole("heading", { level: 1 })],
        ["HP change", character.getByRole("group", { name: "HP change" })],
        ["fifth save", character.getByTestId("save-row").last()],
      ] as const)
        await expectInside(el, character, what);
      await openWorkspace(page, "Act");
      // ADR 0022 step 5.1, ADR 0027 step 9: an entry is one 24 px line; only a long one wraps.
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
}

async function itemPowerRowStep(page: Page, size: Viewport) {
  await page.setViewportSize(size);
  await openDashboard(page);
  await addBuff(page, {
    name: "e2e Strength",
    rounds: 6,
    effects: [
      ["str_bonus", 6],
      ["str_max", 18],
    ],
  });
  const header = page.locator("[data-screen=round-header]");
  const hammer = page.locator(
    '[data-testid="item-power"][data-item="power_gem_dispel"]',
  );
  for (const vp of sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 2560, height: 1440 },
  ])) {
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
}

/** Dismiss every active effect: a step that measures the save rows starts without their bonus tags. */
async function dismissEffects(stack: Stack) {
  const list = (await (await fetch(`${stack.apiUrl}/api/buffs`)).json()) as
    { id: string }[] | { buffs: { id: string }[] };
  for (const b of Array.isArray(list) ? list : list.buffs)
    await fetch(`${stack.apiUrl}/api/buffs/${b.id}`, { method: "DELETE" });
}

/** Delete every XP ledger entry: a step that counts the ledger's rows starts from an empty one. */
async function clearXp(stack: Stack) {
  const ledger = (await (await fetch(`${stack.apiUrl}/api/xp`)).json()) as {
    entries: { id: number }[];
  };
  for (const e of ledger.entries)
    await fetch(`${stack.apiUrl}/api/xp/${e.id}`, { method: "DELETE" });
}

async function savesLabelStep(page: Page, size: Viewport, stack: Stack) {
  await dismissEffects(stack);
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  const character = page.getByTestId("character");
  await page.getByTestId("save-mind-affecting").check();
  await expect(page.getByTestId("save-spell")).toBeVisible();
  await snap(page, "saves-spell-button-fr-1440x900");
  await page.getByTestId("save-spell").click();
  await page.getByTestId("save-spell-search").fill("comma");
  await page.getByTestId("save-spell-option").first().click();
  // The card's saves heading is checked from 1440 up (the 280 px card at 1280 wraps the French label).
  const wide = [
    { width: 1440, height: 900 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ];
  for (const vp of size.width < 1440 ? [] : sizesAt(size, wide)) {
    await page.setViewportSize(vp);
    const size = `${vp.width}x${vp.height}`;
    await expect(page.getByTestId("save-spell-tag")).toBeVisible();
    await expectCharacterCardLines(page, `a save spell set, ${size}`);
    await expectNoClippedText(
      character,
      `character card with a save spell, ${size}`,
    );
    await expectWholeWords(
      character,
      `character card with a save spell, ${size}`,
    );
    await snap(page, `saves-spell-tag-fr-${size}`);
  }
}

async function classLinesStep(
  page: Page,
  size: Viewport,
  mode: "multi" | "three" | "dual",
) {
  await asMultiClass(
    page,
    mode === "multi"
      ? {
          pending: true,
          longClass: "Warden of the Northern Marches and Sunken Citadels",
        }
      : mode === "three"
        ? { three: true, pending: true }
        : { dual: true },
  );
  await openDashboard(page);
  for (const lang of ["fr", "en"] as const) {
    await chooseLanguage(page, lang);
    await page.keyboard.press("Escape");
    const character = page.getByRole("complementary", {
      name: lang === "fr" ? "Personnage" : "Character",
    });
    const summary = character.getByTestId("class-summary");
    await expect(summary).toBeVisible();
    for (const vp of sizesAt(size, VIEWPORTS)) {
      await page.setViewportSize(vp);
      const where = `${mode}, ${lang}, ${vp.width}x${vp.height}`;
      // The classes are one line from 1440 wide; a long class name, or three French names in the
      // 1280 card, wrap between classes instead of being cut. The detail is behind the line.
      if (mode !== "multi")
        expect(
          (await summary.boundingBox())!.height,
          `class summary lines, ${where}`,
        ).toBeLessThanOrEqual(vp.width >= 1440 ? 34 : 50);
      await expect(character.getByTestId("class-entries")).toHaveCount(0);
      await expectNoClippedText(character, `class lines, ${where}`);
      await expectWholeWords(character, `class lines, ${where}`);
      await expectNoPageScroll(page);
      if (mode !== "dual")
        await expectNoClippedText(
          page.getByTestId("level-up").first(),
          `level-up prompt, ${where}`,
        );
      if (vp.width >= 1920 && lang === "fr")
        await snap(page, `${mode}-card-${vp.width}x${vp.height}`);
      // The detail, opened by keyboard: every class, uncut, inside the viewport.
      const detail = await openClassDetail(page);
      await expectNoClippedText(detail, `class detail, ${where}`);
      await expectWholeWords(detail, `class detail, ${where}`);
      const box = (await detail.boundingBox())!;
      expect(box.x, `class detail inside, ${where}`).toBeGreaterThanOrEqual(0);
      expect(
        box.x + box.width,
        `class detail inside, ${where}`,
      ).toBeLessThanOrEqual(vp.width);
      if (lang === "fr" || vp.width === 1280)
        await snap(page, `${mode}-detail-${lang}-${vp.width}x${vp.height}`);
      await page.keyboard.press("Escape");
      await expect(detail).toHaveCount(0);
    }
    // The Experience activity: the standings (and, for a multi-class, the split preview).
    await page
      .getByRole("navigation", {
        name: lang === "fr" ? "Activités" : "Activities",
      })
      .getByRole("button", {
        name: lang === "fr" ? "Expérience" : "Experience",
        exact: true,
      })
      .click();
    const xp = page.getByTestId("xp-top");
    if (mode === "multi") await xp.getByRole("textbox").first().fill("1001");
    for (const vp of sizesAt(size, VIEWPORTS)) {
      await page.setViewportSize(vp);
      const where = `${mode}, ${lang}, ${vp.width}x${vp.height}`;
      if (mode === "multi")
        await expect(xp.getByTestId("xp-split")).toBeVisible();
      await expectNoClippedText(xp, `experience classes, ${where}`);
      await expectNoClippedText(
        page.getByTestId("xp-ledger"),
        `ledger, ${where}`,
      );
      if (vp.width >= 1920 && lang === "fr")
        await snap(page, `${mode}-xp-${vp.width}x${vp.height}`);
    }
    await page
      .getByRole("navigation", {
        name: lang === "fr" ? "Activités" : "Activities",
      })
      .getByRole("button", {
        name: lang === "fr" ? "Agir" : "Act",
        exact: true,
      })
      .click();
  }
}

async function effectsTimelineStep(page: Page, size: Viewport) {
  await openDashboard(page);
  await openWorkspace(page, "Inventory");
  await page
    .getByTestId("item-leather")
    .getByRole("button", { name: /^Edit / })
    .click();
  const af = page.getByTestId("item-form");
  await af.getByRole("button", { name: "More magic" }).click();
  await af.getByRole("button", { name: "Save", exact: true }).click();
  await expect(af).toHaveCount(0);
  await openWorkspace(page, "Act");
  await chooseLanguage(page, "fr");
  for (const vp of sizesAt(size, [
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ])) {
    await page.setViewportSize(vp);
    await expect(page.getByTestId("effects-track")).toBeVisible();
    await expect(page.getByTestId("item-power")).toHaveCount(0);
    await snap(page, `timeline-magic-armour-fr-${vp.width}x${vp.height}`);
  }
}

async function settingsPopoverStep(page: Page, size: Viewport) {
  await page.route("**/api/auth/me", async (route) => {
    const res = await route.fetch();
    const body = (await res.json()) as Record<string, unknown>;
    body.displayName =
      "Aurélie de La Tour d'Auvergne-Lauragais de Saint-Aignan-sur-Cher";
    await route.fulfill({ response: res, json: body });
  });
  await openDashboard(page);
  for (const vp of sizesAt(size, VIEWPORTS.slice(3))) {
    await page.setViewportSize(vp);
    const popover = await openSettings(page);
    await expect(popover).toContainText("Aurélie de La Tour");
    await expectNoClippedText(popover, `settings popover name ${vp.width}`);
    await expectWholeWords(popover, `settings popover name ${vp.width}`);
    await snap(page, `settings-popover-name-${vp.width}x${vp.height}`);
    await page.keyboard.press("Escape");
  }
}

async function themesStep(page: Page) {
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
    await showWizardSpells(page);
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
    // One column, no paragraph in a right half (ADR 0022 step 2.5; the matrix is in the
    // description spec below).
    expect(
      await text.evaluate((el) => {
        const mid = el.getBoundingClientRect().left + el.clientWidth / 2;
        return [...el.querySelectorAll("p")].some(
          (p) => p.getBoundingClientRect().left > mid,
        );
      }),
    ).toBe(false);
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
}

async function actPanelStep(page: Page, size: Viewport, stack: Stack) {
  const api = (method: string, url: string, data: unknown) =>
    fetch(`${stack.apiUrl}${url}`, {
      method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    }).then((r) => expect(r.ok, `${method} ${url}`).toBe(true));
  await api("PATCH", "/api/items/darts", {
    name: { en: LONG_WEAPON, fr: LONG_WEAPON },
  });
  await api("PATCH", "/api/items/power_gem_dispel", {
    name: { en: LONG_ITEM, fr: LONG_ITEM },
  });
  // Emptying the list first gives fresh slots: an earlier step may have spent Rary's.
  await api("PUT", "/api/spells/prepared", { spells: [] });
  await api("PUT", "/api/spells/prepared", { spells: PREPARED });
  await openDashboard(page);
  await openWeapons(page);
  await chooseWeapon(page, "Long Sword +2");
  await chooseWeapon(page, "Short Bow +2");
  await expect(ammoChips(page)).toBeVisible();
  const actions = page.getByRole("region", { name: "Actions" });
  const panel = actions.getByRole("tabpanel");
  for (const vp of sizesAt(size, VIEWPORTS)) {
    await page.setViewportSize(vp);
    const size = `${vp.width}x${vp.height}`;
    let journalTop: number | null = null;
    for (const tab of ACT_TABS) {
      await actions.getByRole("tab", { name: tab }).click();
      await expect(panel).toHaveAttribute("id", `panel-${tab.toLowerCase()}`);
      await expect(panel).not.toBeEmpty();
      if (tab === "Spells")
        await expect(panel).toContainText("Hideous Laughter");
      await expectNoPageScroll(page);
      // ADR 0027 step 6.1: the journal's top edge is the same on every tab.
      const top = (await page.getByTestId("journal-slot").boundingBox())!.y;
      if (journalTop === null) journalTop = top;
      expect(top, `journal top on ${tab}, ${size}`).toBeCloseTo(journalTop, 0);
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
  }
  // The long weapon in hand wraps inside its row at the narrowest viewport.
  await page.setViewportSize(size);
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
}

async function frenchLongNamesStep(page: Page, size: Viewport, stack: Stack) {
  const api = (method: string, url: string, data: unknown) =>
    fetch(`${stack.apiUrl}${url}`, {
      method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    }).then((r) => expect(r.ok, `${method} ${url}`).toBe(true));
  // The long weapon and item names were set by the act panel step, which runs before this one.
  // The Spells tab with the long Tasha's name, a used slot and blank extra slots (ADR 0027 step 10).
  // Emptying the list first gives fresh slots: an earlier step may have spent Rary's.
  await api("PUT", "/api/spells/prepared", { spells: [] });
  await api("PUT", "/api/spells/prepared", { spells: PREPARED });
  const slots = (await (
    await fetch(`${stack.apiUrl}/api/spells/slots`)
  ).json()) as { prepared_spell_id: string }[];
  const rary = slots.findIndex(
    (sl) => sl.prepared_spell_id === "rarys_mnemonic_enhancer",
  );
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
  // Rary's takes a turn to cast: in combat it corrects the round's initiative, which then reads
  // "fin du tour" beside its number (ADR 0035 item 6.2).
  await api("POST", `/api/spells/${rary}/cast`, {});
  await api("POST", "/api/roll/dice", { formula: "2d6+1" });
  await api("POST", "/api/hp/event", { delta: 2, source: "Potion de soins" });
  const log = (await (await fetch(`${stack.apiUrl}/api/log`)).json()) as {
    topId: number;
  };
  await api("POST", "/api/log/undo", { expectedTopId: log.topId });
  // Two XP entries, one with a long reason and a session (ADR 0027 step 14.4): the ledger starts empty
  // here, whatever the earlier steps of the walk added.
  await clearXp(stack);
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
  for (const vp of sizesAt(size, VIEWPORTS)) {
    await page.setViewportSize(vp);
    const where = `fr, ${vp.width}x${vp.height}`;
    // The whole screen (header, rail, journal, dice footer and the shared pieces in every zone).
    await expectNoClippedText(page.locator("body"), `screen, ${where}`);
    await rail.nth(0).click();
    await expectHeaderLayout(page, where);
    await expectInitiativeBreakdown(page, where);
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
        bar.getByRole("button", { name: "CA inconnue" }),
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
      // ADR 0027 step 6: the Weapons tab does not scroll, and the journal (at least 14rem) takes
      // the rest.
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
    // A level 15 bard's budget (20 + 15 × 14 = 230) and its editor wrap uncut in French.
    await openSpend(page, "skillPoints");
    const pointsEditor = page.getByTestId("thief-points-editor");
    await expect(
      pointsEditor.getByTestId("skill-points-available"),
    ).toContainText("Disponibles : 20 + 15 × 14 = 230 (niveau 15)");
    await expectNoClippedText(
      pointsEditor.getByTestId("skill-points-budget"),
      `bard budget, ${where}`,
    );
    await expectNoClippedText(pointsEditor, `bard points editor, ${where}`);
    await expectWholeWords(pointsEditor, `bard points editor, ${where}`);
    await pointsEditor.getByTestId("thief-points-cancel").click();
    // ADR 0037 step 12: the proficiency editor in place of the lists, Kelsen's eight picks with an
    // extra slot on Tumbling (the server's figures), the catalogue split by the bard's groups.
    await openSpend(page, "nonWeapon");
    const picksEditor = page.getByTestId("proficiency-editor");
    await picksEditor
      .getByTestId("proficiency-pick-tumbling")
      .getByTestId("proficiency-more")
      .click();
    await expect(picksEditor.getByTestId("proficiency-pool-spent")).toHaveText(
      "Dépensés 9, restants 1",
    );
    await expect(
      picksEditor
        .getByTestId("proficiency-pick-tumbling")
        .getByTestId("proficiency-pick-terms"),
    ).toContainText("+1 au jet");
    await expectNoClippedText(picksEditor, `proficiency editor, ${where}`);
    await expectWholeWords(picksEditor, `proficiency editor, ${where}`);
    await expectNoPageScroll(page);
    if (SHOTS.has(`${vp.width}x${vp.height}`) || vp.width >= 1920)
      await snap(page, `act-proficiencies-fr-${vp.width}x${vp.height}`);
    if (vp.width === 1920) {
      // The editor in the dark (the appearance follows the system): for review.
      await page.emulateMedia({ colorScheme: "dark" });
      await expect(page.locator("html")).toHaveClass(/dark/);
      await snap(page, "act-proficiencies-fr-dark-1920x1080");
      await page.emulateMedia({ colorScheme: "light" });
    }
    await picksEditor.getByTestId("proficiency-cancel").click();
    await skills.evaluate((el) => el.scrollTo(0, 0));
    if (vp.width === 1440) {
      // The bard's budget and terms (ADR 0027 step 12.3) make the tab scroll inside
      // its panel; the page never scrolls.
      await expectNoPageScroll(page);
      // The bard's block holds its skills, the armour and the Spend button (ADR 0030 step 18).
      const [proficiencies, bard] = await skills
        .getByTestId("skills-blocks")
        .locator(":scope > *")
        .all();
      // The bard's block scrolls in the panel (its top is inside), the proficiencies fit.
      await expectInside(proficiencies, skills, `proficiencies, ${where}`);
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
    await showWizardSpells(page);
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
      // ADR 0027 steps 13.1 and 1.5: the school behind "École", a tag on the count line when set.
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
    // ADR 0027 step 14.1: every row's edit and delete are visible at rest (no hover), at least
    // 24 px, named for their item, and inside the stage's width (the list scrolls vertically).
    await page.mouse.move(0, 0);
    const rowButtons = stage.getByRole("button", {
      name: /^(Modifier|Retirer) /,
    });
    expect(await rowButtons.count(), `row actions, ${where}`).toBeGreaterThan(
      1,
    );
    for (const b of await rowButtons.all()) {
      await expect(b).toBeVisible();
      await expect(b.locator("xpath=..")).toHaveCSS("opacity", "1");
      const box = (await b.boundingBox())!;
      expect(
        Math.min(box.width, box.height),
        `${where}`,
      ).toBeGreaterThanOrEqual(24);
      const st = (await stage.boundingBox())!;
      expect(box.x, `row action left, ${where}`).toBeGreaterThanOrEqual(st.x);
      expect(
        box.x + box.width,
        `row action right, ${where}`,
      ).toBeLessThanOrEqual(st.x + st.width);
    }
    // ADR 0027 step 14.3 and ADR 0035 item 2: one possessions card, inside the stage (the page never
    // scrolls; a long list scrolls inside the card).
    await expectInside(
      stage.getByRole("region", { name: "Objets" }),
      stage,
      `possessions card, ${where}`,
    );
    // The row actions at rest, at the three sizes of the design review.
    await snap(page, `inventory-long-fr-${vp.width}x${vp.height}`);
  }
}

async function saveCardStep(page: Page, size: Viewport) {
  await page.setViewportSize(size);
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
    for (const vp of sizesAt(size, [
      { width: 1280, height: 800 },
      { width: 2560, height: 1440 },
    ])) {
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
}

async function abilityCardStep(page: Page, size: Viewport) {
  await page.setViewportSize(size);
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  await page.getByRole("button", { name: "Jet de Dextérité (17)" }).click();
  const card = page.getByTestId("ability-result");
  await expect(card).toBeVisible();
  const form = await openFold(card, "Ajuster après le jet", "Ajuster ce jet");
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
    for (const vp of sizesAt(size, [
      { width: 1280, height: 800 },
      { width: 2560, height: 1440 },
    ])) {
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
}

async function attackCardStep(page: Page, size: Viewport, stack: Stack) {
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
    for (const vp of sizesAt(size, VIEWPORTS)) {
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
}

async function lastResultStep(page: Page, size: Viewport, stack: Stack) {
  const api = (method: string, url: string, data: unknown) =>
    fetch(`${stack.apiUrl}${url}`, {
      method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    }).then((r) => expect(r.ok, `${method} ${url}`).toBe(true));
  // Emptying the list first gives fresh slots: an earlier step may have spent Rary's.
  await api("PUT", "/api/spells/prepared", { spells: [] });
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
  for (const vp of sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 1920, height: 1080 },
  ])) {
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
}

async function groupHeadingsStep(page: Page, _size: Viewport, stack: Stack) {
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
}

async function spellsTabStep(page: Page, size: Viewport, stack: Stack) {
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
  for (const vp of sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 1920, height: 1080 },
  ])) {
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
    // The band keeps one height (ADR 0027 step 6.1): when the editor is taller than the panel it
    // scrolls inside it (the stacked columns at 1280x800); otherwise both columns end at the panel's
    // content bottom (its 14 px padding).
    const scrolls = await panel.evaluate(
      (el) => el.scrollHeight - el.clientHeight,
    );
    if (scrolls > 1) {
      expect(
        await panel.evaluate((el) => getComputedStyle(el).overflowY),
        `act panel scrolls inside, ${size}`,
      ).toBe("auto");
    } else {
      const inner = (await panel.boundingBox())!;
      for (const name of ["Préparation du jour", "Grimoire"]) {
        const col = (await form.getByRole("region", { name }).boundingBox())!;
        expect(
          Math.abs(inner.y + inner.height - 14 - (col.y + col.height)),
          `${name} bottom, ${size}`,
        ).toBeLessThanOrEqual(2);
      }
    }
    await actions.getByRole("button", { name: "Annuler" }).click();
  }
}

/**
 * ADR 0027 step 11.7: with a long spellbook (every wizard spell of the test catalogue, 31 rows at the levels the mage reaches) the preparation
 * editor fills the act panel and only the grimoire column scrolls; the panel and the page do not.
 */
async function prepareLongBookStep(page: Page, size: Viewport, stack: Stack) {
  const catalogue = JSON.parse(
    readFileSync(
      fileURLToPath(
        new URL("../../api/testdata/catalogue.json", import.meta.url),
      ),
      "utf8",
    ),
  ) as { spells: { id: string; class: string }[] };
  await openDashboard(page);
  expect(
    await apiCall(page, "POST", "/api/characters", {
      name: "Zéphyrin le Collectionneur",
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
  ).toBe(true);
  expect(
    await apiCall(page, "POST", "/api/xp", { base: 400000, bonus: false }),
  ).toBe(true);
  // The slots follow the levels rolled: roll every owed hit die (a level each) before the preparation.
  await reloadSettled(page);
  const roll = page.getByTestId("level-up-roll");
  await expect(roll).toBeVisible();
  while (await roll.isVisible()) {
    await roll.click();
    await page.waitForTimeout(200);
  }
  const known = (await (
    await fetch(`${stack.apiUrl}/api/spells/preparation`)
  ).json()) as { spellbook: { id: string }[] };
  const have = new Set(known.spellbook.map((s) => s.id));
  for (const { id } of catalogue.spells.filter((s) => s.class === "Wizard")) {
    if (have.has(id)) continue;
    expect(
      await apiCall(page, "POST", "/api/character/spellbook/add", {
        spellId: id,
      }),
      `add ${id}`,
    ).toBe(true);
  }
  await reloadSettled(page);
  const actions = page.getByRole("region", { name: "Actions" });
  const panel = actions.getByRole("tabpanel");
  await actions.getByRole("tab", { name: "Spells" }).click();
  for (const vp of sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ])) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await actions.getByRole("button", { name: "Prepare spells" }).click();
    const form = actions.getByRole("form", { name: "Prepare today's spells" });
    const book = form.getByRole("region", { name: "Spellbook" });
    await expect(book.locator("li")).not.toHaveCount(0);
    expect(
      await book.locator("li").count(),
      `a long book, ${where}`,
    ).toBeGreaterThanOrEqual(30);
    await expectNoPageScroll(page);
    // The panel does not scroll; the grimoire list does, and ends at the panel's content bottom.
    expect(
      await panel.evaluate((el) => el.scrollHeight - el.clientHeight),
      `act panel scroll, ${where}`,
    ).toBeLessThanOrEqual(1);
    const list = book.locator("div.overflow-y-auto");
    const m = await list.evaluate((el) => ({
      sh: el.scrollHeight,
      ch: el.clientHeight,
    }));
    expect(m.sh, `the spellbook column scrolls, ${where}`).toBeGreaterThan(
      m.ch,
    );
    const inner = (await panel.boundingBox())!;
    const col = (await book.boundingBox())!;
    expect(
      Math.abs(inner.y + inner.height - 14 - (col.y + col.height)),
      `spellbook column bottom, ${where}`,
    ).toBeLessThanOrEqual(2);
    await snap(page, `prepare-long-book-${where}`);
    await actions.getByRole("button", { name: "Cancel" }).click();
  }
}

async function skillsEditorStep(page: Page, size: Viewport) {
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  await page.getByRole("tab", { name: "Compétences" }).click();
  const panel = await openSpend(page, "nonWeapon");
  const pool = panel.getByTestId("proficiency-languages");
  // Test Bard spends 8 of 10 slots: three languages are one too many.
  for (const name of ["Elfe", "Gnome", "Orque"])
    await pool.getByRole("checkbox", { name, exact: true }).check();
  await expect(pool.getByRole("alert")).toContainText("Budget dépassé");
  for (const vp of sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ])) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expectNoClippedText(pool, `skills editor languages, ${where}`);
    await expectWholeWords(pool, `skills editor languages, ${where}`);
    await snap(page, `skills-languages-edit-fr-${where}`);
  }
  await pool.getByRole("checkbox", { name: "Orque", exact: true }).uncheck();
  await expect(pool.getByTestId("proficiency-languages-slots")).toHaveText(
    "2 emplacements de compétence",
  );
  await panel.getByTestId("proficiency-save").click();
  await expect(page.getByTestId("proficiency-editor")).toHaveCount(0);
  await page.getByTestId("details-trigger").click();
  const pop = page.getByTestId("details-popover");
  await expect(pop.getByTestId("details-language-slots")).toHaveText(
    "2 emplacements de compétence",
  );
  for (const vp of sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ])) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expectNoClippedText(pop, `details card languages view, ${where}`);
    await snap(page, `details-languages-view-fr-${where}`);
  }
}

// The card's lower half shows the details and languages the popover also shows, inline, set ones
// only (ADR 0017 step 7 for the fit: it fits at 1920x1080 and up, and scrolls inside the card below).
// The popover hides the fields not set. French first, the no-cut audit on the card, at every size.
async function cardDetailsStep(page: Page, size: Viewport, stack: Stack) {
  await dismissEffects(stack);
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  const card = page.getByTestId("character");
  await page.getByTestId("details-trigger").click();
  const pop = page.getByTestId("details-popover");
  // The popover lists what is set only, with the one link to the rest.
  await expect(pop.getByText("Non renseigné")).toHaveCount(0);
  await expect(pop.getByTestId("details-set")).toHaveCount(1);
  await pop.getByTestId("details-set").click();
  await pop.getByLabel("Cheveux", { exact: true }).fill("Châtain");
  await pop.getByLabel("Âge", { exact: true }).fill("112");
  await pop.getByRole("button", { name: "Enregistrer" }).click();
  await expect(pop.getByRole("button", { name: "Modifier" })).toBeVisible();
  await page.keyboard.press("Escape");
  const lower = page.getByTestId("card-details");
  await expect(lower.getByText("Châtain")).toBeVisible();
  await expect(lower.getByText("112")).toBeVisible();
  // The languages the skills editor step left.
  await expect(lower.getByText(/Commun, Elfe, Gnome/)).toBeVisible();
  await expect(lower.getByText("Non renseigné")).toHaveCount(0);
  for (const vp of sizesAt(size, VIEWPORTS)) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expect(lower).toBeVisible();
    await expectNoClippedText(card, `character card lower half, ${where}`);
    await expectWholeWords(card, `character card lower half, ${where}`);
    if (vp.height >= 1080) {
      await expectNoScroll(card, `character card, ${where}`);
      await expectNoScroll(lower, `card details, ${where}`);
    }
    await snap(page, `card-details-fr-${where}`);
  }
}

// The dice bar (ADR 0027 step 5.7): one button by default, expanded by it (focus into the formula),
// collapsed by Escape and by a roll. Expanded, it covers no control and cuts no text at the layout
// sizes, in French and English; the page does not scroll.
async function diceBarStep(page: Page, size: Viewport, stack: Stack) {
  await dismissEffects(stack);
  await openDashboard(page);
  const footer = page.getByTestId("dice-footer");
  const toggle = page.getByTestId("dice-toggle");
  const formula = footer.getByLabel(/^(Formula|Formule)$/);
  for (const lang of ["fr", "en"] as const) {
    await chooseLanguage(page, lang);
    for (const vp of sizesAt(size, VIEWPORTS)) {
      await page.setViewportSize(vp);
      const where = `dice bar, ${lang}, ${vp.width}x${vp.height}`;
      await expect(toggle, where).toHaveAttribute("aria-expanded", "false");
      await expect(formula, where).toHaveCount(0);
      await expectNoPageScroll(page);
      await expectNoClippedText(footer, `${where}, collapsed`);
      await toggle.click();
      await expect(formula, where).toBeFocused();
      await expectNoPageScroll(page);
      await expectNoClippedText(footer, `${where}, expanded`);
      // Nothing in the bar is cut by the footer's box or overlaps a neighbour.
      const box = (await footer.boundingBox())!;
      const controls = footer.locator("button:visible, input:visible");
      const n = await controls.count();
      let prevRight = -1;
      for (let i = 0; i < n; i++) {
        const b = (await controls.nth(i).boundingBox())!;
        // The favourites strip scrolls sideways; its chips may reach past the footer.
        if (
          await controls.nth(i).evaluate((e) => !!e.closest("[data-scroll-x]"))
        )
          continue;
        expect(
          b.x,
          `${where}: control ${i} inside, left`,
        ).toBeGreaterThanOrEqual(box.x);
        expect(
          b.x + b.width,
          `${where}: control ${i} inside, right`,
        ).toBeLessThanOrEqual(box.x + box.width + 0.5);
        expect(
          b.x,
          `${where}: control ${i} clear of the last`,
        ).toBeGreaterThanOrEqual(prevRight - 0.5);
        prevRight = b.x + b.width;
      }
      if (lang === "fr" && vp.width >= 1920)
        await snap(page, `dice-bar-open-fr-${vp.width}x${vp.height}`);
      await page.keyboard.press("Escape");
      await expect(formula, where).toHaveCount(0);
      await expect(toggle, where).toBeFocused();
    }
  }
  // A roll collapses it again.
  await chooseLanguage(page, "en");
  await toggle.click();
  await footer.getByTestId("die-6").click();
  await footer.getByRole("button", { name: "Roll", exact: true }).click();
  await expect(formula).toHaveCount(0);
  await expect(toggle).toHaveAttribute("aria-expanded", "false");
}

// The typed add form is two steps and the edit opens over the list: no row moves, and nothing is
// cut in French or English (checked in French first, ADR 0027 step 2).
async function itemFormStep(page: Page, size: Viewport) {
  await openDashboard(page);
  await openWorkspace(page, "Inventory");
  const rows = page.locator("li[data-category]");
  const positions = async () =>
    (await rows.evaluateAll((els) =>
      els.map((e) => {
        const r = e.getBoundingClientRect();
        return [e.getAttribute("data-testid"), r.x, r.y, r.height];
      }),
    )) as unknown[];
  for (const lang of ["fr", "en"] as const) {
    await chooseLanguage(page, lang);
    const fr = lang === "fr";
    await openTypedAdd(page, lang);
    const f = page.getByTestId("item-form");
    await f
      .getByLabel(fr ? "Nom" : "Name")
      .fill("Hache de bataille des longs hivers");
    await f.getByLabel(fr ? "Catégorie" : "Category").selectOption("weapon");
    await f.getByRole("button", { name: fr ? "Suivant" : "Next" }).click();
    await expect(f.getByTestId("item-summary")).toBeVisible();
    for (const vp of sizesAt(size, VIEWPORTS)) {
      await page.setViewportSize(vp);
      await expectNoPageScroll(page);
      await expectNoClippedText(
        f,
        `add form step 2, ${lang}, ${vp.width}x${vp.height}`,
      );
      if (fr && vp.width >= 1920)
        await snap(page, `item-form-step2-fr-${vp.width}x${vp.height}`);
    }
    // The extras (how it is used, the dice rolled and the save against the effect) uncut too.
    await f.getByText(fr ? "Plus d'options" : "More options").click();
    await expect(
      f.getByLabel(fr ? "Dés lancés à l'usage" : "Dice rolled on use"),
    ).toBeVisible();
    for (const vp of sizesAt(size, VIEWPORTS)) {
      await page.setViewportSize(vp);
      await expectNoClippedText(
        f,
        `add form extras, ${lang}, ${vp.width}x${vp.height}`,
      );
      if (fr && vp.width >= 1920)
        await snap(page, `item-form-extras-fr-${vp.width}x${vp.height}`);
    }
    await f.getByRole("button", { name: fr ? "Retour" : "Back" }).click();
    await expect(f.getByLabel(fr ? "Nom" : "Name")).toHaveValue(
      "Hache de bataille des longs hivers",
    );
    await f.getByRole("button", { name: fr ? "Annuler" : "Cancel" }).click();
    await expect(f).toHaveCount(0);

    await page.setViewportSize(size);
    const before = await positions();
    expect(before.length).toBeGreaterThan(3);
    await rows
      .nth(1)
      .getByRole("button", { name: fr ? /^Modifier / : /^Edit / })
      .click();
    await expect(f).toBeVisible();
    for (const vp of sizesAt(size, VIEWPORTS)) {
      await page.setViewportSize(vp);
      await expectNoClippedText(
        f,
        `edit dialog, ${lang}, ${vp.width}x${vp.height}`,
      );
      if (fr && vp.width >= 1920)
        await snap(page, `item-edit-dialog-fr-${vp.width}x${vp.height}`);
    }
    await page.keyboard.press("Escape");
    await expect(f).toHaveCount(0);
    await page.setViewportSize(size);
    const after = await positions();
    expect(after, `no row moved after the edit, ${lang}`).toEqual(before);
  }
}

async function pieceSpellStep(page: Page, size: Viewport) {
  await openDashboard(page);
  await openWorkspace(page, "Inventory");
  await chooseLanguage(page, "fr");
  await openTypedAdd(page, "fr");
  const wf = page.getByTestId("item-form");
  await wf.getByRole("button", { name: "Suivant" }).click();
  await wf.getByText("Plus d'options").click();
  await wf.getByLabel("Composante de sort").selectOption("gem");
  for (const vp of sizesAt(size, VIEWPORTS)) {
    await page.setViewportSize(vp);
    await expectNoPageScroll(page);
    await expectNoClippedText(wf, `piece editor, fr, ${vp.width}x${vp.height}`);
    if (vp.width >= 1920)
      await snap(page, `component-editor-fr-${vp.width}x${vp.height}`);
  }
}

async function piecePowersStep(page: Page, size: Viewport) {
  await openDashboard(page);
  await openWorkspace(page, "Inventory");
  for (const lang of ["fr", "en"] as const) {
    await chooseLanguage(page, lang);
    await openTypedAdd(page, lang);
    const wf = page.getByTestId("item-form");
    await wf
      .getByLabel(lang === "fr" ? "Catégorie" : "Category")
      .selectOption("magic");
    await wf
      .getByRole("button", { name: lang === "fr" ? "Suivant" : "Next" })
      .click();
    await wf.getByTestId("power-templates").click();
    await page.getByTestId("power-template-ring_of_protection").click();
    await wf.getByTestId("power-add").click();
    const second = wf.getByTestId("power-row").nth(1);
    await second.getByTestId("power-min-str").fill("18");
    await second.getByRole("combobox").last().selectOption("melee");
    await second.getByTestId("power-without-magic-armour").check();
    await second
      .getByRole("button", {
        name: lang === "fr" ? "Effet" : "Effect",
        exact: true,
      })
      .click();
    for (const vp of sizesAt(size, VIEWPORTS)) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      await expectNoPageScroll(page);
      await expectNoClippedText(wf, `powers editor, ${where}`);
      if (lang === "fr" && vp.width >= 1920)
        await snap(page, `powers-editor-fr-${vp.width}x${vp.height}`);
    }
    await wf
      .getByRole("button", { name: lang === "fr" ? "Annuler" : "Cancel" })
      .click();
    await page
      .getByTestId("item-leather")
      .getByRole("button", { name: lang === "fr" ? /^Modifier / : /^Edit / })
      .click();
    const af = page.getByTestId("item-form");
    await af
      .getByRole("button", {
        name: lang === "fr" ? "Plus de magie" : "More magic",
      })
      .click();
    await af.getByTestId("power-add").click();
    for (const vp of sizesAt(size, [
      VIEWPORTS[0],
      VIEWPORTS[3],
      VIEWPORTS[4],
    ])) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      await expectNoPageScroll(page);
      await expectNoClippedText(af, `armour editor +N, ${where}`);
      await af.getByTestId("item-powers").scrollIntoViewIfNeeded();
      if (lang === "fr" && vp.width >= 1920)
        await snap(page, `armour-plus-editor-fr-${vp.width}x${vp.height}`);
    }
    await af
      .getByRole("button", { name: lang === "fr" ? "Annuler" : "Cancel" })
      .click();
  }
}

async function ajouterStep(page: Page, size: Viewport) {
  await openDashboard(page);
  await openWorkspace(page, "Inventory");
  const names = {
    fr: ["Armes", "Munitions", "Armures", "Boucliers"],
    en: ["Weapons", "Ammunition", "Armour", "Shields"],
  } as const;
  for (const lang of ["fr", "en"] as const) {
    await chooseLanguage(page, lang);
    await page
      .getByRole("button", {
        name: lang === "fr" ? "Ajouter un objet" : "Add item",
      })
      .click();
    const add = page.getByTestId("add-piece");
    for (const category of names[lang]) {
      await add.getByRole("button", { name: category, exact: true }).click();
      await add.getByTestId("add-rows").getByRole("button").first().click();
      await expect(add.getByTestId("add-submit")).toBeVisible();
      for (const vp of sizesAt(size, VIEWPORTS)) {
        await page.setViewportSize(vp);
        const where = `${category}, ${lang}, ${vp.width}x${vp.height}`;
        await expectNoPageScroll(page);
        await expectNoClippedText(add, `add from the catalogue, ${where}`);
        if (
          vp.width >= 1920 &&
          (category === "Armures" ||
            category === "Armes" ||
            category === "Weapons" ||
            category === "Armour")
        )
          await snap(
            page,
            `add-catalogue-${category.toLowerCase()}-${lang}-${vp.width}x${vp.height}`,
          );
      }
    }
    await add
      .getByRole("tab", { name: lang === "fr" ? "Saisir" : "Type in" })
      .click();
    for (const vp of sizesAt(size, [VIEWPORTS[0], VIEWPORTS[3]])) {
      await page.setViewportSize(vp);
      await expectNoPageScroll(page);
      await expectNoClippedText(
        add,
        `add typed, ${lang}, ${vp.width}x${vp.height}`,
      );
      if (lang === "fr" && vp.width === 1920)
        await snap(page, `add-typed-fr-${vp.width}x${vp.height}`);
    }
    await add
      .getByRole("button", { name: lang === "fr" ? "Fermer" : "Close" })
      .click();
    await expect(add).toHaveCount(0);
  }
}

async function acShieldStep(page: Page, size: Viewport) {
  // Like the other viewport sweeps: five viewports in two languages and four states take about
  // 20 s unloaded, and a starved runner needs more than the 30 s default.
  await openDashboard(page);
  await addBuff(page, {
    name: "Protection from Evil",
    effects: [["ac", -2]],
  });
  await openWorkspace(page, "Inventory");
  await openTypedAdd(page);
  const f = page.getByTestId("item-form");
  await f.getByLabel("Name").fill("Chain mail of the long winter watch");
  await f.getByLabel("Category").selectOption("armour");
  await f.getByRole("button", { name: "Next" }).click();
  await f.getByLabel("Base AC").fill("5");
  await f.getByRole("button", { name: "Add", exact: true }).click();
  await page
    .getByRole("button", { name: "Equip Chain mail of the long winter watch" })
    .click();
  // A body shield: +1 against melee, +2 against missiles (ADR 0033 step 10.3).
  await openTypedAdd(page);
  await f.getByLabel("Name").fill("Body shield");
  await f.getByLabel("Category").selectOption("shield");
  await f.getByRole("button", { name: "Next" }).click();
  await f.getByLabel("Shield kind").selectOption("body");
  await f.getByLabel("AC bonus (melee)").fill("1");
  await f.getByLabel("AC bonus (missiles)").fill("2");
  await f.getByRole("button", { name: "Add", exact: true }).click();
  await page.getByRole("button", { name: "Equip Body shield" }).click();
  const card = page.getByTestId("ac-card");
  // 5, shield -1, Dexterity -3, Protection from Evil -2; against missiles -2.
  await expect(card.getByTestId("ac-card-value")).toHaveText("−1");
  await expect(
    page.getByTestId("ac-shield").getByTestId("ac-value"),
  ).toHaveText("−1");
  await expect(card.getByTestId("ac-breakdown")).toContainText(
    "−2 vs missiles",
  );

  const character = page.getByTestId("character");
  const shield = page.getByTestId("ac-shield");
  const pop = page.getByTestId("ac-popover");
  const check = async (
    state: string,
    langs: readonly ("fr" | "en")[] = ["fr", "en"],
  ) => {
    for (const lang of langs) {
      await chooseLanguage(page, lang);
      for (const vp of sizesAt(size, VIEWPORTS)) {
        const where = `${state}, ${lang}, ${vp.width}x${vp.height}`;
        await page.setViewportSize(vp);
        await expectNoPageScroll(page);
        await expectInside(shield, character, `AC shield, ${where}`);
        const size = async (l: Locator) =>
          l.evaluate((el) => parseFloat(getComputedStyle(el).fontSize));
        expect(
          await size(shield.getByTestId("ac-value")),
          `the AC is smaller than the HP, ${where}`,
        ).toBeLessThan(
          await size(page.getByTestId("hp-value").locator("span").first()),
        );
        await expectNoClippedText(card, `AC card, ${where}`);
        await shield.click();
        await expect(pop).toBeVisible();
        await expectOnScreen(page, pop, `AC popover, ${where}`);
        await expectNoClippedText(pop, `AC popover, ${where}`);
        if (vp.width >= 1920)
          await snap(page, `ac-${state}-${lang}-${vp.width}x${vp.height}`);
        await page.keyboard.press("Escape");
        await expect(pop).toHaveCount(0);
      }
    }
  };
  await check("derived");

  await card.getByTestId("ac-card-override").click();
  await card.getByTestId("ac-override-value").fill("4");
  await card
    .getByTestId("ac-override-label")
    .fill("Shield spell against hurled missiles");
  await card.getByTestId("ac-override-save").click();
  await expect(card.getByTestId("ac-override")).toBeVisible();
  await expect(shield).toHaveAttribute("data-overridden", "true");
  await check("override");

  // The dark theme, French first.
  await chooseSetting(page, "Dark");
  await check("override-dark", ["fr"]);
  await card.getByTestId("ac-override-clear").click();
  await expect(card.getByTestId("ac-override")).toHaveCount(0);
  await check("derived-dark", ["fr"]);

  // The widest numbers the shield holds: -10 and 10.
  for (const value of ["-10", "10"]) {
    await card.getByTestId("ac-card-override").click();
    await card.getByTestId("ac-override-value").fill(value);
    await card.getByTestId("ac-override-save").click();
    await expect(shield.getByTestId("ac-value")).toHaveText(
      value.replace("-", "−"),
    );
    for (const vp of sizesAt(size, [
      VIEWPORTS[0],
      VIEWPORTS[VIEWPORTS.length - 1],
    ])) {
      await page.setViewportSize(vp);
      await expectNoClippedText(
        shield,
        `AC shield ${value}, ${vp.width}x${vp.height}`,
      );
      const box = (await shield.getByTestId("ac-value").boundingBox())!;
      const outer = (await shield.boundingBox())!;
      expect(box.width, `the number fits the shield, ${value}`).toBeLessThan(
        outer.width - 16,
      );
      if (value === "-10")
        await snap(page, `ac-minus10-fr-dark-${vp.width}x${vp.height}`);
    }
    await card.getByTestId("ac-override-clear").click();
    await expect(card.getByTestId("ac-override")).toHaveCount(0);
  }
}

async function scrollSpellStep(page: Page, size: Viewport) {
  const FR_SPELL =
    "Nom de sortilège invraisemblablement long des anciens archimages";
  await page.setViewportSize(size);
  await openDashboard(page);
  const call = (method: string, url: string, body: unknown) =>
    apiCall(page, method, url, body);
  expect(
    await call("POST", "/api/items", {
      name: {
        en: "Scroll of the ancient archmages",
        fr: "Parchemin des anciens archimages",
      },
      category: "magic",
      magicKind: "scroll",
      spellId: "long_spell",
      behaviour: "stock",
      count: 2,
    }),
  ).toBe(true);
  expect(await call("POST", "/api/round/advance", {})).toBe(true);
  expect(
    await call("POST", "/api/items/scroll_of_the_ancient_archmages/use", {}),
  ).toBe(true);
  await reloadSettled(page);
  await chooseLanguage(page, "fr");
  // The rail's buttons in order (French names): Act, Spells, XP, Inventory.
  const rail = page.getByTestId("rail").getByRole("button");
  for (const vp of sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ])) {
    await page.setViewportSize(vp);
    const where = `fr, ${vp.width}x${vp.height}`;
    // The initiative breakdown's term names the scroll and its spell.
    await page.getByTestId("initiative-value").hover();
    const breakdown = page.getByTestId("initiative-breakdown");
    await expect(breakdown).toContainText(
      `Parchemin des anciens archimages (${FR_SPELL}) : +1`,
    );
    await expectNoClippedText(breakdown, `scroll initiative term, ${where}`);
    await expectWholeWords(breakdown, `scroll initiative term, ${where}`);
    await expectInside(breakdown, page.locator("body"), `breakdown, ${where}`);
    await snap(page, `scroll-initiative-term-${vp.width}x${vp.height}`);
    await page.mouse.move(0, 0);
    await expect(breakdown).toBeHidden();
    // The editor's Spell picker shows the long name whole.
    await rail.nth(2).click();
    // The scroll's row in the possessions list names the spell, whole.
    const scrollRow = page.getByTestId("item-scroll_of_the_ancient_archmages");
    await expect(scrollRow.getByTestId("item-stats")).toContainText(FR_SPELL);
    await expectNoClippedText(scrollRow, `scroll row, ${where}`);
    await expectWholeWords(scrollRow, `scroll row, ${where}`);
    await snap(page, `scroll-row-spell-${vp.width}x${vp.height}`);
    await page
      .getByRole("button", {
        name: "Modifier Parchemin des anciens archimages",
      })
      .click();
    const form = page.getByTestId("item-form");
    const picker = form.getByLabel("Sort", { exact: true });
    await expect(picker).toHaveValue("long_spell");
    await expect(picker.locator("option:checked")).toHaveText(FR_SPELL);
    await expectNoClippedText(form, `item form, ${where}`);
    await expectWholeWords(form, `item form, ${where}`);
    await expectInside(picker, form, `spell picker, ${where}`);
    // A native select cuts its selected text without any scroll width: measure it in the select's font.
    const room = await picker.evaluate((el) => {
      const cs = getComputedStyle(el);
      const ctx = document.createElement("canvas").getContext("2d")!;
      ctx.font = cs.font;
      const text = (el as HTMLSelectElement).selectedOptions[0].text;
      const pad = parseFloat(cs.paddingLeft) + parseFloat(cs.paddingRight);
      return el.clientWidth - pad - ctx.measureText(text).width;
    });
    expect(room, `long spell name fits its picker, ${where}`).toBeGreaterThan(
      24,
    );
    await snap(page, `scroll-spell-form-${vp.width}x${vp.height}`);
    await form.getByRole("button", { name: "Annuler" }).click();
    await expect(form).toHaveCount(0);
    await rail.nth(0).click();
  }
}

async function nonProficiencyStep(page: Page, size: Viewport) {
  await page.setViewportSize(size);
  await openDashboard(page);
  const call = (method: string, url: string, body: unknown) =>
    apiCall(page, method, url, body);
  await openWeapons(page);
  await chooseWeapon(page, "Long Sword +2");
  const thac0 = attackForm(page).getByTestId("tile-thac0");
  const proficient = Number(await thac0.textContent());

  expect(
    await call("PUT", "/api/proficiencies", {
      weapon: ["short_bow", "dagger", "dart", "sling", "other_melee"].map(
        (type) => ({ type, slots: 1 }),
      ),
    }),
  ).toBe(true);
  expect(
    await call("POST", "/api/items", {
      name: { en: "Homebrew cudgel", fr: "Gourdin maison" },
      category: "weapon",
      damageSm: "1d6",
      damageL: "1d3",
      damageTypes: ["b"],
      weaponKind: "melee",
      behaviour: "stock",
      count: 1,
    }),
  ).toBe(true);
  await reloadSettled(page);
  await openWeapons(page);
  await expect(thac0).toHaveText(String(proficient + 3));
  await attackForm(page).getByLabel("Target AC").fill("5");
  await attackForm(page)
    .getByRole("button", { name: "Attack", exact: true })
    .click();
  const card = page.getByTestId("attack-result");
  await expect(card).toContainText("Long Sword +2 vs AC 5");
  await card.getByText("Calculation details").click();
  const penalty = card
    .getByTestId("to-hit-modifiers")
    .getByRole("listitem")
    .filter({ hasText: "Non-proficiency (Long sword)" });
  await expect(penalty).toHaveText(/Non-proficiency \(Long sword\)\s*−3/);
  // The journal's entry opens in the same Detail view: its row is the selected one.
  await expect(
    combatLog(page)
      .getByRole("button", { name: /Attack Long Sword \+2 vs AC 5/ })
      .first(),
  ).toHaveAttribute("aria-pressed", "true");

  // The typeless homebrew weapon: wielded, tagged, no penalty in its preview.
  await chooseWeapon(page, "Homebrew cudgel");
  const missing =
    '[data-testid="class-warning"][data-code="weapon_type_missing"]';
  await expect(attackForm(page).locator(missing)).toHaveText(
    "No weapon type: weapon proficiency not checked",
  );
  const form = (await (await page.request.get("/api/attack/form")).json()) as {
    preview: { modifiers: { kind: string }[] };
  };
  expect(form.preview.modifiers.map((m) => m.kind)).not.toContain(
    "nonproficient",
  );

  const rail = page.getByTestId("rail").getByRole("button");
  for (const lang of ["en", "fr"] as const) {
    await chooseLanguage(page, lang);
    const attack = page.getByRole("form", {
      name: lang === "fr" ? "Attaque" : "Attack",
    });
    const words =
      lang === "fr"
        ? [
            "Non-maîtrise (Épée longue)",
            "Sans type d'arme : compétence martiale non vérifiée",
          ]
        : [
            "Non-proficiency (Long sword)",
            "No weapon type: weapon proficiency not checked",
          ];
    for (const vp of sizesAt(size, [
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ])) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      await rail.nth(0).click();
      await expect(card.getByTestId("to-hit-modifiers")).toContainText(
        words[0],
      );
      await expect(attack.locator(missing)).toHaveText(words[1]);
      await expectNoClippedText(card, `result card, ${where}`);
      await expectNoClippedText(attack, `attack card, ${where}`);
      await expectNoClippedText(combatLog(page), `journal, ${where}`);
      await snap(page, `nonproficient-attack-${lang}-${vp.width}x${vp.height}`);
      await rail.nth(2).click();
      const row = page.getByTestId("item-homebrew_cudgel");
      await expect(row.locator(missing)).toHaveText(words[1]);
      await expect(page.getByTestId("ac-card").locator(missing)).toHaveCount(0);
      // Below 1920 the walk's items carry damage dice chips that 9 px overflow their cell at 1280
      // and 1440 (the spec never checked this list there): left to the dedicated inventory step.
      if (vp.width >= 1920)
        await expectNoClippedText(
          page.getByTestId("stage"),
          `inventory, ${where}`,
        );
      await snap(
        page,
        `nonproficient-inventory-${lang}-${vp.width}x${vp.height}`,
      );
    }
  }
}

async function classWarningsStep(page: Page, size: Viewport) {
  await page.setViewportSize(size);
  await openDashboard(page);
  const call = (method: string, url: string, body: unknown) =>
    apiCall(page, method, url, body);
  expect(
    await call("POST", "/api/characters", {
      name: "Ilyana Mainfroide d'Ombrebrume",
      class: "mage",
      raceCode: "human",
      details: { alignment: "neutral_good", sex: "female" },
      str: 9,
      dex: 14,
      con: 10,
      int: 17,
      wis: 10,
      cha: 12,
      hpMax: 4,
      xp: 0,
    }),
  ).toBe(true);
  await reloadSettled(page);
  const pieces = [
    {
      id: "full_plate_armour_of_the_northern_wardens",
      state: "worn",
      body: {
        name: {
          en: "Full plate armour of the northern wardens",
          fr: "Armure de plates complète des gardiens du nord",
        },
        category: "armour",
        baseAc: 1,
        skillColumn: "none_allowed",
        armorRow: "full_plate",
      },
    },
    {
      id: "tower_shield",
      state: "worn",
      body: {
        name: { en: "Tower shield", fr: "Bouclier-tour" },
        category: "shield",
        shieldKind: "body",
        acMelee: 1,
        acMissile: 2,
      },
    },
    {
      id: "great_claymore",
      state: "wielded",
      body: {
        name: { en: "Great claymore", fr: "Grande claymore à deux mains" },
        category: "weapon",
        damageSm: "1d10",
        damageL: "3d6",
        damageTypes: ["s"],
        weaponKind: "melee",
        weaponType: "long_sword",
        twoHanded: true,
      },
    },
  ];
  for (const p of pieces) {
    expect(
      await call("POST", "/api/items", {
        ...p.body,
        behaviour: "stock",
        count: 1,
      }),
      `create ${p.id}`,
    ).toBe(true);
    // Warned, never refused: wearing and wielding succeed.
    expect(
      await call("POST", `/api/items/${p.id}/state`, { state: p.state }),
      `equip ${p.id}`,
    ).toBe(true);
  }
  await reloadSettled(page);

  const codes = async (id: string) =>
    page
      .getByTestId(`item-${id}`)
      .getByTestId("class-warning")
      .evaluateAll((els) => els.map((e) => e.getAttribute("data-code")));
  const check = async (
    lang: "en" | "fr",
    vp: { width: number; height: number },
  ) => {
    await page.setViewportSize(vp);
    const where = `${lang}, ${vp.width}x${vp.height}`;
    const shot = vp.width === 1920 || vp.width === 2560;
    // The Inventaire: the warnings on the rows and on the AC card.
    await rail.nth(2).click();
    await expect(
      page.getByTestId("item-full_plate_armour_of_the_northern_wardens"),
    ).toBeVisible();
    expect(await codes("full_plate_armour_of_the_northern_wardens")).toEqual([
      "armour_not_allowed",
    ]);
    expect(await codes("tower_shield")).toEqual([
      "armour_not_allowed",
      "hands",
    ]);
    expect(await codes("great_claymore")).toEqual([
      "weapon_not_allowed",
      "hands",
    ]);
    const card = page.getByTestId("ac-card");
    await expect(card.getByTestId("ac-warnings")).toBeVisible();
    await expect(card.getByTestId("class-warning")).toHaveCount(3);
    // The shield's two warnings sit under its name, shown once.
    await expect(card.getByTestId("ac-warning-piece")).toHaveCount(2);
    await expect(card.getByTestId("ac-warnings")).toContainText(
      lang === "fr" ? "Pas pour un magicien" : "Not for a mage",
    );
    await expectNoClippedText(page.getByTestId("stage"), `inventory, ${where}`);
    if (shot)
      await snap(
        page,
        `class-warnings-inventory-${lang}-${vp.width}x${vp.height}`,
      );
    // The shield beside the HP: a badge, and the popover lists the warnings.
    const shield = page.getByTestId("ac-shield");
    await expect(shield.getByTestId("ac-shield-warning")).toBeVisible();
    await shield.click();
    const popover = page.getByTestId("ac-popover");
    await expect(popover.getByTestId("class-warning")).toHaveCount(3);
    await expectNoClippedText(popover, `AC popover, ${where}`);
    if (shot)
      await snap(
        page,
        `class-warnings-shield-${lang}-${vp.width}x${vp.height}`,
      );
    await page.keyboard.press("Escape");
    await expect(popover).toHaveCount(0);
    // The attack card: the weapon in hand carries its warnings.
    // The Weapons tab stays open between visits (its name is English-only in the helper).
    await rail.nth(0).click();
    const attack = page.getByRole("form", {
      name: lang === "fr" ? "Attaque" : "Attack",
    });
    await expect(attack.getByTestId("class-warning")).toHaveCount(2);
    await expectNoClippedText(attack, `attack card, ${where}`);
    if (shot)
      await snap(
        page,
        `class-warnings-attack-${lang}-${vp.width}x${vp.height}`,
      );
  };
  // The weapon in hand is picked once (in English); the choice is kept on the server.
  await openWorkspace(page, "Act");
  await openWeapons(page);
  await chooseWeapon(page, "Great claymore");
  const rail = page.getByTestId("rail").getByRole("button");
  for (const lang of ["en", "fr"] as const) {
    await chooseLanguage(page, lang);
    for (const vp of sizesAt(size, VIEWPORTS)) await check(lang, vp);
  }
}

async function spellDescriptionStep(page: Page, size: Viewport, stack: Stack) {
  const put = await fetch(`${stack.apiUrl}/api/spells/prepared`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ spells: ["chromatic_orb", "magic_missile"] }),
  });
  expect(put.ok).toBe(true);
  await openDashboard(page);

  /** The description box's shape: one column, the measure, the rhythm and the tables. */
  const expectOneColumn = async (box: Locator, where: string) => {
    const m = await box.evaluate((el) => {
      const probe = document.createElement("span");
      probe.style.cssText = "position:absolute;visibility:hidden;width:1ch";
      el.appendChild(probe);
      const ch = probe.getBoundingClientRect().width;
      probe.remove();
      const blocks = [...el.children] as HTMLElement[];
      const rect = (e: Element) => e.getBoundingClientRect();
      const cs = getComputedStyle(el);
      return {
        columns: cs.columnCount + "/" + cs.columnWidth,
        widthCh: rect(el).width / ch,
        lefts: blocks.map((b) => Math.round(rect(b).left)),
        gaps: blocks
          .slice(1)
          .map((b, i) => Math.round(rect(b).top - rect(blocks[i]).bottom)),
        lineHeight: parseFloat(cs.lineHeight),
        blocks: blocks.length,
        tables: [...el.querySelectorAll("[data-testid=spell-table]")].map(
          (t) => {
            const cell = t.querySelector("td")!;
            const ccs = getComputedStyle(cell);
            return {
              pad: [ccs.paddingLeft, ccs.paddingTop],
              inside: rect(t).right <= rect(el).right + 1,
            };
          },
        ),
        page: document.documentElement.scrollWidth - innerWidth,
      };
    });
    expect(m.columns, `no CSS columns, ${where}`).toMatch(/^auto\/auto$/);
    expect(m.widthCh, `measure, ${where}`).toBeLessThanOrEqual(75.5);
    expect(new Set(m.lefts).size, `one left edge, ${where}`).toBe(1);
    expect(m.blocks).toBeGreaterThan(1);
    for (const g of m.gaps)
      expect(g, `blocks apart, ${where}`).toBeGreaterThanOrEqual(16);
    for (const t of m.tables) {
      expect(t.pad, `cell padding, ${where}`).toEqual(["12px", "8px"]);
      expect(t.inside, `table in its box, ${where}`).toBe(true);
    }
    expect(m.page, `page does not widen, ${where}`).toBeLessThanOrEqual(0);
  };

  // The cast result's folded text (a long spell; the table spell cannot be cast in the fixture).
  await page.getByRole("tab", { name: "Spells" }).click();
  await page.getByRole("button", { name: "Cast Magic Missile" }).click();
  const card = page.getByTestId("cast-result");
  await expect(card).toBeVisible();
  await card.getByText("Spell text").click();

  const rail = page.getByTestId("rail").getByRole("button");
  for (const lang of ["en", "fr"] as const) {
    await chooseLanguage(page, lang);
    for (const vp of sizesAt(size, VIEWPORTS)) {
      await page.setViewportSize(vp);
      const where = `${lang} ${vp.width}x${vp.height}`;
      const shot = lang === "fr" || vp.width >= 1920;
      // The cast result.
      await rail.nth(0).click();
      if (!(await card.getByTestId("cast-text").isVisible()))
        await card.getByText(/Spell text|Texte du sort/).click();
      await expect(card.getByTestId("cast-text")).toBeVisible();
      await expectOneColumn(card.getByTestId("cast-text"), `cast, ${where}`);
      await expectNoClippedText(card, `cast result, ${where}`);
      if (shot)
        await snap(page, `description-cast-${lang}-${vp.width}x${vp.height}`);
      // The lookup, a table spell then a long one.
      await rail.nth(1).click();
      for (const [query, name, title] of [
        ["chromatic", "chromatic", /Chromatic Orb|Orbe chromatique/],
        ["magic missile", "missile", /Magic Missile|Projectile magique/],
      ] as const) {
        await page
          .getByRole("combobox", {
            name:
              lang === "fr" ? "Rechercher des sorts..." : "Search spells...",
          })
          .fill(query);
        const detail = page.getByTestId("spell-detail");
        await expect(detail.getByRole("heading").first()).toHaveText(title);
        const text = detail.getByTestId("spell-description-text");
        await expectOneColumn(
          text.locator("> div"),
          `lookup ${name}, ${where}`,
        );
        await expectNoClippedText(detail, `spell detail ${name}, ${where}`);
        await expectNoPageScroll(page);
        if (shot)
          await snap(
            page,
            `description-lookup-${name}-${lang}-${vp.width}x${vp.height}`,
          );
      }
    }
  }
}

async function endEffectsStep(page: Page, size: Viewport, stack: Stack) {
  await dismissEffects(stack);
  await page.setViewportSize(size);
  await openDashboard(page);
  for (const lang of ["fr", "en"] as const) {
    await chooseLanguage(page, lang);
    for (const vp of sizesAt(size, [
      { width: 1280, height: 800 },
      { width: 2560, height: 1440 },
    ])) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      expect(
        await apiCall(page, "POST", "/api/hp/event", {
          delta: -8,
          source: "e2e goblin",
        }),
      ).toBe(true);
      expect(
        await apiCall(page, "POST", "/api/spells/receive", {
          spellId: "polymorph_self",
          casterLevel: 15,
        }),
      ).toBe(true);
      await reloadSettled(page);
      await expect(page.getByTestId("hp-value")).toHaveText(/\d+ \/ \d+/);
      await page.getByTestId("buff-stop").click();
      const menu = page.getByRole("menu");
      await expect(menu).toBeVisible();
      await expectNoClippedText(menu, `stop menu, ${where}`);
      await expectWholeWords(menu, `stop menu, ${where}`);
      await page.getByTestId("buff-end-voluntary").click();
      const ended = page.getByTestId("notice-ended");
      await expect(ended).toBeVisible();
      await expectNoClippedText(ended, `end notice, ${where}`);
      await expectWholeWords(ended, `end notice, ${where}`);
      await expectNoClippedText(page.getByRole("status"), `notice, ${where}`);
      await expectNoClippedText(combatLog(page), `journal, ${where}`);
      await snap(page, `end-effects-${lang}-${vp.width}x${vp.height}`);
    }
  }
}

async function weaponProfEditorStep(page: Page, size: Viewport) {
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  await createInFlow(page, {
    name: "Borin",
    className: "Guerrier",
    race: "Humain",
    scores: { FOR: "16", DEX: "14", CON: "15", INT: "10", SAG: "10", CHA: "9" },
    hpMax: "10",
  });
  await expect(page.getByTestId("character")).toContainText("Borin");
  for (const lang of ["fr", "en"] as const) {
    const tr = (fr: string, en: string) => (lang === "fr" ? fr : en);
    await chooseLanguage(page, lang);
    await page.setViewportSize(size);
    // A new character opens on Act (Agir), where the Skills tab is.
    const actions = page.getByRole("region", { name: "Actions" });
    await actions
      .getByRole("tab", { name: tr("Compétences", "Skills") })
      .click();
    await openSpend(page, "weapon");
    const editor = page.getByTestId("weapon-editor");
    if (lang === "fr") {
      // The specialization in a long bow (3 slots) and a plain short sword: 4 of 4 spent.
      const bow = editor.getByTestId("weapon-row-long_bow");
      await bow.getByTestId("weapon-add").click();
      await bow.getByTestId("weapon-specialize").click();
      await editor
        .getByTestId("weapon-row-short_sword")
        .getByTestId("weapon-add")
        .click();
      await expect(editor.getByTestId("weapon-pool-spent")).toHaveText(
        "Dépensés 4, restants 0",
      );
    }
    for (const vp of sizesAt(size, VIEWPORTS)) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      await expectNoClippedText(editor, `weapon editor, ${where}`);
      await expectWholeWords(editor, `weapon editor, ${where}`);
      await expectNoPageScroll(page);
      if (vp.width >= 1920)
        await snap(page, `weapon-editor-${lang}-${vp.width}x${vp.height}`);
    }
    if (lang === "fr") await editor.getByTestId("weapon-save").click();
    else await editor.getByTestId("weapon-cancel").click();
    await expect(editor).toBeHidden();
  }
}

async function profPopupStep(page: Page, size: Viewport) {
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  await page.getByRole("tab", { name: "Compétences" }).click();
  const panel = page.getByRole("tabpanel", { name: "Compétences" });
  const dialog = page.getByTestId("proficiency-details");
  for (const vp of sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ])) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expect(
      panel.getByTestId("proficiency-name-musical_instrument"),
    ).toBeVisible();
    await expectNoClippedText(panel, `skills rows, ${where}`);
    await expectWholeWords(panel, `skills rows, ${where}`);
    await snap(page, `proficiency-rows-fr-${where}`);
    await panel.getByTestId("proficiency-name-musical_instrument").click();
    await expect(dialog.getByTestId("proficiency-description")).toBeVisible();
    await expectNoClippedText(dialog, `proficiency pop-up, ${where}`);
    await expectWholeWords(dialog, `proficiency pop-up, ${where}`);
    await snap(page, `proficiency-popup-fr-${where}`);
    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
  }
  await openSpend(page, "nonWeapon");
  const editor = page.getByTestId("proficiency-editor");
  for (const vp of sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ])) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expectNoClippedText(editor, `picker rows, ${where}`);
    await expectWholeWords(editor, `picker rows, ${where}`);
    await snap(page, `proficiency-picker-fr-${where}`);
  }
}

async function longRestStep(page: Page, size: Viewport) {
  await openDashboard(page);
  await nextRound(page);
  await page.getByRole("tab", { name: "Spells" }).click();
  for (const lang of ["fr", "en"] as const) {
    await chooseLanguage(page, lang);
    await page.keyboard.press("Escape");
    await page
      .getByRole("button", {
        name: lang === "fr" ? "Repos long" : "Long rest",
      })
      .click();
    const dialog = page.getByTestId("long-rest-confirm");
    await expect(dialog).toBeVisible();
    await expect(dialog).toContainText(
      lang === "fr" ? /tour \d+/ : /round \d+/,
    );
    for (const vp of sizesAt(size, VIEWPORTS)) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      await expectNoClippedText(dialog, `long rest confirmation, ${where}`);
      await expectWholeWords(dialog, `long rest confirmation, ${where}`);
      await expectNoPageScroll(page);
      if (vp.width >= 1920 && lang === "fr")
        await snap(page, `long-rest-confirm-fr-${vp.width}x${vp.height}`);
      if (vp.width >= 1920 && lang === "en")
        await snap(page, `long-rest-confirm-en-${vp.width}x${vp.height}`);
    }
    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
  }
}

async function weaponInHandStep(page: Page, size: Viewport, stack: Stack) {
  const api = (method: string, url: string, data: unknown) =>
    fetch(`${stack.apiUrl}${url}`, {
      method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    }).then((r) => expect(r.ok, `${method} ${url}`).toBe(true));
  await openDashboard(page);
  await openWeapons(page);
  // Trim or grow the character's weapons so exactly `wanted` are besides the one in hand.
  let added = 0;
  const growTo = async (wanted: number) => {
    const weapons = (await (
      await fetch(`${stack.apiUrl}/api/weapons`)
    ).json()) as { id: string }[];
    const held = (
      (await (await fetch(`${stack.apiUrl}/api/attack/form`)).json()) as {
        form: { weaponId: string };
      }
    ).form.weaponId;
    const spare = weapons.filter((w) => w.id !== held);
    for (const w of spare.slice(wanted))
      await fetch(`${stack.apiUrl}/api/items/${w.id}`, { method: "DELETE" });
    for (let n = spare.length; n < wanted; n++) {
      added += 1;
      await api("POST", "/api/items", {
        name: {
          en: `Spare ${LONG_WEAPON} number ${added}`,
          fr: `Arme de rechange ${LONG_WEAPON} numéro ${added}`,
        },
        category: "weapon",
        damageSm: "1d6",
        damageL: "1d6",
        damageTypes: ["s"],
        weaponKind: "melee",
        behaviour: "stock",
        count: 1,
      });
    }
    await reloadSettled(page);
    await openWeapons(page);
  };
  for (const wanted of [2, 6]) {
    await chooseLanguage(page, "en");
    await growTo(wanted);
    for (const lang of ["en", "fr"] as const) {
      await chooseLanguage(page, lang);
      const label = lang === "en" ? "Other weapons" : "Autres armes";
      for (const vp of sizesAt(size, [
        VIEWPORTS[0],
        VIEWPORTS[3],
        VIEWPORTS[4],
      ])) {
        await page.setViewportSize(vp);
        const where = `${wanted} others, ${lang}, ${vp.width}x${vp.height}`;
        const f =
          lang === "en"
            ? attackForm(page)
            : page.getByRole("form", { name: "Attaque" });
        // No native select in the panel; damage, speed and to-hit are separate labelled chips.
        await expect(
          f.locator("select"),
          `native select, ${where}`,
        ).toHaveCount(0);
        const stats = f.getByTestId("weapon-stats").locator("dt");
        await expect(stats.first()).toBeVisible();
        expect(await stats.count()).toBeGreaterThanOrEqual(4);
        const group = f.getByRole("group", { name: label });
        if (wanted <= 4)
          await expect(group.getByRole("button")).toHaveCount(wanted);
        else await expect(group.getByTestId("other-weapon-menu")).toBeVisible();
        await expectNoClippedText(f, `weapons tab, ${where}`);
        await snap(
          page,
          `weapon-chips-${wanted}-${lang}-${vp.width}x${vp.height}`,
        );
      }
    }
  }
  // With more than four the menu holds the rest, reachable by keyboard; choosing one puts it in hand.
  await chooseLanguage(page, "en");
  await page.setViewportSize(size);
  const menu = attackForm(page).getByTestId("other-weapon-menu");
  await menu.focus();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("menuitem").first()).toBeVisible();
  await snap(page, "weapon-menu-open-en-1920x1080");
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("Enter");
  await expect(attackForm(page).getByTestId("weapon-in-hand")).not.toBeEmpty();
}

/**
 * ADR 0017 step 1 (wide screens): from 2200 px wide the stage's result column is as wide as the act
 * panel (1fr | 1fr, instead of 1.35fr | 1fr) and the journal flows in two columns, so the same height
 * shows twice the rows; below it nothing changes (one journal column, the 1.35 : 1 split). No widget
 * and no journal line passes a readable length (about 1000 px, a column of the two).
 */
async function expectWideStage(
  page: Page,
  size: Viewport,
  where: string,
  withRows = true,
) {
  const wide = size.width >= 2200;
  const at = `${where}, ${size.width}x${size.height}`;
  const band = page.getByTestId("now-band");
  const [act, result] = await Promise.all([
    band.locator(":scope > *").nth(0).boundingBox(),
    band.locator(":scope > *").nth(1).boundingBox(),
  ]);
  const ratio = result!.width / act!.width;
  if (wide)
    expect(ratio, `result column as wide as act, ${at}`).toBeGreaterThan(0.95);
  else expect(ratio, `1.35 : 1 split kept, ${at}`).toBeLessThan(0.8);
  expect(act!.width, `act panel readable, ${at}`).toBeLessThanOrEqual(
    960 * 1.4,
  );
  const log = combatLog(page);
  if (!withRows) {
    if (size.width >= 1920)
      await snap(page, `wide-stage-${where}-${size.width}x${size.height}`);
    return;
  }
  const rows = await log.getByRole("listitem").all();
  expect(rows.length, `journal rows, ${at}`).toBeGreaterThanOrEqual(4);
  const lefts = new Set<number>();
  for (const row of rows) {
    const box = await row.boundingBox();
    if (!box) continue;
    lefts.add(Math.round(box.x / 40));
    if (wide)
      expect(box.width, `journal line length, ${at}`).toBeLessThanOrEqual(1000);
  }
  expect(lefts.size, `journal columns, ${at}`).toBe(wide ? 2 : 1);
  await expectNoClippedText(log, `journal, ${at}`);
  if (size.width >= 1920)
    await snap(page, `wide-stage-${where}-${size.width}x${size.height}`);
}

async function wideStageStep(page: Page, size: Viewport) {
  await openDashboard(page);
  for (const [i, src] of ["Orc axe", "Potion sip", "Trap", "Fall"].entries())
    await recordHp(page, i % 2 ? "heal" : "damage", 2 + i, src);
  await expectWideStage(page, size, "bard");
  await openWorkspace(page, "Spells");
  await expectNoPageScroll(page);
  if (size.width >= 1920)
    await snap(page, `wide-spells-${size.width}x${size.height}`);
  await openWorkspace(page, "Act");
}

async function journalRowsStep(page: Page, size: Viewport) {
  await openDashboard(page);
  await recordHp(page, "damage", 7, "Orc axe");
  await recordHp(page, "heal", 3, "Potion sip");
  // An attack is the latest result.
  await openWeapons(page);
  await chooseWeapon(page, "Long Sword +2");
  const form = attackForm(page);
  await form.getByLabel("Target AC").fill("5");
  await form.getByRole("button", { name: "Attack", exact: true }).click();
  // Adding a catalogue item and equipping it at once is one transaction: the undo group.
  await openWorkspace(page, "Inventory");
  await page.getByRole("button", { name: "Add item" }).click();
  await page.getByTestId("add-row-long_sword").click();
  await page.getByTestId("add-submit").click();
  await expect(page.getByTestId("add-piece")).toHaveCount(0);
  await openWorkspace(page, "Act");
  const log = combatLog(page);
  const group = log.getByTestId("log-transaction").first();
  await expect(group).toBeVisible();

  const look = (el: Locator) =>
    el.evaluate((node) => {
      const s = getComputedStyle(node);
      const clear = (c: string) =>
        c === "rgba(0, 0, 0, 0)" || c === "transparent";
      return {
        bracket: s.borderLeftWidth,
        fill: !clear(s.backgroundColor),
        bar: s.boxShadow !== "none",
        outline: s.outlineStyle === "none" ? "0px" : s.outlineWidth,
      };
    });
  // The undo group: a 2 px bracket, no fill.
  expect(await look(group)).toMatchObject({ bracket: "2px", fill: false });

  const damage = logEntry(page, "Orc axe");
  const heal = logEntry(page, "Potion sip");
  const icon = (row: Locator) => row.locator("svg").first().innerHTML();
  expect(await icon(damage), "damage and healing icons differ").not.toBe(
    await icon(heal),
  );

  // The newest result is the latest: the outline, no fill, no bar.
  const latestRow = log.locator('[data-latest="true"]');
  await expect(latestRow).toHaveCount(1);
  expect(await look(latestRow)).toMatchObject({
    fill: false,
    bar: false,
    outline: "2px",
  });
  await snap(page, "journal-rows-latest-en");
  // A row the player picks: the fill, no bar, no outline; a row of the group does not copy it.
  await damage.click();
  await expect(damage).toHaveAttribute("data-selected", "true");
  expect(await look(damage)).toMatchObject({
    fill: true,
    bar: false,
    outline: "0px",
  });
  for (const row of await group.getByRole("listitem").all())
    expect(await look(row)).toMatchObject({ fill: false });
  await snap(page, "journal-rows-en");

  // Every row is at least a 24 px target; keyboard focus shows a ring.
  for (const row of await log.getByRole("listitem").all())
    expect((await row.boundingBox())!.height).toBeGreaterThanOrEqual(24);
  await heal.getByTestId("log-select").focus();
  await page.keyboard.press("Shift+Tab");
  await page.keyboard.press("Tab");
  expect(
    await heal
      .getByTestId("log-select")
      .evaluate((el) => getComputedStyle(el).outlineStyle),
    "focus ring",
  ).toBe("solid");

  for (const lang of ["en", "fr"] as const) {
    await chooseLanguage(page, lang);
    for (const vp of sizesAt(size, [
      { width: 1280, height: 800 },
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ])) {
      await page.setViewportSize(vp);
      const where = `${lang} ${vp.width}x${vp.height}`;
      await expectNoClippedText(log, `journal rows, ${where}`);
      await snap(page, `journal-rows-${lang}-${vp.width}x${vp.height}`);
    }
  }
  for (const [mode, theme] of [
    ["Sombre", "Arcane"],
    ["Clair", "Parchemin"],
  ] as const) {
    await chooseSetting(page, mode);
    await chooseTheme(page, theme);
    await page.waitForTimeout(150);
    await expectNoClippedText(log, `journal rows, ${theme}`);
    await snap(page, `journal-rows-fr-${theme.toLowerCase()}`);
  }
}

// One focus ring on every tab stop in every theme (ADR 0026 step 2.2), and the 24 px targets of the
// small controls (the name, the level, the version, the mind-affecting checkbox).
async function focusRingTargetsStep(page: Page) {
  await openDashboard(page);
  await expectMinTarget(page.getByTestId("details-trigger"), "character name");
  await expectMinTarget(page.getByTestId("level-open"), "level");
  await expectMinTarget(page.getByTestId("version-open"), "version badge");
  await expectMinTarget(
    page.getByTestId("save-mind-affecting"),
    "against the mind",
  );
  for (const [mode, themes] of [
    ["Dark", ["Arcane", "Deep", "Plain", "Forest", "Ember", "Amber"]],
    ["Light", ["Arcane", "Parchment", "Plain", "Forest"]],
  ] as const) {
    await chooseSetting(page, mode);
    for (const name of themes) {
      await chooseTheme(page, name);
      await expectOneFocusRing(page, `${mode} ${name}`);
      if (name === "Arcane" || name === "Parchment") {
        await page.getByTestId("version-open").focus();
        await snap(
          page,
          `focus-ring-${mode.toLowerCase()}-${name.toLowerCase()}`,
        );
      }
    }
  }
}

type Step = (
  page: Page,
  size: Viewport,
  stack: Stack,
  context: BrowserContext,
) => Promise<void>;

/**
 * ADR 0017 step 2: no visible text is set below the 12 px floor, on the four activities of the
 * dashboard. Computed styles are a browser's answer, no unit test can give them.
 */
async function textFloorStep(page: Page, size: Viewport) {
  await page.setViewportSize(size);
  await openDashboard(page);
  for (const lang of ["fr", "en"] as const) {
    await chooseLanguage(page, lang);
    const rail = page.getByTestId("rail").getByRole("button");
    for (const [i, activity] of [
      "Act",
      "Spells",
      "Inventory",
      "Experience",
    ].entries()) {
      await rail.nth(i).click();
      await expectTextFloor(
        page.locator("body"),
        `${activity}, ${lang}, ${size.width}x${size.height}`,
      );
    }
  }
}

/** The same floor on the roster, the admin page and the ruleset page. */
async function textFloorPagesStep(page: Page, size: Viewport) {
  await page.setViewportSize(size);
  await openDashboard(page);
  await page.getByTestId("roster-open").click();
  await expect(page.getByTestId("roster")).toBeVisible();
  await expectTextFloor(
    page.locator("body"),
    `roster, ${size.width}x${size.height}`,
  );
  await page.keyboard.press("Escape");
  await page.goto("/admin");
  await expect(page.getByTestId("admin-users")).toBeVisible();
  await expectTextFloor(
    page.locator("body"),
    `admin, ${size.width}x${size.height}`,
  );
  await page.goto("/ruleset");
  await expect(page.getByTestId("crit-table")).toBeVisible();
  await expectTextFloor(
    page.locator("body"),
    `ruleset, ${size.width}x${size.height}`,
  );
}

/**
 * The dashboard walk: every screen and state of the dashboard in turn, in one test on one stack, at one
 * size. Each step runs the no-cut, whole-word and page-scroll checks of its region. The server state
 * carries over from step to step (what a step adds stays), so the steps that change the character
 * more (a multi-class sheet) come last; the browser's local storage is cleared between steps, so each
 * starts in the default language and theme.
 */
const DASHBOARD_STEPS: [string, Step][] = [
  ["end effects", endEffectsStep],
  ["shell", shellStep],
  ["item power row", itemPowerRowStep],
  ["text floor", textFloorStep],
  ["act panel", actPanelStep],
  ["French and long names", frenchLongNamesStep],
  ["saves label line", savesLabelStep],
  ["save card", saveCardStep],
  ["ability check card", abilityCardStep],
  ["attack card", attackCardStep],
  ["last result", lastResultStep],
  ["journal rows", journalRowsStep],
  ["wide stage", wideStageStep],
  ["spells tab", spellsTabStep],
  ["spell description", spellDescriptionStep],
  ["inventory, AC shield", acShieldStep],
  ["scroll spell", scrollSpellStep],
  ["piece editor spell component", pieceSpellStep],
  ["item form steps and edit dialog", itemFormStep],
  ["piece editor powers", piecePowersStep],
  ["Ajouter", ajouterStep],
  ["skills editor", skillsEditorStep],
  ["card details", cardDetailsStep],
  ["dice bar", diceBarStep],
  ["proficiency pop-up", profPopupStep],
  ["effects timeline", effectsTimelineStep],
  ["long rest", longRestStep],
  ["settings popover", settingsPopoverStep],
  ["focus ring and targets", focusRingTargetsStep],
  ["non-proficiency", nonProficiencyStep],
  ["weapon in hand", weaponInHandStep],
  ["class warnings", classWarningsStep],
  ["class lines, multi-class", (p, s) => classLinesStep(p, s, "multi")],
  ["class lines, three classes", (p, s) => classLinesStep(p, s, "three")],
  ["class lines, dual-class", (p, s) => classLinesStep(p, s, "dual")],
  ["weapon proficiency editor", weaponProfEditorStep],
  ["prepare, long spellbook", prepareLongBookStep],
];

async function runWalk(
  page: Page,
  size: Viewport,
  steps: [string, Step][],
  stack: Stack,
) {
  const context = page.context();
  for (const [name, step] of steps) {
    // Each step starts from the walk's size, signed in, with no route of an earlier step left.
    await page.unrouteAll({ behavior: "ignoreErrors" });
    await context.addCookies([
      {
        name: stack.session.name,
        value: stack.session.value,
        domain: new URL(stack.webUrl).hostname,
        path: "/",
        httpOnly: true,
        sameSite: "Lax",
      },
    ]);
    await page.setViewportSize(size);
    await test.step(name, async () => {
      await step(page, size, stack, context);
    });
    // Local storage holds the language and the theme: the next step starts from the defaults.
    await page.evaluate(() => localStorage.clear()).catch(() => undefined);
  }
}

// Two halves per size, each on a stack of its own: one session would run past the 90 s test timeout on
// the CI runner.
// The first ten steps take about as long as the rest.
const HALF = 10;
for (const size of [VIEWPORTS[0], VIEWPORTS[1], VIEWPORTS[3], VIEWPORTS[4]]) {
  for (const [part, steps] of [
    ["first half", DASHBOARD_STEPS.slice(0, HALF)],
    ["second half", DASHBOARD_STEPS.slice(HALF)],
  ] as const) {
    test(`dashboard walk at ${size.width}x${size.height}, ${part}`, async ({
      page,
      stack,
    }) => {
      test.setTimeout(90_000);
      await runWalk(page, size, [...steps], stack);
    });
  }
}

// Shots of the themes and the group headings, on a stack of their own.
test("dashboard walk at 1440x900, themes and group headings", async ({
  page,
  stack,
}) => {
  test.setTimeout(90_000);
  await runWalk(
    page,
    VIEWPORTS[1],
    [
      ["themes", themesStep],
      ["group headings", groupHeadingsStep],
    ],
    stack,
  );
});

// The design's second viewport (ADR 0022): the shell and the act panel, on a stack of their own.
test("dashboard walk at 1707x1300", async ({ page, stack }) => {
  test.setTimeout(90_000);
  await runWalk(
    page,
    VIEWPORTS[2],
    [
      ["shell", shellStep],
      ["act panel", actPanelStep],
    ],
    stack,
  );
});

// ADR 0022 step 5.4: the handle between the now band and the journal resizes the journal by drag
// (mouse) and arrows (keyboard); a click (or Enter) on it folds the journal into a one-line bar and
// back, a drag of 4 px or more only resizes; both are kept per device
// across a reload. What only a browser proves: pointer drag, layout heights and localStorage.
async function journalSplitStep(page: Page) {
  await openDashboard(page);
  await recordHp(page, "damage", 3, "Split goblin");
  const slot = page.getByTestId("journal-slot");
  const band = page.getByTestId("now-band");
  const handle = page.getByTestId("journal-handle");
  const heightOf = async (l: Locator) => (await l.boundingBox())!.height;
  // Default: the band has its fixed resting height or less, the journal at least 14rem.
  expect(await heightOf(band)).toBeLessThanOrEqual(680 + 1);
  // Measure the journal once the logged entry has rendered and the layout has stopped moving.
  await expect(combatLog(page).getByRole("listitem").first()).toBeVisible();
  const settled = async () => {
    let last = -1;
    await expect
      .poll(async () => {
        const now = await heightOf(slot);
        const same = now === last;
        last = now;
        return same;
      })
      .toBe(true);
    return last;
  };
  const before = await settled();
  expect(before).toBeGreaterThanOrEqual(224);

  // Drag the handle up by 120 px: the journal grows by as much, the band shrinks. Up, not down:
  // the default journal may already be at its 14rem minimum, which a downward drag cannot go under.
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
  // Keyboard: ArrowUp grows it by one step.
  await handle.focus();
  await page.keyboard.press("ArrowUp");
  await expect.poll(() => heightOf(slot)).toBeCloseTo(dragged + 16, 0);
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
  await reloadSettled(page);
  await expect(page.getByTestId("journal-bar")).toContainText("Split goblin");
  await page.getByTestId("journal-handle").click();
  await expect(combatLog(page).getByRole("listitem").first()).toBeVisible();
  await expect.poll(() => heightOf(slot)).toBeCloseTo(dragged + 16, 0);
}

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
async function railStep(page: Page) {
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
  await expect(inventory.getByTestId("category-chips")).toBeVisible();
  await openTypedAdd(inventory);
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
}

const isDark = (page: Page) =>
  page.evaluate(() => document.documentElement.classList.contains("dark"));
const rootVar = (page: Page, name: string) =>
  page.evaluate(
    (n) =>
      getComputedStyle(document.documentElement).getPropertyValue(n).trim(),
    name,
  );

// ADR 0026 step 3.1: the theme grid never leaves one theme alone on a row.
async function expectNoLoneRow(list: Locator, what: string) {
  const tops = await list
    .getByRole("radio")
    .evaluateAll((els) =>
      els.map((e) => Math.round(e.getBoundingClientRect().top)),
    );
  const perRow = new Map<number, number>();
  for (const t of tops) perRow.set(t, (perRow.get(t) ?? 0) + 1);
  expect(
    [...perRow.values()].every((n) => n >= 2),
    `${what}: themes per row ${[...perRow.values()].join(",")}`,
  ).toBe(true);
}

// ADR 0026 step 4: a dark and a light theme per device, the mode picks which applies, "system"
// switches live with the OS; both choices survive a reload and the inline script paints the
// effective one before the app runs.
async function settingsStep(page: Page) {
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
  await reloadSettled(page);
  expect(await isDark(page)).toBe(true);
  expect(await rootVar(page, "--tint")).toMatch(DEEP);
  await page.unroute(appScript);

  await reloadSettled(page);
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
  await expectNoLoneRow(popover.getByTestId("settings-theme"), "dark list, fr");
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
  await expectNoLoneRow(
    popover.getByTestId("settings-theme"),
    "light list, fr",
  );
  await snap(page, "settings-popover-fr-light");
  await expect.poll(() => rootVar(page, "--tint")).toMatch(PARCHMENT);
  await popover.getByRole("radio", { name: "Sombre", exact: true }).click();
  await expect(
    popover
      .getByTestId("settings-theme")
      .getByRole("radio", { name: "Abysses", exact: true }),
  ).toHaveAttribute("aria-checked", "true");
  await expect.poll(() => rootVar(page, "--tint")).toMatch(DEEP);
}

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

// What only a browser proves: the Inventaire dice chip and the Sorts "To the dice" fill the composer
// (a client store shared across workspaces), and Roll on a spell's dice reaches the dice endpoint
// and lands in the card and the journal. The formula wording is a unit test (spellValues), the
// values at level are Go tests (spellstats). Screens of both workspaces at the four viewports.
const COMPOSER_EXTRA_SPELLS = {
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
};

async function composerStep(page: Page) {
  await openDashboard(page);
  const formula = page
    .getByRole("form", { name: "Dice" })
    .getByLabel("Formula");
  // "To the dice" expands the composer by itself.

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
  await showWizardSpells(page);
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
  await expect(formula).toHaveCount(0);
  // The roll shows at once in the header's last result, from any activity (ADR 0022 step 2.2).
  const last = page.getByTestId("last-result");
  await expect(last.getByTestId("last-result-text")).toContainText("d8+8");
  const total = Number(
    await last.getByTestId("last-result-chip").textContent(),
  );
  expect(total).toBeGreaterThanOrEqual(9);
  expect(total).toBeLessThanOrEqual(16);
  await openDice(page);
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
  const fire = Number(await last.getByTestId("last-result-chip").textContent());
  expect(fire).toBeGreaterThanOrEqual(10);
  expect(fire).toBeLessThanOrEqual(60);

  for (const vp of VIEWPORTS) {
    await page.setViewportSize(vp);
    await expectNoPageScroll(page);
    await snap(page, `spells-lookup-${vp.width}x${vp.height}`);
  }
}

// The login page (ADR 0022 step 1, ADR 0027 step 2): with both login buttons and the longest
// refusal message, no text is cut in French and English at every layout viewport; screenshots in
// French. The e2e API has no OAuth settings, so its login options are answered here with both.
async function loginStep(
  page: Page,
  size: Viewport,
  _stack: Stack,
  context: BrowserContext,
) {
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
    for (const vp of sizesAt(size, VIEWPORTS)) {
      await page.setViewportSize(vp);
      await expectNoClippedText(card, `login page ${lang} ${vp.width}`);
      await expectWholeWords(card, `login page ${lang} ${vp.width}`);
      await expectNoPageScroll(page);
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
    await reloadSettled(page);
    await expect(page.locator("html")).toHaveClass(/dark/);
    await expect(card.getByTestId("login-error")).toBeVisible();
    await expectNoClippedText(card, `login page dark ${dark}`);
    await snap(page, `login-fr-dark-${dark}`);
  }
}

// ADR 0022 step 1 for the database banner (ADR 0025 step 10.4), French first: with both of its
// lines (a write whose outcome is unknown), it wraps and is never cut, fr and en, 1280x800 to
// 2560x1440.
async function dbBannerStep(page: Page, size: Viewport) {
  await page.setViewportSize(size);
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
  await recordHp(page, "damage", 4, "e2e lost blow", { refused: true });
  const banner = page.getByTestId("database-banner");
  await expect(banner).toContainText(
    "Your last action may not have been saved",
  );
  for (const lang of ["fr", "en"] as const) {
    await chooseLanguage(page, lang);
    for (const vp of sizesAt(size, [
      { width: 1280, height: 800 },
      { width: 2560, height: 1440 },
    ])) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      await expectNoClippedText(banner, `database banner, ${where}`);
      if (lang === "fr")
        await snap(page, `database-banner-${where.replace(", ", "-")}`);
    }
  }
}

// The admin section (ADR 0029 step 1.3), French first: a player with a long email and two
// characters' worth of names, a disabled user and a long pending invite; the invites and users cards
// are never cut, fr and en, 1280x800 to 2560x1440.
async function adminStep(
  page: Page,
  size: Viewport,
  stack: Stack,
  context: BrowserContext,
) {
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
    language: "fr",
  });

  // The dev login has no Google name: give two users one (a long one too) in the list, and the
  // session's user, so the name-over-email rows are laid out as in production.
  await page.route("**/api/admin/users", async (route) => {
    const res = await route.fetch();
    const body = (await res.json()) as {
      users: { email: string; displayName?: string }[];
    };
    for (const u of body.users) {
      if (u.email === "bob@example.com") u.displayName = "Bob Lamarre";
      if (u.email.startsWith("aurelie"))
        u.displayName =
          "Aurélie de La Tour d'Auvergne-Lauragais de Saint-Aignan-sur-Cher";
    }
    await route.fulfill({ response: res, json: body });
  });
  await page.setViewportSize(size);
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  await page.goto("/admin");
  const invites = page.getByTestId("admin-invites");
  const users = page.getByTestId("admin-users");
  await expect(users.getByTestId("admin-user")).toHaveCount(4);
  await expect(invites.getByRole("listitem")).toHaveCount(1);
  await invites
    .getByRole("button", { name: /^Copier le lien de l.application/ })
    .click();
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
    // The language tag, Resend and the outcome hint are part of the card: the longest hint is the
    // failed one (the address in it is the long one), forced here since the e2e sender succeeds.
    await page.route("**/api/admin/invites/*/resend", async (route) => {
      const res = await route.fetch();
      await route.fulfill({
        response: res,
        json: { ...(await res.json()), mail: "failed" },
      });
    });
    await invites
      .getByRole("button", {
        name: lang === "fr" ? /^Envoyer de nouveau/ : /again$/,
      })
      .click();
    await expect(invites.getByTestId("admin-mail-hint")).toBeVisible();
    for (const vp of sizesAt(size, [
      { width: 1280, height: 800 },
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ])) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      await expectNoClippedText(invites, `admin invites, ${where}`);
      await expectNoClippedText(users, `admin users, ${where}`);
      // The admin page may scroll as a page (a long user list), but nothing escapes its scrollers.
      expect(await escapedAbsolutes(page), `admin, ${where}`).toEqual([]);
      await snap(page, `admin-${lang}-${vp.width}x${vp.height}`);
    }
  }
}

// The Ruleset screen (ADR 0041 step 8), French first: the crit table with a long error from the
// server, the house-rule switches with the level limits mode and the success effects are never
// cut, fr and en, 1280x800 to 2560x1440. The page may scroll as a page, but nothing escapes its
// scrollers.
async function rulesetAdminStep(page: Page, size: Viewport) {
  await page.setViewportSize(size);
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  await page.goto("/ruleset");
  const crit = page.getByTestId("crit-table");
  const rules = page.getByTestId("ruleset-house-rules");
  const effects = page.getByTestId("ruleset-success-effects");
  await expect(crit.getByRole("group", { name: /^Plage 1$/ })).toBeVisible();
  await expect(effects).toContainText("Acrobatie");

  for (const lang of ["fr", "en"] as const) {
    if (lang === "en") {
      await page.goto("/");
      await chooseLanguage(page, "en");
      await page.goto("/ruleset");
      await expect(effects).toContainText("Tumbling");
    }
    // The longest message there is: the server's refusal of the table, shown under the rows.
    await crit
      .getByLabel(lang === "fr" ? "À" : "To")
      .first()
      .fill("40");
    // A house-rule tick is a second unsaved edit, so two cards carry the "not saved" tag.
    await rules.getByRole("checkbox").first().click();
    await expect(page.getByTestId("unsaved-mark")).toHaveCount(2);
    await page
      .getByRole("button", {
        name: lang === "fr" ? /^Enregistrer tous/ : /^Save every change/,
      })
      .click();
    await expect(crit.getByRole("alert")).toBeVisible();
    // The refused table keeps its edit and its tag; the accepted house rule is saved and untagged.
    await expect(page.getByTestId("unsaved-mark")).toHaveCount(1);
    // A two-digit number's field is sized to it (about 4 rem), not stretched to its column.
    const toBox = await crit
      .getByLabel(lang === "fr" ? "À" : "To")
      .first()
      .boundingBox();
    expect(toBox!.width, "crit field width").toBeLessThan(80);
    for (const vp of sizesAt(size, [
      { width: 1280, height: 800 },
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ])) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      await expectNoClippedText(crit, `ruleset crit table, ${where}`);
      await expectNoClippedText(rules, `ruleset house rules, ${where}`);
      await expectNoClippedText(effects, `ruleset success effects, ${where}`);
      await expectNoClippedText(
        page.getByTestId("ruleset-save-bar"),
        `ruleset save bar, ${where}`,
      );
      expect(await escapedAbsolutes(page), `ruleset, ${where}`).toEqual([]);
      await snap(page, `ruleset-${lang}-${vp.width}x${vp.height}`);
    }
  }
}

// The Ruleset screen as a player sees it (ADR 0041 step 8): every card shown read-only, the weapon
// switch included, no edit control, uncut in fr and en at 1280 to 2560. /api/auth/me answers role
// "player" for the root admin's sheet, as admin.spec does.
async function rulesetPlayerStep(page: Page, size: Viewport) {
  await page.route("**/api/auth/me", async (route) => {
    const res = await route.fetch();
    await route.fulfill({
      response: res,
      json: { ...(await res.json()), role: "player" },
    });
  });
  await page.setViewportSize(size);
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  await page.goto("/ruleset");
  const crit = page.getByTestId("crit-table");
  const rules = page.getByTestId("ruleset-house-rules");
  const effects = page.getByTestId("ruleset-success-effects");
  await expect(crit.getByRole("group", { name: /^Plage 1$/ })).toBeVisible();
  await expect(effects).toContainText("Acrobatie");
  await expect(page.getByTestId("ruleset-weapon-lift")).toBeDisabled();
  await expect(page.locator("main").getByRole("button")).toHaveCount(0);
  await expect(
    page.locator("main input:not([readonly]):not([disabled])"),
  ).toHaveCount(0);

  for (const lang of ["fr", "en"] as const) {
    if (lang === "en") {
      await page.goto("/");
      await chooseLanguage(page, "en");
      await page.goto("/ruleset");
      await expect(effects).toContainText("Tumbling");
    }
    for (const vp of sizesAt(size, [
      { width: 1280, height: 800 },
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ])) {
      await page.setViewportSize(vp);
      const where = `player, ${lang}, ${vp.width}x${vp.height}`;
      await expectNoClippedText(crit, `ruleset crit table, ${where}`);
      await expectNoClippedText(rules, `ruleset house rules, ${where}`);
      await expectNoClippedText(effects, `ruleset success effects, ${where}`);
      expect(await escapedAbsolutes(page), `ruleset, ${where}`).toEqual([]);
      await snap(page, `ruleset-player-${lang}-${vp.width}x${vp.height}`);
    }
  }
}

// The imported character lacks its alignment and sex (the roster cards' tag), the one created below has them.

// The roster, "Vos personnages" (ADR 0034 step 7), and the intro of a user with no character (step
// 1), French first: the roster with a long name, its rename and retire dialogs, the character card's
// header with the roster's way in, the details with Nom first (step 7.2) and the cog menu left with
// no character action (step 8) are never cut, fr and en, 1280x800, 1920x1080 and 2560x1440, and the
// page never scrolls. The current character retired elsewhere (another tab) leaves a usable screen:
// back in the game the page loads on the next one.
async function rosterStep(page: Page, size: Viewport) {
  await page.setViewportSize(size);
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
          body: (await r.json()) as {
            characters: { id: number }[];
            currentId: number | null;
          },
        };
      },
      [method, url, body] as const,
    );
  const created = await call("POST", "/api/characters", {
    name: "Aldéric de la Tour-d'Auvergne-Lesparre",
    class: "bard",
    raceCode: "half_elf",
    details: { alignment: "chaotic_neutral", sex: "male" },
    str: 10,
    dex: 15,
    con: 12,
    int: 14,
    wis: 9,
    cha: 16,
    hpMax: 6,
    xp: 0,
  });
  expect(created.ok).toBe(true);
  await reloadSettled(page);
  const sizes = sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ]);
  const dialog = page.getByTestId("character-dialog");
  const roster = page.getByTestId("roster");
  const character = page.getByTestId("character");

  for (const lang of ["fr", "en"] as const) {
    await page.setViewportSize(sizes[0]!);
    if (lang === "en") await chooseLanguage(page, "en");
    const tr = (fr: string, en: string) => (lang === "fr" ? fr : en);
    for (const vp of sizes) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      // The cog menu: settings and account, no character action.
      const settings = await openSettings(page);
      await expect(
        settings.getByRole("button", {
          name: tr("Créer un nouveau personnage", "Create a new character"),
        }),
      ).toHaveCount(0);
      await expectNoClippedText(settings, `cog menu, ${where}`);
      await snap(page, `cog-menu-${lang}-${vp.width}x${vp.height}`);
      await page.keyboard.press("Escape");
      // The card's header: the long name wraps beside the level and the roster's button.
      await expectNoClippedText(
        character.locator(".group-band"),
        `character card header, ${where}`,
      );
      await page.getByTestId("details-trigger").click();
      const pop = page.getByTestId("details-popover");
      await pop.getByRole("button", { name: tr("Modifier", "Edit") }).click();
      await expect(
        pop.getByLabel(tr("Nom", "Name"), { exact: true }),
      ).toHaveValue("Aldéric de la Tour-d'Auvergne-Lesparre");
      await expectNoClippedText(pop, `details with Nom, ${where}`);
      await snap(page, `character-card-${lang}-${vp.width}x${vp.height}`);
      await page.keyboard.press("Escape");
      await expect(pop).toBeHidden();

      await page.getByTestId("roster-open").click();
      await expect(roster.getByTestId("roster-card")).toHaveCount(2);
      // Each card shows its HP; race · alignment and no tag on the one with a complete identity,
      // the tag on the one still to choose (the imported sheet).
      await expect(roster.getByTestId("roster-hp")).toHaveCount(2);
      const complete = roster
        .getByTestId("roster-card")
        .filter({ hasText: "Aldéric" });
      await expect(complete.getByTestId("roster-identity")).toHaveText(
        tr("Demi-elfe · Chaotique neutre", "Half-elf · Chaotic neutral"),
      );
      await expect(complete.getByTestId("roster-missing")).toHaveCount(0);
      await expect(complete.getByTestId("roster-hp")).toHaveText(
        tr("PV 6/6", "HP 6/6"),
      );
      await expect(roster.getByTestId("roster-missing")).toHaveCount(1);
      await expect(roster.getByTestId("roster-missing")).toHaveText(
        tr("Alignement et sexe à choisir", "Alignment and sex to pick"),
      );
      await expectNoClippedText(page.locator("body"), `roster, ${where}`);
      await expectNoPageScroll(page);
      await snap(page, `roster-${lang}-${vp.width}x${vp.height}`);

      await roster
        .getByRole("button", { name: /^(Renommer|Rename) Aldéric/ })
        .click();
      await expect(dialog.getByTestId("rename-form")).toBeVisible();
      await expectNoClippedText(dialog, `rename dialog, ${where}`);
      await snap(page, `roster-rename-${lang}-${vp.width}x${vp.height}`);
      await page.keyboard.press("Escape");
      await expect(dialog).toBeHidden();
      await roster
        .getByRole("button", { name: /^(Retirer|Retire) Aldéric/ })
        .click();
      await expect(dialog.getByTestId("retire-confirm")).toBeVisible();
      await expectNoClippedText(dialog, `retire dialog, ${where}`);
      await snap(page, `roster-retire-${lang}-${vp.width}x${vp.height}`);
      await page.keyboard.press("Escape");
      await expect(dialog).toBeHidden();
      await page
        .getByRole("button", {
          name: tr("Revenir à la partie", "Back to the game"),
        })
        .click();
      await expect(character).toBeVisible();
    }
  }

  // The current character retired elsewhere while the roster shows: back in the game, the server
  // says the character changed and the page loads on the one it made current.
  await page.setViewportSize(sizes[0]!);
  await page.getByTestId("roster-open").click();
  await expect(roster.getByTestId("roster-card")).toHaveCount(2);
  const retired = await call(
    "PATCH",
    `/api/characters/${created.body.currentId}`,
    { retired: true },
  );
  expect(retired.ok).toBe(true);
  await page.getByRole("button", { name: "Back to the game" }).click();
  await expect(character).toContainText("Test Bard");

  // The last character retired: the intro (no character left), then Commencer opens the flow.
  const list = await call("GET", "/api/characters", undefined);
  for (const c of list.body.characters)
    expect(
      (await call("PATCH", `/api/characters/${c.id}`, { retired: true })).ok,
    ).toBe(true);
  await page.setViewportSize(size);
  // Not a goto: a request the page had in flight answers "character changed" now and reloads the
  // page itself, which interrupts a navigation.
  await reloadSettled(page);
  const intro = page.getByTestId("intro");
  await expect(intro).toBeVisible();
  for (const lang of ["fr", "en"] as const) {
    await page
      .getByRole("radio", {
        name: lang === "fr" ? "Français" : "English",
        exact: true,
      })
      .click();
    for (const vp of sizesAt(size, [
      { width: 1280, height: 800 },
      { width: 2560, height: 1440 },
    ])) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      await expectNoClippedText(page.locator("body"), `intro, ${where}`);
      await expectWholeWords(intro, `intro, ${where}`);
      await expectNoPageScroll(page);
      await snap(page, `intro-${lang}-${vp.width}x${vp.height}`);
    }
  }
  await page
    .getByRole("button", { name: "Start creating your character" })
    .click();
  await expect(page.getByTestId("creation-step")).toBeVisible();
  // Quitter with nothing entered goes straight back to the intro.
  await page.getByRole("button", { name: "Leave the creation" }).click();
  await expect(intro).toBeVisible();
}

// The creation route (ADR 0034 steps 2 to 4), judged in French first (ADR 0027 step 2) and audited
// in English: each step, filled as a level 5 half-elf bard with the server's rolls (the longest
// content: method V's six totals, five HP rows, the bard's skills), is never cut at 1280x800,
// 1920x1080 and 2560x1440, the page never scrolls (the step card does) and the frame is capped at
// 1680 px on wide screens (step 2.2). A refusal opens its step with the field's error.
async function creationStep(page: Page, size: Viewport) {
  // Leaving a filled-in flow asks the browser's beforeunload question: accept it.
  await openDashboard(page);
  const sizes = sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ]);
  for (const lang of ["fr", "en"] as const) {
    await page.setViewportSize(sizes[0]!);
    await page.goto("/");
    await chooseLanguage(page, lang);
    await page.goto("/create");
    const step = page.getByTestId("creation-step");
    const next = step.getByRole("button", {
      name: lang === "fr" ? "Passer à l'étape suivante" : "Go to the next step",
    });
    const audit = async (name: string) => {
      for (const vp of sizes) {
        await page.setViewportSize(vp);
        const where = `${name}, ${lang}, ${vp.width}x${vp.height}`;
        await expectNoClippedText(page.locator("body"), `creation ${where}`);
        await expectNoPageScroll(page);
        const frame = await page
          .locator("[data-screen=create-character]")
          .boundingBox();
        expect(frame!.width, `frame width, ${where}`).toBeLessThanOrEqual(1680);
        await snap(page, `creation-${name}-${lang}-${vp.width}x${vp.height}`);
      }
      await page.setViewportSize(sizes[0]!);
      // One name per step: the stepper's current entry and the step card's title read the same.
      if (name !== "refusal")
        await expect(
          page.getByTestId("creation-steps").locator("[aria-current=step]"),
        ).toContainText(
          (await step.getByRole("heading", { level: 1 }).textContent()) ?? "",
        );
    };
    const tr = (fr: string, en: string) => (lang === "fr" ? fr : en);

    await step
      .getByRole("radio", { name: tr("Barde", "Bard"), exact: true })
      .click();
    // ADR 0034 step 4: the starting level is Level by default, XP the alternative.
    await expect(
      step
        .getByTestId("starting-level")
        .getByRole("radio", { name: tr("Niveau", "Level"), exact: true }),
    ).toBeChecked();
    await step
      .getByRole("radio", { name: tr("Niveau 5", "Level 5"), exact: true })
      .click();
    await audit("1-class");
    await next.click();
    await step
      .getByRole("radio", { name: tr("Demi-elfe", "Half-elf"), exact: true })
      .click();
    await audit("2-race");
    await next.click();
    await step.getByTestId("score-mode").getByRole("radio").first().click();
    await step.getByRole("radiogroup").nth(1).getByRole("radio").nth(4).click();
    await step.getByTestId("score-roll").getByRole("button").first().click();
    await expect(step.getByTestId("score-total-5")).toBeVisible();
    // The totals are rolled: Intelligence takes the highest one, so the bard's spellbook below
    // always has a spell level to enter (a low Intelligence caps it at none, which is right).
    const totals = await Promise.all(
      [0, 1, 2, 3, 4, 5].map(async (i) => {
        const hint = await step
          .getByTestId(`score-total-${i}`)
          .getAttribute("aria-label");
        return Number(/(\d+)$/.exec(hint ?? "")?.[1]);
      }),
    );
    const codes = ["dex", "int", "cha", "str", "wis", "con"];
    const best = totals.indexOf(Math.max(...totals));
    [codes[1], codes[best]] = [codes[best]!, codes[1]!];
    for (const [i, code] of codes.entries()) {
      await step.getByTestId(`score-total-${i}`).click();
      await step.getByTestId(`place-${code}`).click();
    }
    await audit("3-scores");
    await next.click();
    await step.getByTestId("hp-roll").getByRole("button").first().click();
    await expect(step.getByTestId("hp-roll").locator("tbody tr")).toHaveCount(
      6,
    );
    for (const [label, points] of [
      [tr("Escalade", "Climb walls"), "25"],
      [tr("Détection des bruits", "Detect noise"), "20"],
    ] as const)
      await step.getByLabel(label).fill(points);
    await audit("4-hp-skills");
    await next.click();
    // ADR 0037 step 14: a pick of the bard's groups with an extra slot, and Healing outside them
    // (priest only: the server's cost of 3), within any rolled Intelligence's budget.
    await expect(step.getByTestId("proficiency-pool-terms")).toBeVisible();
    // ADR 0037 steps 7 and 14: a weapon pick on the same step, on the server's weapon pool.
    await expect(step.getByTestId("weapon-pool-terms")).toBeVisible();
    await step
      .getByTestId("weapon-row-long_sword")
      .getByTestId("weapon-add")
      .click();
    await expect(
      step
        .getByTestId("weapon-row-long_sword")
        .getByTestId("weapon-row-status"),
    ).toHaveText(tr("Maîtrisée · 1 emplacement", "Proficient · 1 slot"));
    for (const name of [tr("Acrobatie", "Tumbling"), tr("Soins", "Healing")])
      await step
        .getByRole("button", {
          name: new RegExp(`^${tr("Ajouter", "Add")} ${name}`),
        })
        .click();
    await step
      .getByTestId("proficiency-pick-tumbling")
      .getByTestId("proficiency-more")
      .click();
    await expect(
      step
        .getByTestId("proficiency-pick-tumbling")
        .getByTestId("proficiency-pick-terms"),
    ).toContainText(tr("+1 au jet", "+1 to the check"));
    await audit("4b-proficiencies");
    if (lang === "fr") {
      // The step in the dark (the appearance follows the system): for review.
      await page.setViewportSize({ width: 1920, height: 1080 });
      await page.emulateMedia({ colorScheme: "dark" });
      await expect(page.locator("html")).toHaveClass(/dark/);
      await snap(page, "creation-4b-proficiencies-fr-dark-1920x1080");
      await page.emulateMedia({ colorScheme: "light" });
      await page.setViewportSize(sizes[0]!);
    }
    await next.click();
    // ADR 0030 step 20.7: the bard's starting spellbook at level 5 (PHB Table 32: 1st and 2nd
    // level), nothing pre-ticked; a spell of each level, the counts per level against Table 4.
    await expect(step).toHaveAttribute("data-step", "spellbook");
    await expect(step.getByTestId("spellbook-level-2")).toBeVisible();
    await expect(step.getByTestId("spellbook-picks")).toHaveCount(0);
    for (const id of ["sleep", "tashas_uncontrollable_hideous_laughter"])
      await step.getByTestId(`spellbook-add-${id}`).click();
    await expect(
      step.getByTestId("spellbook-picks").getByRole("listitem"),
    ).toHaveCount(2);
    await expect(
      step
        .getByTestId("spellbook-level-2")
        .getByTestId("spellbook-level-count"),
    ).toHaveText(/^1 /);
    await audit("4c-spellbook");
    await next.click();
    await step
      .getByLabel(tr("Nom", "Name"), { exact: true })
      .fill("Aldéric de la Tour-d'Auvergne-Lesparre");
    // The languages over the non-weapon slots (ADR 0037 step 6): eight with the five slots picked,
    // more than any rolled Intelligence gives; the server's refusal at the field, uncut. The
    // preview runs once an alignment and a sex are chosen; the languages are the proficiency
    // step's (ADR 0037 step 18), where the refusal shows.
    const grid = step.getByTestId("alignment-grid");
    await grid.getByRole("radio").nth(1).click();
    await step
      .getByRole("radiogroup", { name: tr("Sexe", "Sex") })
      .getByRole("radio")
      .nth(1)
      .click();
    await step
      .getByRole("button", {
        name: new RegExp(
          `^${tr("Revenir à l'étape précédente", "Back to the previous step")}`,
        ),
      })
      .click();
    await expect(step).toHaveAttribute("data-step", "spellbook");
    await step
      .getByRole("button", {
        name: new RegExp(
          `^${tr("Revenir à l'étape précédente", "Back to the previous step")}`,
        ),
      })
      .click();
    await expect(step).toHaveAttribute("data-step", "proficiencies");
    const chips = step.getByTestId("language-chips").getByRole("checkbox");
    for (let i = 0; i < 8; i++) await chips.nth(i).click();
    const languagesError = step
      .getByTestId("language-chips")
      .getByTestId("field-error");
    await expect(languagesError).toContainText(
      tr("Budget dépassé", "Over budget"),
    );
    await expectNoClippedText(
      step.getByTestId("language-chips"),
      `creation languages refusal, ${lang}`,
    );
    if (lang === "fr")
      for (const vp of sizes.filter((v) => v.width >= 1920)) {
        await page.setViewportSize(vp);
        await snap(
          page,
          `creation-4b-languages-refused-fr-${vp.width}x${vp.height}`,
        );
      }
    await page.setViewportSize(sizes[0]!);
    await next.click();
    await expect(step).toHaveAttribute("data-step", "spellbook");
    await next.click();
    await expect(step).toHaveAttribute("data-step", "identity");
    // The longest content: a refused alignment (its note; the preview then answers that refusal
    // first) and the three rolls' breakdowns.
    await grid.getByRole("radio").first().click();
    await expect(step.getByTestId("alignment-refusal")).toBeVisible();
    for (const f of ["height", "weight", "age"]) {
      await step.getByTestId(`details-roll-${f}`).click();
      await expect(
        step.getByTestId(`details-roll-${f}-breakdown`),
      ).toBeVisible();
    }
    await step
      .getByLabel(tr("Cheveux", "Hair"))
      .fill(tr("Auburn, tressés en couronne", "Auburn, braided in a crown"));
    await audit("5-identity");
    await grid.getByRole("radio").nth(1).click();
    // Back within the slots (the server refuses the eight), on the proficiency step: the review
    // gets its figures.
    await step
      .getByRole("button", {
        name: new RegExp(
          `^${tr("Revenir à l'étape précédente", "Back to the previous step")}`,
        ),
      })
      .click();
    await step
      .getByRole("button", {
        name: new RegExp(
          `^${tr("Revenir à l'étape précédente", "Back to the previous step")}`,
        ),
      })
      .click();
    for (let i = 0; i < 8; i++) await chips.nth(i).click();
    await expect(languagesError).toHaveCount(0);
    await next.click();
    await next.click();
    await next.click();
    await expect(step.getByTestId("review-thac0")).toHaveText(/^\d+$/);
    // The review lists the proficiencies the step picked and the summary beside it is gone: no
    // fact shows twice.
    await expect(step.getByTestId("review-weapon-picks")).toContainText(
      tr("Épée longue", "Long sword"),
    );
    await expect(step.getByTestId("review-proficiency-picks")).toContainText(
      tr("Soins", "Healing"),
    );
    await expect(page.getByTestId("creation-building")).toHaveCount(0);
    // The stepper does not repeat the choices either.
    await expect(
      page.getByTestId("creation-steps").getByText(/Aldéric/),
    ).toHaveCount(0);
    await audit("6-review");

    if (lang === "fr") {
      // A refusal opens its step: max HP 0 is out of range, shown on "Points de vie et talents".
      for (let i = 0; i < 4; i++)
        await step.getByRole("button", { name: /^Revenir/ }).click();
      await step.getByLabel("PV max").fill("0");
      for (let i = 0; i < 4; i++) await next.click();
      await step
        .getByRole("button", { name: "Créer ce personnage et le jouer" })
        .click();
      await expect(step).toHaveAttribute("data-step", "extras");
      await expect(step.getByTestId("field-error")).toBeVisible();
      await audit("refusal");
    }
  }
}

// The class step's multi-class and dual-class rows (ADR 0042 steps 1 and 4): three classes with a
// long XP, and a dual-class character refused at Créer (Intelligence 14 is under the 17 the new
// class needs), whose refusal opens the class step; never cut, French first, 1280 to 2560.
async function creationMultiStep(page: Page, size: Viewport) {
  await openDashboard(page);
  const sizes = sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ]);
  for (const lang of ["fr", "en"] as const) {
    await page.setViewportSize(sizes[0]!);
    await page.goto("/");
    await chooseLanguage(page, lang);
    await page.goto("/create");
    const tr = (fr: string, en: string) => (lang === "fr" ? fr : en);
    const step = page.getByTestId("creation-step");
    const next = step.getByRole("button", {
      name: tr("Passer à l'étape suivante", "Go to the next step"),
    });
    const audit = async (name: string) => {
      for (const vp of sizes) {
        await page.setViewportSize(vp);
        const where = `${name}, ${lang}, ${vp.width}x${vp.height}`;
        await expectNoClippedText(page.locator("body"), `creation ${where}`);
        await expectNoPageScroll(page);
        await snap(page, `creation-${name}-${lang}-${vp.width}x${vp.height}`);
      }
      await page.setViewportSize(sizes[0]!);
    };
    const kind = step.getByRole("radiogroup", {
      name: tr("Type de personnage", "Kind of character"),
    });
    const pick = async (
      row: number,
      label: string,
      cls: string,
      xp: string,
    ) => {
      const r = step.getByTestId(`class-row-${row}`);
      await r.getByLabel(label).selectOption(cls);
      await r.getByLabel(tr("XP", "XP"), { exact: true }).fill(xp);
    };
    await kind
      .getByRole("radio", { name: tr("Multiclassé", "Multi-class") })
      .click();
    await step.getByRole("button", { name: tr("Ajouter", "Add") }).click();
    await pick(0, tr("Classe 1", "Class 1"), "fighter", "1250000");
    await pick(1, tr("Classe 2", "Class 2"), "cleric", "1250000");
    await pick(2, tr("Classe 3", "Class 3"), "thief", "1250000");
    await audit("classes-multi");

    // Dual-class with Intelligence 14: refused at Créer, the refusal on the class step.
    await kind
      .getByRole("radio", { name: tr("Double classé", "Dual-class") })
      .click();
    await step
      .getByTestId("class-row-0")
      .getByLabel(tr("Classe quittée", "Class left"))
      .selectOption("fighter");
    await step
      .getByTestId("class-row-0")
      .getByLabel(tr("Quittée au niveau", "Left at level"))
      .fill("9");
    await step
      .getByTestId("class-row-1")
      .getByLabel(tr("Nouvelle classe", "New class"))
      .selectOption("mage");
    await step
      .getByTestId("class-row-1")
      .getByLabel("XP", { exact: true })
      .fill("20000");
    await next.click();
    await step
      .getByRole("radio", { name: tr("Elfe", "Elf"), exact: true })
      .click();
    await next.click();
    await step.getByRole("radio", { name: tr("Saisir", "Type") }).click();
    for (const [fr, en, score] of [
      ["FOR", "STR", "17"],
      ["DEX", "DEX", "16"],
      ["CON", "CON", "15"],
      ["INT", "INT", "14"],
      ["SAG", "WIS", "16"],
      ["CHA", "CHA", "10"],
    ])
      await step.getByLabel(tr(fr!, en!), { exact: true }).fill(score!);
    await next.click();
    await step.getByLabel(tr("PV max", "Max HP")).fill("60");
    await next.click();
    await expect(step).toHaveAttribute("data-step", "proficiencies");
    // The spellbook step appears once the draft names the class that carries it.
    await expect(
      page.getByRole("navigation", { name: tr("Étapes", "Steps") }),
    ).toContainText(tr("Grimoire", "Spellbook"));
    await next.click();
    await expect(step).toHaveAttribute("data-step", "spellbook");
    await next.click();
    await step.getByLabel(tr("Nom", "Name"), { exact: true }).fill("Vael");
    await step
      .getByTestId("alignment-grid")
      .getByRole("radio", { name: /^[^:]+$/ })
      .first()
      .click();
    await step
      .getByRole("radiogroup", { name: tr("Sexe", "Sex") })
      .getByRole("radio")
      .first()
      .click();
    await next.click();
    await step
      .getByRole("button", {
        name: tr(
          "Créer ce personnage et le jouer",
          "Create this character and play it",
        ),
      })
      .click();
    await expect(step).toHaveAttribute("data-step", "class");
    await expect(step.getByTestId("field-error").first()).toBeVisible();
    await audit("classes-dual-refused");
  }
}

// ADR 0030 step 7 and ADR 0022 step 1, judged in French (ADR 0027 step 2): the level-up prompt on
// the character card, at its widest (a hit die to roll, a second level owed, the manual entry
// open), is never cut from 1280 to 2560, and each button says one word (ADR 0027 step 3).
async function levelUpPromptStep(page: Page, size: Viewport) {
  await openDashboard(page);
  const call = (method: string, url: string, body: unknown) =>
    apiCall(page, method, url, body);
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
    }),
  ).toBe(true);
  expect(await call("POST", "/api/xp", { base: 2500, bonus: false })).toBe(
    true,
  );
  await reloadSettled(page);
  await chooseLanguage(page, "fr");
  const prompt = page.getByTestId("level-up");
  await expect(prompt.getByTestId("level-up-title")).toHaveText(
    "Niveau 2 atteint",
  );
  await expect(prompt.getByTestId("level-up-more")).toBeVisible();
  for (const vp of sizesAt(size, VIEWPORTS)) {
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
}

// ADR 0037 step 15 and ADR 0022 step 1, judged in French (ADR 0027 step 2): once level 2 is taken,
// what it grants (a bard's 15 points) under the prompt, and what is left to spend (a level 1 bard's
// slots and points), the class line's tag and its popover, each row with its one-word link, never
// cut from 1280 to 2560. Screenshots of both at every viewport.
async function levelUpGrantsStep(page: Page, size: Viewport) {
  await openDashboard(page);
  const call = (method: string, url: string, body: unknown) =>
    apiCall(page, method, url, body);
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
    }),
  ).toBe(true);
  expect(await call("POST", "/api/xp", { base: 2500, bonus: false })).toBe(
    true,
  );
  await reloadSettled(page);
  await chooseLanguage(page, "fr");
  await page
    .getByTestId("level-up")
    .getByRole("button", { name: "Lancer le dé de vie du niveau 2 (d6)" })
    .click();
  const grants = page.getByTestId("level-up-grants");
  await expect(grants).toContainText("Le niveau 2 apporte");
  await expect(grants).toContainText("+15 points de talent");
  const tag = page.getByTestId("to-spend-tag");
  const popover = page.getByTestId("level-up-to-spend");
  for (const vp of sizesAt(size, VIEWPORTS)) {
    await page.setViewportSize(vp);
    const where = `fr, ${vp.width}x${vp.height}`;
    await expectNoClippedText(grants, `level-up grants, ${where}`);
    await expectWholeWords(grants, `level-up grants, ${where}`);
    await expectNoClippedText(tag, `to-spend tag, ${where}`);
    await expectMinTarget(tag, `to-spend tag, ${where}`);
    await snap(page, `level-up-grants-fr-${vp.width}x${vp.height}`);
    await tag.click();
    await expect(popover).toContainText("35 points de talent");
    await expectNoClippedText(popover, `to-spend popover, ${where}`);
    await expectWholeWords(popover, `to-spend popover, ${where}`);
    await snap(page, `level-up-to-spend-fr-${vp.width}x${vp.height}`);
    await page.keyboard.press("Escape");
    await expect(popover).toHaveCount(0);
  }
}

// ADR 0030 step 20.8 and ADR 0022 step 1, judged in French (ADR 0027 step 2): once a mage takes
// level 3, its grants (a non-weapon slot and the free 2nd-level spell, each with its link) and the
// to-spend popover listing the free spell, never cut from 1280 to 2560; the character
// card does not scroll for them from 1440x900 up (ADR 0022 step 2.8), with a three-line name.
// Screenshots of both at every viewport.
async function levelUpFreeSpellStep(page: Page, size: Viewport) {
  await openDashboard(page);
  const call = (method: string, url: string, body: unknown) =>
    apiCall(page, method, url, body);
  expect(
    await call("POST", "/api/characters", {
      name: "Aldéric de la Tour-d'Auvergne-Lesparre",
      class: "mage",
      raceCode: "human",
      details: { alignment: "neutral_good", sex: "male" },
      str: 10,
      dex: 15,
      con: 12,
      int: 14,
      wis: 9,
      cha: 16,
      hpMax: 4,
      xp: 0,
    }),
  ).toBe(true);
  expect(await call("POST", "/api/xp", { base: 5000, bonus: false })).toBe(
    true,
  );
  await reloadSettled(page);
  await chooseLanguage(page, "fr");
  const prompt = page.getByTestId("level-up");
  await prompt
    .getByRole("button", { name: "Lancer le dé de vie du niveau 2 (d4)" })
    .click();
  await prompt
    .getByRole("button", { name: "Lancer le dé de vie du niveau 3 (d4)" })
    .click();
  const grants = page.getByTestId("level-up-grants");
  await expect(grants).toContainText("Le niveau 3 apporte");
  await expect(grants).toContainText("+1 sort gratuit, niv. 2");
  const tag = page.getByTestId("to-spend-tag");
  const popover = page.getByTestId("level-up-to-spend");
  const character = page.getByTestId("character");
  for (const vp of sizesAt(size, VIEWPORTS)) {
    await page.setViewportSize(vp);
    const where = `fr, ${vp.width}x${vp.height}`;
    await expectNoClippedText(grants, `free spell grants, ${where}`);
    await expectWholeWords(grants, `free spell grants, ${where}`);
    await snap(page, `level-up-free-spell-fr-${vp.width}x${vp.height}`);
    if (vp.width >= 1440)
      await expectNoScroll(
        character,
        `character card with the free spell, ${where}`,
      );
    await tag.click();
    await expect(popover).toContainText("1 sort gratuit, niv. 2");
    await expectNoClippedText(popover, `free spell to-spend, ${where}`);
    await expectWholeWords(popover, `free spell to-spend, ${where}`);
    await snap(
      page,
      `level-up-free-spell-to-spend-fr-${vp.width}x${vp.height}`,
    );
    await page.keyboard.press("Escape");
    await expect(popover).toHaveCount(0);
  }
}

// ADR 0030 step 9 through the class's features (ADR 0040 step 3), on the creation route's "Points
// de vie et talents" step at 1920x1080 and 2560x1440: a human fighter of Strength 18 is offered the
// 18/xx field, a human mage of Strength 18 is not. The form has no rule of its own: the field
// follows the draft's exceptionalStrength (the cases are Go's, composition draft_exceptional_test).
async function creationExtrasStep(page: Page, size: Viewport) {
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  const step = page.getByTestId("creation-step");
  const next = () =>
    step.getByRole("button", { name: "Passer à l'étape suivante" }).click();
  for (const [cls, name, offered] of [
    ["Guerrier", "fighter", true],
    ["Magicien", "mage", false],
  ] as const) {
    await page.setViewportSize({ width: 1920, height: 1080 });
    await page.goto("/create");
    await step.getByRole("radio", { name: cls, exact: true }).click();
    await next();
    await step.getByRole("radio", { name: "Humain", exact: true }).click();
    await next();
    await step.getByRole("radio", { name: "Saisir" }).click();
    for (const [ability, score] of Object.entries({
      FOR: "18",
      DEX: "14",
      CON: "15",
      INT: "15",
      SAG: "10",
      CHA: "9",
    }))
      await step.getByLabel(ability, { exact: true }).fill(score);
    await next();
    // The draft answered for the entries with Strength 18: the field is there or not from then on.
    const answered = page.waitForResponse(
      (r) => r.url().endsWith("/api/characters/draft") && r.ok(),
    );
    await step.getByLabel("PV max").fill("8");
    const draft = (await (await answered).json()) as {
      exceptionalStrength: boolean;
    };
    expect(draft.exceptionalStrength, name).toBe(offered);
    const field = step.getByLabel("Pourcentage (18/xx)");
    await expect(field).toHaveCount(offered ? 1 : 0);
    for (const vp of sizesAt(size, [
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ])) {
      await page.setViewportSize(vp);
      const where = `${name} extras, ${vp.width}x${vp.height}`;
      await expectNoClippedText(step, where);
      await snap(
        page,
        `creation-extras-exceptional-${name}-fr-${vp.width}x${vp.height}`,
      );
    }
  }
}

// ADR 0030 steps 9, 11 and 12 in French at 1280x800 and 2560x1440: the 18/50 Strength tile, the
// attack card's rate, "spécialisée" tag and point-blank checkbox (the heavy crossbow is the
// specialization pick, ADR 0037 step 9; the long sword a plain pick), and the weapon editor's Type
// d’arme control, none of them cut. Screenshot of the weapon editor at both sizes.
async function fighterStep(page: Page, size: Viewport) {
  await page.setViewportSize(size);
  await openDashboard(page);
  const call = (method: string, url: string, body: unknown) =>
    apiCall(page, method, url, body);
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
  // The new character is the current one: a request the page had in flight answers "character
  // changed" and reloads it (api/http.ts). Reload now and wait for the new sheet, so no reload
  // lands in the middle of the calls below.
  await reloadSettled(page);
  await expect(page.getByText("Borin Gueule-de-Fer").first()).toBeVisible();
  // Weapon pieces (ADR 0033 step 1), equipped to be held (step 6).
  for (const [id, w] of [
    [
      "long_sword",
      {
        name: { en: "Long Sword", fr: "Épée longue" },
        category: "weapon",
        behaviour: "stock",
        count: 1,
        damageSm: "1d8",
        damageL: "1d12",
        weaponKind: "melee",
        weaponType: "long_sword",
      },
    ],
    [
      "heavy_crossbow",
      {
        name: { en: "Heavy Crossbow", fr: "Arbalète lourde" },
        category: "weapon",
        behaviour: "stock",
        count: 1,
        damageSm: "1d4",
        damageL: "1d6",
        weaponKind: "missile",
        fires: "quarrel",
        weaponType: "heavy_crossbow",
      },
    ],
  ] as const) {
    expect(await call("POST", "/api/items", w), w.name.en).toBe(true);
    expect(
      await call("POST", `/api/items/${id}/state`, { state: "wielded" }),
    ).toBe(true);
  }
  // One specialization: the heavy crossbow's (2 slots), the long sword a plain pick.
  expect(
    await call("PUT", "/api/proficiencies", {
      weapon: [
        { type: "long_sword", slots: 1 },
        { type: "heavy_crossbow", slots: 2 },
      ],
    }),
  ).toBe(true);
  await reloadSettled(page);
  await expect(page.getByTestId("str-value")).toHaveText("18/50");

  await openWorkspace(page, "Act");
  await openWeapons(page);
  const f = page.getByRole("form", { name: "Attaque" });
  for (const weapon of ["Long Sword", "Heavy Crossbow"]) {
    await chooseLanguage(page, "en");
    await chooseWeapon(page, weapon);
    await chooseLanguage(page, "fr");
    await expect(f.getByTestId("tile-rate")).toContainText("att.");
    if (weapon === "Heavy Crossbow") {
      await expect(f.getByTestId("tile-specialized")).toHaveText("spécialisée");
      await expect(f.getByTestId("point-blank")).toBeVisible();
    } else await expect(f.getByTestId("tile-specialized")).toHaveCount(0);
    for (const vp of sizesAt(size, [
      { width: 1280, height: 800 },
      { width: 2560, height: 1440 },
    ])) {
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
    await page.setViewportSize(size);
  }

  await chooseLanguage(page, "en");
  await openWorkspace(page, "Inventory");
  await chooseLanguage(page, "fr");
  for (const vp of sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 2560, height: 1440 },
  ])) {
    await page.setViewportSize(vp);
    await page
      .getByTestId("item-long_sword")
      .getByRole("button", { name: "Modifier Épée longue" })
      .click();
    const wf = page.getByTestId("item-form");
    await expect(wf.getByLabel("Type d’arme")).toBeVisible();
    await expect(wf.getByLabel("Spécialisée")).toHaveCount(0);
    const where = `weapon editor, fr, ${vp.width}x${vp.height}`;
    await expectNoClippedText(wf, where);
    await snap(page, `fighter-weapon-editor-fr-${vp.width}x${vp.height}`);
    await wf.getByRole("button", { name: "Annuler" }).click();
    await expect(wf).toBeHidden();
  }
}

// The mage's Learn control (ADR 0030 step 20) in the lookup's detail: the button, then the
// roll's answer (and "in the spellbook" once learned), uncut in French at 1280x800 and 2560x1440.
async function mageLearnStep(page: Page, size: Viewport) {
  await page.setViewportSize(size);
  await openDashboard(page);
  const created = await apiCall(page, "POST", "/api/characters", {
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
  });

  expect(created).toBe(true);
  await reloadSettled(page);
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
  await expect(learn.getByTestId("add-spell")).toHaveText(
    "Ajouter au grimoire",
  );
  for (const vp of sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 2560, height: 1440 },
  ])) {
    await page.setViewportSize(vp);
    await expectNoClippedText(
      learn,
      `Learn and Add buttons, fr, ${vp.width}x${vp.height}`,
    );
    await snap(page, `mage-learn-add-buttons-fr-${vp.width}x${vp.height}`);
  }
  await page.setViewportSize(size);
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
  for (const vp of sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 2560, height: 1440 },
  ])) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expectNoClippedText(learn, `Learn failure, fr, ${where}`);
    await snap(page, `mage-learn-failed-fr-${where}`);
  }
  await page.unroute("**/api/character/spellbook/learn");
  await page.setViewportSize(size);
  const answered = page.waitForResponse("**/api/character/spellbook/learn");
  await learn.getByRole("button", { name: /^Apprendre/ }).click();
  const real = (await (await answered).json()) as { roll: number };
  await expect(learn.getByTestId("learn-result")).toContainText(
    String(real.roll),
  );
  for (const vp of sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 2560, height: 1440 },
  ])) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expectNoClippedText(learn, `Learn answer, fr, ${where}`);
    await expectNoClippedText(
      page.getByTestId("character"),
      `mage character card, fr, ${where}`,
    );
    await snap(page, `mage-learn-fr-${where}`);
  }

  // Ajouter (ADR 0030 step 20.6): the add's state key reads the spellbook again, so the detail
  // shows the spell in the book (the refusals and the no-die rule are Go tests, learnsvc).
  await page.setViewportSize(size);
  await page
    .getByRole("combobox", { name: "Rechercher des sorts..." })
    .fill("Alarm");
  await expect(detail.getByRole("heading")).toHaveText("Alarme");
  await learn.getByTestId("add-spell").click();
  await expect(learn).toContainText("Dans le grimoire");
  await expect(learn.getByTestId("add-spell")).toHaveCount(0);
  await snap(page, "mage-add-known-fr-1280x800");

  // The spell_add entry worded in French on top of the journal; its undo reads the spellbook again,
  // so the detail drops "Dans le grimoire" and offers Ajouter (what the undo writes is a Go test,
  // composition).
  await expectUndoOn(page, "Ajouté au grimoire : Alarme (sans jet)");
  await expect(page.getByTestId("undo")).toBeEnabled();
  await page.evaluate(() => (document.activeElement as HTMLElement)?.blur());
  await page.keyboard.press("ControlOrMeta+z");
  await expect(learn).not.toContainText("Dans le grimoire");
  await expect(learn.getByTestId("add-spell")).toHaveText(
    "Ajouter au grimoire",
  );
  await page
    .getByRole("navigation", { name: "Activités" })
    .getByRole("button", { name: "Agir", exact: true })
    .click();
  await expect(
    logEntry(page, "Ajouté au grimoire : Alarme (sans jet)"),
  ).toHaveAttribute("data-status", "voided");
  await snap(page, "mage-add-undone-journal-fr-1280x800");
}

// The lookup says why a spell cannot be written in the spellbook (ADR 0030 step 20.1), in the
// server's own words: a level-5 mage with Intelligence 9 (highest spell level 4) sees the reason
// instead of Learn and Add on a level-5 spell, and the buttons on a level-4 one. The reason
// matches the server's refusal (a unit test compares the codes; the refusal is a Go test).
async function mageLimitStep(page: Page, size: Viewport) {
  await page.setViewportSize(size);
  await openDashboard(page);
  const created = await apiCall(page, "POST", "/api/characters", {
    name: "Elminster",
    class: "mage",
    raceCode: "human",
    details: { alignment: "neutral_good", sex: "male" },
    str: 9,
    dex: 12,
    con: 11,
    int: 9,
    wis: 12,
    cha: 10,
    hpMax: 4,
    xp: 20000,
  });
  expect(created).toBe(true);
  await reloadSettled(page);
  await openWorkspace(page, "Spells");
  const classes = page.getByRole("group", {
    name: /^(Spell list|Liste de sorts)$/,
  });
  const detail = page.getByTestId("spell-detail");
  const learn = detail.getByTestId("learn-spell");
  for (const lang of ["en", "fr"] as const) {
    await chooseLanguage(page, lang);
    await classes.getByRole("button", { name: /^(Wizard|Magicien)$/ }).click();
    const box = page.getByRole("combobox", {
      name: /^(Search spells\.\.\.|Rechercher des sorts\.\.\.)$/,
    });
    await box.fill("magic jar");
    await expect(page.getByTestId("spell-row").first()).toHaveAttribute(
      "data-id",
      "magic_jar",
    );
    await expect(learn.getByTestId("learn-refused")).toHaveAttribute(
      "data-code",
      "spell_level_above_intelligence",
    );
    await expect(learn.getByTestId("learn-refused")).toHaveText(
      lang === "en"
        ? "This Intelligence limits spells to level 4."
        : "Cette Intelligence limite les sorts au niveau 4.",
    );
    await expect(learn.getByRole("button")).toHaveCount(0);
    await expect(learn.getByTestId("add-spell")).toHaveCount(0);
    for (const vp of sizesAt(size, [
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ])) {
      await page.setViewportSize(vp);
      await expectNoClippedText(
        learn,
        `spellbook refusal, ${lang}, ${vp.width}x${vp.height}`,
      );
      await snap(page, `mage-lookup-reason-${lang}-${vp.width}x${vp.height}`);
    }
    await page.setViewportSize(size);
    // A spell the Intelligence allows keeps its buttons.
    await box.fill("alarm");
    await expect(page.getByTestId("spell-row").first()).toHaveAttribute(
      "data-id",
      "alarm",
    );
    await expect(learn.getByTestId("add-spell")).toBeVisible();
    await expect(learn.getByTestId("learn-refused")).toHaveCount(0);
  }
}

// The cleric's screens (ADR 0030 steps 14 and 16) uncut in French at 1280x800 and 2560x1440: the
// Turning tab (Table 61 at the realized level, the longest outcome labels), the turning result card
// and its journal row, and a cast's spell failure on the result card. Wisdom 9 gives a spell
// failure chance; the failed cast is a fixed response (the dice are real; the failure itself is a
// Go test, castsvc).
/**
 * Every Act tab the character has, at each of the walk's sizes: the journal's top edge is the same on
 * all of them (ADR 0027 step 6.1) and a tab taller than the band scrolls inside its panel, nothing cut.
 */
async function expectJournalTopSteadyOnEveryTab(
  page: Page,
  size: Viewport,
  who: string,
) {
  const actions = page.getByRole("region", { name: /^(Actions|Action)$/ });
  const names = (await actions.getByRole("tab").allInnerTexts()).map((n) =>
    n.replace(/\s*\d+$/, "").trim(),
  );
  for (const vp of sizesAt(size, VIEWPORTS)) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    let top: number | null = null;
    for (const name of names) {
      await actions.getByRole("tab", { name, exact: false }).first().click();
      await expect(actions.getByRole("tabpanel")).not.toBeEmpty();
      const now = (await page.getByTestId("journal-slot").boundingBox())!.y;
      if (top === null) top = now;
      expect(now, `${who}: journal top on ${name}, ${where}`).toBeCloseTo(
        top,
        0,
      );
      await expectNoPageScroll(page);
      await expectNoClippedText(actions, `${who}: ${name} tab, ${where}`);
    }
    await actions.getByRole("tab").first().click();
  }
}

async function clericStep(page: Page, size: Viewport) {
  await page.setViewportSize(size);
  await openDashboard(page);
  let created = await apiCall(page, "POST", "/api/characters", {
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
  });
  if (created === true)
    created = await apiCall(page, "PUT", "/api/spells/prepared", {
      spells: ["bless"],
    });

  expect(created).toBe(true);
  await reloadSettled(page);
  await openWorkspace(page, "Act");
  await chooseLanguage(page, "fr");
  await expectJournalTopSteadyOnEveryTab(page, size, "cleric");
  await page.getByRole("tab", { name: "Renvoi" }).click();
  const turning = page.getByTestId("turn-undead");
  await expect(turning.getByTestId("turn-cell-skeleton")).toBeVisible();
  for (const vp of sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 2560, height: 1440 },
  ])) {
    await page.setViewportSize(vp);
    await expectNoClippedText(
      turning,
      `turning panel, fr, ${vp.width}x${vp.height}`,
    );
  }
  await page.setViewportSize(size);
  await turning.getByRole("button", { name: "Repousser : Squelette" }).click();
  const turned = page.getByTestId("turn-result");
  await expect(turned).toContainText("Clerc de niveau 1");
  for (const vp of sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 2560, height: 1440 },
  ])) {
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

  await page.setViewportSize(size);
  await page.route("**/api/spells/*/cast", (route) =>
    route.fulfill({
      json: {
        action: "cast",
        spellId: "bless",
        spell: "Bless",
        level: 1,
        casterLevel: 1,
        slotIndex: 0,
        untracked: [],
        failure: { d100: 12, chance: 20, failed: true },
        logEntryId: 999_999,
      },
    }),
  );
  await page.getByRole("tab", { name: "Sorts" }).click();
  await page.getByRole("button", { name: "Lancer Bless" }).click();
  const failure = page.getByTestId("cast-failure");
  await expect(failure).toHaveAttribute("data-failed", "true");
  for (const vp of sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 2560, height: 1440 },
  ])) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expectNoClippedText(failure, `spell failure, fr, ${where}`);
    await snap(page, `cleric-spell-failure-fr-${where}`);
  }
  await page.unroute("**/api/spells/*/cast");
}

// The thief's screens (ADR 0030 steps 1, 17 and 19) uncut in French at 1280x800 and 2560x1440: the
// creation form's skill points and armour fields, the Skills tab's thieving skills with the
// budget warnings (70 points at 1st, 40 in one skill: both breached), the Spend editor's
// over-budget refusal (ADR 0037 step 10) and the attack card's backstab option.
async function thiefStep(page: Page, size: Viewport) {
  await page.setViewportSize(size);
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  // The creation route's "Points de vie et talents" step of a thief; leaving it accepts the
  // browser's beforeunload question.
  page.on("dialog", (d) => void d.accept());
  await page.goto("/create");
  const step = page.getByTestId("creation-step");
  const next = step.getByRole("button", { name: "Passer à l'étape suivante" });
  await step.getByRole("radio", { name: "Voleur", exact: true }).click();
  await next.click();
  await step.getByRole("radio", { name: "Petite-gens", exact: true }).click();
  await next.click();
  await step.getByRole("radio", { name: "Saisir" }).click();
  for (const a of ["FOR", "DEX", "CON", "INT", "SAG", "CHA"])
    await step.getByLabel(a, { exact: true }).fill(a === "DEX" ? "8" : "12");
  // The class cards show each class's ability minimums, the server flags the ones the scores miss
  // (the thief asks for Dexterity 9) and the flag follows a corrected score.
  const back = step.getByRole("button", {
    name: /^Revenir à l'étape précédente/,
  });
  await back.click();
  await back.click();
  const thiefMinimums = step.getByTestId("minimums-thief");
  await expect(thiefMinimums).toContainText("DEX 9");
  await expect(thiefMinimums.locator("[data-unmet]")).toHaveCount(1);
  await expect(
    step.getByTestId("minimums-bard").locator("[data-unmet]"),
  ).toHaveCount(3); // Dex 8, Int 12 and Cha 12 against 12, 13 and 15
  for (const vp of sizesAt(size, [
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ])) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expectNoClippedText(step, `thief class minimums, fr, ${where}`);
    await snap(page, `thief-class-minimums-fr-${where}`);
  }
  await page.setViewportSize(size);
  await next.click();
  await next.click();
  await step.getByLabel("DEX", { exact: true }).fill("12");
  await back.click();
  await back.click();
  await expect(thiefMinimums.locator("[data-unmet]")).toHaveCount(0);
  await next.click();
  await next.click();
  await next.click();
  // 1st level: 60 points in all, 30 in one skill; the budget shows what is left as points are entered.
  const entry = step.getByTestId("thief-skills-entry");
  const budget = entry.getByTestId("skill-budget");
  await expect(budget).toContainText("Budget du niveau 1 : 60 points");
  await expect(budget).toContainText("Restants : 60");
  // The budget reads the class and level only: a Strength of 2, which the scores step lets
  // through, leaves it shown.
  await back.click();
  const badScore = page.waitForResponse(
    (r) =>
      r.url().endsWith("/api/characters/draft") &&
      r.request().postData()?.includes('"str":2,') === true,
  );
  await step.getByLabel("FOR", { exact: true }).fill("2");
  await next.click();
  await badScore;
  await expect(budget).toContainText("Budget du niveau 1 : 60 points");
  await back.click();
  await step.getByLabel("FOR", { exact: true }).fill("12");
  await next.click();
  await entry.getByLabel("Crochetage").fill("25");
  await expect(budget).toContainText("Restants : 35");
  await entry.getByLabel("Crochetage").fill("");
  await expect(budget).toContainText("Restants : 60");
  // Over budget: 90 points in a 60-point level shows a negative balance in red.
  for (const skill of ["Crochetage", "Escalade", "Vol à la tire"])
    await entry.getByLabel(skill).fill("30");
  await expect(budget).toContainText("Restants : -30");
  await expect(budget.locator("[data-over]")).toHaveCount(1);
  await snap(page, "thief-form-over-budget-fr");
  // The server refuses to create a thief over the budget (ADR 0037 step 10): the draft's answer
  // lists the refusal, shown once the draft is seen past the proficiency step, and back on this
  // step the message shows under the points.
  await step.getByLabel("PV max").fill("5");
  const refused = page.waitForResponse(
    async (r) =>
      r.url().endsWith("/api/characters/draft") &&
      (await r.text()).includes('"over_budget"'),
  );
  await next.click();
  await next.click();
  await refused;
  await back.click();
  await back.click();
  const overBudget = step.getByText("Budget dépassé : 90 pour 60 disponibles.");
  await expect(overBudget).toBeVisible();
  await expectNoClippedText(entry, "thief creation over budget, fr, 1280x800");
  await snap(page, "thief-form-refused-over-budget-fr");
  for (const skill of ["Crochetage", "Escalade", "Vol à la tire"])
    await entry.getByLabel(skill).fill("");
  await expect(budget).toContainText("Restants : 60");
  // The figures follow the draft's class: a fighter has no budget, back to the thief it is again 60.
  const goBack = async () => {
    for (let i = 0; i < 3; i++) await back.click();
  };
  const goOn = async () => {
    for (let i = 0; i < 3; i++) await next.click();
  };
  await goBack();
  await step.getByRole("radio", { name: "Guerrier", exact: true }).click();
  await goOn();
  await expect(step.getByTestId("skill-budget")).toHaveCount(0);
  await goBack();
  await step.getByRole("radio", { name: "Voleur", exact: true }).click();
  await goOn();
  await expect(step.getByTestId("skill-budget")).toContainText("Restants : 60");
  for (const vp of sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ])) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expectNoClippedText(entry, `thief creation fields, fr, ${where}`);
    await snap(page, `thief-form-fr-${where}`);
  }
  await page.goto("/");
  await expect(page.getByTestId("character")).toBeVisible();

  await page.setViewportSize(size);
  const call = (method: string, url: string, body: unknown) =>
    apiCall(page, method, url, body);
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
      skillPoints: { find_remove_traps: 30, hide_in_shadows: 30 },
    }),
  ).toBe(true);
  // A character is never created or saved over its budget (ADR 0037 step 10): only imported data
  // is. The tab's two reads answer as for 40 and 30 points stored, the warnings and the negative
  // balance the layout must hold; the editor's preview and save stay the server's.
  const imported = { find_remove_traps: 40, hide_in_shadows: 30 };
  await page.route("**/api/character", async (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    const res = await route.fetch();
    const body = await res.json();
    if (body.classes?.[0]?.class === "thief")
      body.derived.skillSets[0].warnings = [
        { code: "skill_points_total", points: 70, limit: 60 },
        {
          code: "skill_points_per_skill",
          skill: "find_remove_traps",
          points: imported.find_remove_traps,
          limit: 30,
        },
      ];
    await route.fulfill({ response: res, json: body });
  });
  await page.route("**/api/skills", async (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    const res = await route.fetch();
    const body = await res.json();
    for (const set of body.skillSets)
      if (set.budget) Object.assign(set.budget, { spent: 70, left: -10 });
    await route.fulfill({ response: res, json: body });
  });
  // The skills read the armour worn (ADR 0033 step 14): a studded leather, the longest column name.
  expect(
    await call("POST", "/api/items", {
      name: { en: "Studded leather", fr: "Cuir clouté" },
      category: "armour",
      behaviour: "stock",
      count: 1,
      baseAc: 7,
      skillColumn: "padded_studded",
      armorRow: "studded_leather",
    }),
  ).toBe(true);
  expect(
    await call("POST", "/api/items/studded_leather/state", { state: "worn" }),
  ).toBe(true);
  expect(
    await call("POST", "/api/items", {
      name: { en: "Dagger", fr: "Dague" },
      category: "weapon",
      behaviour: "stock",
      count: 1,
      damageSm: "1d4",
      damageL: "1d3",
      weaponKind: "melee",
      weaponType: "dagger",
    }),
  ).toBe(true);
  expect(
    await call("POST", "/api/items/dagger/state", { state: "wielded" }),
  ).toBe(true);
  await reloadSettled(page);
  await chooseLanguage(page, "en");
  await openWorkspace(page, "Act");
  await chooseLanguage(page, "fr");
  await page.getByRole("tab", { name: "Compétences" }).click();
  const skills = page.getByTestId("skills-blocks");
  for (const vp of sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 2560, height: 1440 },
  ])) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expectNoClippedText(skills, `thieving skills, fr, ${where}`);
    // The skill armour line (its longest column name) sits uncut in the block.
    const armour = page.getByTestId("skill-armor");
    await expect(armour).toHaveText(
      "Armure des talents : Matelassée ou cuir clouté",
    );
    await expectNoClippedText(armour, `thief skill armour, fr, ${where}`);
    await snap(page, `thief-skills-fr-${where}`);
  }

  // The budget, each skill's terms and the Spend editor with its live preview, uncut in French
  // (ADR 0027 step 2): the server's figures, nothing computed here.
  await openSpend(page, "skillPoints");
  const editor = page.getByTestId("thief-points-editor");
  await expect(editor.getByTestId("skill-points-available")).toContainText(
    "Disponibles : 60 + 30 × 0 = 60",
  );
  // Over budget (70 of 60): left is negative, in the warning colour, no refusal.
  await expect(editor.getByTestId("skill-points-left")).toHaveAttribute(
    "data-over",
    "true",
  );
  // Above the range the server refuses the preview: its field error shows under the field, the
  // figures are marked out of date, and Save reports the same field.
  await editor.getByTestId("thief-points-open_locks").fill("96");
  await expect(editor.getByRole("alert")).toContainText("de 0 à 95");
  await expect(editor.getByTestId("skill-preview-stale")).toBeVisible();
  // Save's answer lands before the next keystroke (the alert above is already the preview's).
  const saved = page.waitForResponse(
    (r) => r.url().endsWith("/api/skills/points") && r.status() === 400,
  );
  await editor.getByTestId("thief-points-save").click();
  await saved;
  await expect(editor.getByRole("alert")).toContainText("de 0 à 95");
  await expect(editor).toBeVisible();
  // Over the budget (the 60 stored, 5 more) the preview is refused, its message under the editor.
  await editor.getByTestId("thief-points-open_locks").fill("5");
  await expect(editor.getByRole("alert")).toHaveText(
    "Budget dépassé : 65 pour 60 disponibles.",
  );
  await expect(editor.getByTestId("skill-preview-stale")).toBeVisible();
  // A percentage capped at 95 can't be reached within a 1st-level budget: the preview answers the
  // stored points with Open Locks at 112 % raw, so the cap tag stands in the layout beside Save's
  // refusal (the server's, under the editor).
  await page.route("**/api/skills/points/preview", async (route) => {
    const res = await route.fetch({
      postData: JSON.stringify({
        set: "thief",
        points: { find_remove_traps: 30, hide_in_shadows: 30 },
      }),
    });
    const body = await res.json();
    for (const sk of body.skillSets[0].skills)
      if (sk.id === "open_locks")
        Object.assign(sk, { points: 6, rawPercent: 112, percent: 95 });
    await route.fulfill({ response: res, json: body });
  });
  await editor.getByTestId("thief-points-open_locks").fill("6");
  await expect(editor.getByTestId("skill-limit-open_locks")).toContainText(
    "plafonné à 95 %",
  );
  await editor.getByTestId("thief-points-save").click();
  await expect(editor.getByRole("alert")).toHaveText(
    "Budget dépassé : 66 pour 60 disponibles.",
  );
  for (const vp of sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ])) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expectNoClippedText(editor, `skill points editor, fr, ${where}`);
    await expectWholeWords(editor, `skill points editor, fr, ${where}`);
    await editor.getByTestId("skill-limit-open_locks").scrollIntoViewIfNeeded();
    await snap(page, `thief-points-editor-preview-fr-${where}`);
  }
  await editor.getByTestId("thief-points-cancel").click();
  await page.unroute("**/api/skills/points/preview");

  await page.setViewportSize(size);
  await page.getByRole("tab", { name: "Armes" }).click();
  const f = page.getByRole("form", { name: "Attaque" });
  await f.getByTestId("backstab").check();
  for (const vp of sizesAt(size, [
    { width: 1280, height: 800 },
    { width: 2560, height: 1440 },
  ])) {
    await page.setViewportSize(vp);
    const where = `${vp.width}x${vp.height}`;
    await expectNoClippedText(f, `backstab attack card, fr, ${where}`);
    await snap(page, `thief-backstab-card-fr-${where}`);
  }
}

// The version badge (ADR 0024 step 1.4): it sits inside a box the screen already has, never in a band
// of its own: the dashboard's dice footer (right end), a header card (roster, creation), the login
// card. No page scroll or cut text at 1920x1080 and 2560x1440. The e2e API is a local build, so the
// real answer shows the dev pill and a version with its commit; staging and production are the
// health answers given here. Screenshots in French.
async function versionBadgeStep(
  page: Page,
  size: Viewport,
  _stack: Stack,
  context: BrowserContext,
) {
  // Judged at 1920x1080 and 2560x1440 only (the creation page at 1280x800 reaches past the window).
  if (size.width < 1920) return;
  const sizes = sizesAt(size, [
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ]);
  const badge = page.getByTestId("version-badge");
  const expectBadgeIn = async (
    host: Locator,
    where: string,
    end: "right-end" | "anywhere" = "anywhere",
  ) => {
    await expect(badge, where).toBeVisible();
    await expectNoPageScroll(page);
    await expectNoClippedText(badge, where);
    const b = (await badge.boundingBox())!;
    const h = (await host.boundingBox())!;
    expect(b.x, `${where}: inside the box, left`).toBeGreaterThanOrEqual(h.x);
    expect(b.y, `${where}: inside the box, top`).toBeGreaterThanOrEqual(h.y);
    expect(
      b.x + b.width,
      `${where}: inside the box, right`,
    ).toBeLessThanOrEqual(h.x + h.width);
    expect(
      b.y + b.height,
      `${where}: inside the box, bottom`,
    ).toBeLessThanOrEqual(h.y + h.height);
    if (end === "right-end")
      // Aligned with the footer's content: at its right end, centred on its line.
      expect(
        h.x + h.width - (b.x + b.width),
        `${where}: at the right end`,
      ).toBeLessThanOrEqual(16);
    // No band under the screen: the lowest visible box ends at the window's bottom padding at most.
    const vp = page.viewportSize()!;
    const lowest = await page.evaluate(() => {
      let bottom = 0;
      for (const el of document.querySelectorAll("#root *")) {
        const cs = getComputedStyle(el);
        if (cs.position === "fixed" || cs.display === "none") continue;
        const r = el.getBoundingClientRect();
        if (r.width > 0 && r.height > 0 && r.top < innerHeight)
          bottom = Math.max(bottom, r.bottom);
      }
      return bottom;
    });
    expect(lowest, `${where}: nothing past the window`).toBeLessThanOrEqual(
      vp.height,
    );
  };
  const footer = page.getByTestId("dice-footer");
  const header = page.locator("[data-screen] > header").first();

  // The real answer of this local build: the dev pill and 0.0.0+<sha> (or the last tag).
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  await expect(badge.getByTestId("version-pill")).toHaveText("dev");
  await expect(badge.getByTestId("version-text")).toHaveText(
    /^\d+\.\d+\.\d+\+[0-9a-f]{7}(\.dirty)?$/,
  );
  for (const vp of sizes) {
    await page.setViewportSize(vp);
    await expectBadgeIn(footer, `dashboard dev ${vp.width}`, "right-end");
    await snap(page, `version-dashboard-dev-${vp.width}x${vp.height}`);
  }

  // The roster and the creation flow.
  await page.setViewportSize(sizes[0]!);
  await page.getByTestId("roster-open").click();
  await expect(page.getByTestId("roster")).toBeVisible();
  for (const vp of sizes) {
    await page.setViewportSize(vp);
    await expectBadgeIn(header, `roster ${vp.width}`);
    await snap(page, `version-roster-${vp.width}x${vp.height}`);
  }
  await page.goto("/create");
  await expect(page.getByTestId("creation-step")).toBeVisible();
  for (const vp of sizes) {
    await page.setViewportSize(vp);
    await expectBadgeIn(header, `creation ${vp.width}`);
    await snap(page, `version-creation-${vp.width}x${vp.height}`);
  }

  // The three environments, from the health answer, on the dashboard and in English.
  for (const [env, version, pill] of [
    ["production", "1.4.2", null],
    ["staging", "1.4.3", "préprod"],
  ] as const) {
    await page.route("**/api/health", (r) =>
      r.fulfill({
        json: { status: "ok", commit: "abc", version, env, login: "google" },
      }),
    );
    await page.goto("/");
    await expect(page.getByTestId("version-text")).toHaveText(version);
    if (pill) await expect(page.getByTestId("version-pill")).toHaveText(pill);
    else await expect(page.getByTestId("version-pill")).toHaveCount(0);
    for (const vp of sizes) {
      await page.setViewportSize(vp);
      await expectBadgeIn(footer, `dashboard ${env} ${vp.width}`, "right-end");
      await snap(page, `version-dashboard-${env}-${vp.width}x${vp.height}`);
    }
    await page.unroute("**/api/health");
  }

  // The login page (no session), staging, in the four themes' modes.
  await context.clearCookies();
  await page.route("**/api/health", (r) =>
    r.fulfill({
      json: {
        status: "ok",
        commit: "abc",
        version: "1.4.3",
        env: "staging",
        login: "google",
      },
    }),
  );
  for (const theme of [
    { mode: "dark", darkTheme: "deep", lightTheme: "arcane" },
    { mode: "dark", darkTheme: "arcane", lightTheme: "arcane" },
    { mode: "light", darkTheme: "deep", lightTheme: "parchment" },
    { mode: "dark", darkTheme: "plain", lightTheme: "plain" },
  ]) {
    await page.evaluate(
      (s) =>
        localStorage.setItem(
          "critical.settings.v1",
          JSON.stringify({ lang: "fr", ...s }),
        ),
      theme,
    );
    await page.goto("/login");
    for (const vp of sizes) {
      await page.setViewportSize(vp);
      await expectBadgeIn(
        page.getByTestId("login-card"),
        `login ${theme.darkTheme}/${theme.lightTheme} ${vp.width}`,
      );
      if (vp.width === 1920)
        await snap(
          page,
          `version-login-${theme.mode === "light" ? theme.lightTheme : theme.darkTheme}-${vp.width}x${vp.height}`,
        );
    }
  }
}

// The "update available" button beside the version badge: it shows after two consecutive health
// answers with a newer version (the rule's cases are unit-tested; here the wiring), takes no band
// or height, stays inside the same box as the badge, and a click reloads the page. A tab that is
// hidden sends no heartbeat. Screenshots in French at 1920x1080 and 2560x1440.
async function newVersionStep(page: Page) {
  const sizes = [
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ];
  let served = "1.4.2";
  let asked = 0;
  await page.route("**/api/health", (r) => {
    asked++;
    return r.fulfill({
      json: {
        status: "ok",
        commit: "abc",
        version: served,
        env: "production",
        login: "google",
      },
    });
  });
  // A tab-focus heartbeat; the page ignores a second one within two seconds.
  const focusTab = async () => {
    await page.waitForTimeout(2100);
    await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  };
  const toast = page.getByTestId("new-version");
  const badge = page.getByTestId("version-badge");

  await openDashboard(page);
  await chooseLanguage(page, "fr");
  await expect(page.getByTestId("version-text")).toHaveText("1.4.2");
  const heightBefore = (await page.getByTestId("dice-footer").boundingBox())!
    .height;

  // A hidden tab sends nothing.
  await page.evaluate(() => {
    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      get: () => "hidden",
    });
  });
  served = "1.4.3";
  const askedBefore = asked;
  await focusTab();
  await page.waitForTimeout(300);
  expect(asked, "a hidden tab sends no heartbeat").toBe(askedBefore);
  await page.evaluate(() => {
    delete (document as unknown as { visibilityState?: string })
      .visibilityState;
  });

  // One newer answer shows nothing; the second one shows the button.
  await focusTab();
  await expect.poll(() => asked).toBe(askedBefore + 1);
  await expect(toast).toHaveCount(0);
  await focusTab();
  await expect(toast).toBeVisible();
  await expect(toast).toHaveText("Recharger");
  await expect(badge.getByTestId("version-text")).toHaveText("1.4.2");

  for (const vp of sizes) {
    await page.setViewportSize(vp);
    await expectNoPageScroll(page);
    await expectNoClippedText(badge, `toast ${vp.width}`);
    const footer = (await page.getByTestId("dice-footer").boundingBox())!;
    const t = (await toast.boundingBox())!;
    expect(t.x).toBeGreaterThanOrEqual(footer.x);
    expect(t.x + t.width).toBeLessThanOrEqual(footer.x + footer.width);
    expect(t.y).toBeGreaterThanOrEqual(footer.y);
    expect(t.y + t.height).toBeLessThanOrEqual(footer.y + footer.height);
    expect(footer.height, "no added height").toBe(heightBefore);
    await snap(page, `new-version-dashboard-${vp.width}x${vp.height}`);
  }

  // It follows to another screen without a reset (the roster's header card).
  await page.setViewportSize(sizes[0]!);
  await page.getByTestId("roster-open").click();
  await expect(page.getByTestId("roster")).toBeVisible();
  await expect(toast).toBeVisible();
  const header = (await page
    .locator("[data-screen] > header")
    .first()
    .boundingBox())!;
  const tr = (await toast.boundingBox())!;
  expect(tr.y + tr.height).toBeLessThanOrEqual(header.y + header.height);
  await expectNoPageScroll(page);
  await snap(page, "new-version-roster-1920x1080");

  // Nothing reloaded by itself; a click reloads, and the page then runs the new version.
  let loads = 0;
  page.on("framenavigated", (f) => {
    if (f === page.mainFrame()) loads++;
  });
  const reloaded = page.waitForEvent("load");
  await toast.click();
  await reloaded;
  expect(loads).toBeGreaterThan(0);
  await expect(page.getByTestId("version-text")).toHaveText("1.4.3");
  await expect(toast).toHaveCount(0);
}

// ADR 0043 step 5 and ADR 0047 items 4, 5 and 11: the PSP block in the character card's 280-384px
// column (below the HP and the HP change) and the rest popover over it, with a paying power kept up
// so the warning and the stop-and-recover caption are on screen, uncut in French and English, 1280 to
// 2560. Screenshots at 1920x1080 and 2560x1440, French first (ADR 0027 step 2).
async function pspStep(page: Page, size: Viewport) {
  await openDashboard(page);
  await keepPowerUp(page);
  const card = page.getByTestId("character");
  const panel = page.getByTestId("psp-panel");
  await expect(panel.getByTestId("psp-value")).toHaveText("37 / 40");

  // The card reads HP, HP change, then the PSPs.
  const top = async (l: Locator) => (await l.boundingBox())?.y ?? -1;
  const hp = await top(card.getByTestId("hp-value"));
  const change = await top(card.getByRole("group", { name: "HP change" }));
  const psp = await top(panel);
  expect(hp).toBeGreaterThan(0);
  expect(change, "HP change under the HP").toBeGreaterThan(hp);
  expect(psp, "PSP block under the HP change").toBeGreaterThan(change);

  for (const lang of ["fr", "en"] as const) {
    await chooseLanguage(page, lang);
    await page.keyboard.press("Escape");
    for (const vp of sizesAt(size, VIEWPORTS)) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      await expect(panel).toBeVisible();
      const box = await panel.boundingBox();
      expect(box?.width, `panel width, ${where}`).toBeGreaterThanOrEqual(200);
      expect(box?.width, `panel width, ${where}`).toBeLessThanOrEqual(384);
      await expectNoClippedText(panel, `PSP block, ${where}`);
      await expectWholeWords(panel, `PSP block, ${where}`);
      await expectNoPageScroll(page);

      await panel.getByTestId("psp-adjust").click();
      const adjust = page.getByTestId("psp-adjust-popover");
      await expect(adjust).toBeVisible();
      await expectNoClippedText(adjust, `adjust popover, ${where}`);
      await expectWholeWords(adjust, `adjust popover, ${where}`);
      await page.keyboard.press("Escape");
      await expect(adjust).toHaveCount(0);

      await panel.getByTestId("psp-rest").click();
      const rest = page.getByTestId("psp-rest-popover");
      await expect(rest.getByTestId("psp-rest-stops")).toBeVisible();
      await expectNoClippedText(rest, `rest popover, ${where}`);
      await expectWholeWords(rest, `rest popover, ${where}`);
      const inside = await rest.boundingBox();
      expect(inside?.x, `rest popover left, ${where}`).toBeGreaterThanOrEqual(
        0,
      );
      expect(
        (inside?.x ?? 0) + (inside?.width ?? 0),
        `rest popover right, ${where}`,
      ).toBeLessThanOrEqual(vp.width);
      if (vp.width >= 1920)
        await snap(page, `psp-rest-${lang}-${vp.width}x${vp.height}`);
      await page.keyboard.press("Escape");
      await expect(rest).toHaveCount(0);
      if (vp.width >= 1920)
        await snap(page, `psp-${lang}-${vp.width}x${vp.height}`);
    }
  }
}

// ADR 0047: Act > Pouvoirs (the round's power and the adjustment, the defence chips, the target band,
// the groups of rows), uncut in French and English, 1280 to 2560, in combat so the chips are live.
// Screenshots at 1920x1080 and 2560x1440, French first (ADR 0027 step 2).
async function powersStep(page: Page, size: Viewport) {
  await openDashboard(page);
  expect(
    await apiCall(page, "PUT", "/api/psionics/picks", {
      picks: [
        { kind: "discipline", id: "telepathy" },
        { kind: "power", id: "mind_blank" },
        { kind: "power", id: "thought_shield" },
        { kind: "power", id: "mind_thrust" },
        { kind: "power", id: "contact" },
        { kind: "power", id: "mindlink" },
        { kind: "discipline", id: "metapsionics" },
        { kind: "power", id: "intensify" },
      ],
    }),
  ).toBe(true);
  await reloadSettled(page);
  await nextRound(page);
  for (const lang of ["fr", "en"] as const) {
    await chooseLanguage(page, lang);
    await page.keyboard.press("Escape");
    await page
      .getByRole("tab", { name: lang === "fr" ? "Pouvoirs" : "Powers" })
      .click();
    const tab = page.getByTestId("powers-tab");
    await expect(tab.getByTestId("defence-chip").first()).toBeVisible();
    await tab.getByTestId("target-name").fill("Orc");
    // The standing mind blank raised, so the band shows its Stop beside the chip.
    const standing = tab.locator(
      "[data-testid=defence-chip][data-defence=standing]",
    );
    if ((await standing.count()) === 0)
      await tab
        .getByTestId("defence-chip")
        .filter({ hasText: /Mind Blank|Néant spirituel/ })
        .click();
    await expect(standing).toHaveCount(1);
    await expect(
      tab.getByRole("button", { name: /^(Stop maintaining|Arrêter)/ }),
    ).toHaveCount(1);
    // Relative to the tab: the stage may scroll when the bar opens, the rows must not move within it.
    const rowBoxes = async () => {
      const origin = (await tab.boundingBox())!;
      return Promise.all(
        (await tab.getByTestId("power-row").all()).map(async (r) => {
          const b = (await r.boundingBox())!;
          return { ...b, x: b.x - origin.x, y: b.y - origin.y };
        }),
      );
    };
    const select = (id: string) =>
      tab
        .locator(`[data-testid=power-row][data-id=${id}]`)
        .getByRole("button")
        .last()
        .click();
    const bar = page.getByTestId("action-bar");
    const close = () => bar.getByRole("button").first().click();
    for (const vp of sizesAt(size, VIEWPORTS)) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      await tab.getByTestId("target-band").locator("select").selectOption({
        index: 1,
      });
      await expect(bar).toHaveCount(0);
      await expect(
        tab.getByTestId("attack-score-target").first(),
      ).toBeVisible();
      await expectNoClippedText(tab, `powers tab, ${where}`);
      await expectWholeWords(tab, `powers tab, ${where}`);
      await expectNoPageScroll(page);
      const closed = await rowBoxes();
      expect(closed.length).toBeGreaterThan(3);
      const forms: [string, string][] = [
        ["mind_thrust", "attack-form"],
        ["intensify", "use-power"],
      ];
      for (const [id, form] of forms) {
        await select(id);
        await expect(bar.getByTestId(form)).toBeVisible();
        expect(await rowBoxes(), `rows with ${form} open, ${where}`).toEqual(
          closed,
        );
        await expectNoClippedText(tab, `${form}, ${where}`);
        await expectWholeWords(tab, `${form}, ${where}`);
        await expectNoPageScroll(page);
        if (vp.width >= 1920)
          await snap(page, `powers-${form}-${lang}-${vp.width}x${vp.height}`);
        await close();
        await expect(bar).toHaveCount(0);
      }
      await tab.getByTestId("incoming-attack").click();
      await expect(bar.getByTestId("defend-form")).toBeVisible();
      expect(await rowBoxes(), `rows with incoming open, ${where}`).toEqual(
        closed,
      );
      await expectNoClippedText(tab, `incoming, ${where}`);
      await expectWholeWords(tab, `incoming, ${where}`);
      await expectNoPageScroll(page);
      if (vp.width >= 1920)
        await snap(page, `powers-incoming-${lang}-${vp.width}x${vp.height}`);
      await close();
    }
  }
}

// The Maintained card of Act > Pouvoirs (ADR 0047 item 10): a tangent row with a long mind name, its
// cost, Stop and the balance, uncut in French and English, 1280 to 2560. Screenshots at 1920x1080 and
// 2560x1440, French first (ADR 0027 step 2).
async function maintainedStep(page: Page, size: Viewport) {
  await openDashboard(page);
  expect(
    await apiCall(page, "PUT", "/api/psionics/picks", {
      picks: [
        { kind: "discipline", id: "telepathy" },
        { kind: "power", id: "contact" },
        { kind: "power", id: "mindlink" },
        { kind: "power", id: "mind_thrust" },
      ],
    }),
  ).toBe(true);
  await reloadSettled(page);
  await nextRound(page);
  await chooseLanguage(page, "fr");
  await page.keyboard.press("Escape");
  await page.getByRole("tab", { name: "Pouvoirs" }).click();
  const tab = page.getByTestId("powers-tab");
  await keepATangent(
    page,
    tab,
    "Gardien des Mille Portes de la Citadelle Oubliée",
  );
  const card = page.getByTestId("maintained-card");
  await expect(card).toBeVisible();
  await expect(card.getByTestId("maintained-balance")).toBeVisible();
  for (const lang of ["fr", "en"] as const) {
    if (lang === "en") {
      await chooseLanguage(page, "en");
      await page.keyboard.press("Escape");
    }
    for (const vp of sizesAt(size, VIEWPORTS)) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      await expect(card.getByTestId("maintained-balance")).toBeVisible();
      await expectNoClippedText(card, `maintained card, ${where}`);
      await expectWholeWords(card, `maintained card, ${where}`);
      await expectNoPageScroll(page);
      if (vp.width >= 1920)
        await snap(page, `maintained-${lang}-${vp.width}x${vp.height}`);
    }
  }
}

// The bar's two forms of a power paid on contact, uncut in French and English, 1280 to 2560: the
// table-cost form (its controls from the table's rows, the server's cost) and the state with no contact
// held. Screenshots at 1920x1080 and 2560x1440, French first (ADR 0027 step 2).
async function contactFormsStep(page: Page, size: Viewport) {
  await openDashboard(page);
  expect(
    await apiCall(page, "PUT", "/api/psionics/picks", {
      picks: [
        { kind: "discipline", id: "telepathy" },
        { kind: "power", id: "contact" },
        { kind: "power", id: "mindlink" },
        { kind: "power", id: "mind_thrust" },
      ],
    }),
  ).toBe(true);
  await reloadSettled(page);
  await nextRound(page);
  for (const lang of ["fr", "en"] as const) {
    await chooseLanguage(page, lang);
    await page.keyboard.press("Escape");
    await page
      .getByRole("tab", { name: lang === "fr" ? "Pouvoirs" : "Powers" })
      .click();
    const tab = page.getByTestId("powers-tab");
    const band = tab.getByTestId("target-band");
    const bar = page.getByTestId("action-bar");
    const select = (id: string) =>
      tab
        .locator(`[data-testid=power-row][data-id=${id}]`)
        .getByRole("button")
        .last()
        .click();
    await band.getByTestId("target-name").fill("");
    await select("mindlink");
    await expect(bar.getByTestId("no-contact")).toBeVisible();
    for (const vp of sizesAt(size, VIEWPORTS)) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      await expectNoClippedText(tab, `no contact, ${where}`);
      await expectWholeWords(tab, `no contact, ${where}`);
      await expectNoPageScroll(page);
      if (vp.width >= 1920)
        await snap(page, `powers-no-contact-${lang}-${vp.width}x${vp.height}`);
    }
    await bar.getByRole("button").first().click();
    await band.getByTestId("target-name").fill("Orc");
    await band
      .getByRole("radio", { name: lang === "fr" ? "Ouvert" : "Open" })
      .click();
    await band.getByRole("textbox").nth(1).fill("3");
    await select("contact");
    const form = bar.getByTestId("table-form");
    await expect(form.getByTestId("table-preview")).toBeVisible();
    for (const vp of sizesAt(size, VIEWPORTS)) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      await expectNoClippedText(tab, `table form, ${where}`);
      await expectWholeWords(tab, `table form, ${where}`);
      await expectNoPageScroll(page);
      if (vp.width >= 1920)
        await snap(page, `powers-table-form-${lang}-${vp.width}x${vp.height}`);
    }
    await bar.getByRole("button").first().click();
  }
}

// ADR 0047: the action bar is pinned to the panel's bottom edge, outside the scrolling lists. With 16
// known powers (11 rows and 5 defence chips: the most the test catalogue allows) at 1280x800, 1920x1080 and 2560x1440, in French and English: the armed bar sits whole
// inside the Act panel wherever the list is scrolled, the last row stays reachable above it, and the
// bar follows the last row in the tab order. Screenshots at 1280x800 and 1920x1080, French first.
async function longListStep(page: Page, size: Viewport) {
  await openDashboard(page);
  const picks = [
    "mindlink",
    "psionic_blast",
    "contact",
    "mind_blank",
    "mind_thrust",
    "ego_whip",
    "id_insinuation",
    "psychic_crush",
    "thought_shield",
    "mental_barrier",
    "intellect_fortress",
    "tower_of_iron_will",
    "switch_personality",
    "intensify",
    "displacement",
    "graft_weapon",
  ];
  expect(
    await apiCall(page, "PUT", "/api/psionics/picks", {
      picks: [
        { kind: "discipline", id: "telepathy" },
        { kind: "discipline", id: "metapsionics" },
        { kind: "discipline", id: "psychometabolism" },
        ...picks.map((id) => ({ kind: "power", id })),
      ],
    }),
  ).toBe(true);
  await reloadSettled(page);
  for (const lang of ["fr", "en"] as const) {
    await chooseLanguage(page, lang);
    await page.keyboard.press("Escape");
    await page
      .getByRole("tab", { name: lang === "fr" ? "Pouvoirs" : "Powers" })
      .click();
    const tab = page.getByTestId("powers-tab");
    const panel = page.getByRole("tabpanel");
    const rows = tab.getByTestId("power-row");
    // The five defence modes are chips, not rows.
    await expect(rows).toHaveCount(picks.length - 5);
    const bar = page.getByTestId("action-bar");
    for (const vp of sizesAt(
      size,
      VIEWPORTS.filter((v) => [1280, 1920, 2560].includes(v.width)),
    )) {
      await page.setViewportSize(vp);
      const where = `${lang}, ${vp.width}x${vp.height}`;
      const inside = async (what: string) => {
        const b = (await bar.boundingBox())!;
        const p = (await panel.boundingBox())!;
        expect(b.y, `bar top, ${what}, ${where}`).toBeGreaterThanOrEqual(
          p.y - 0.5,
        );
        expect(
          b.y + b.height,
          `bar bottom, ${what}, ${where}`,
        ).toBeLessThanOrEqual(p.y + p.height + 0.5);
        return b;
      };
      // Armed from the first row, the list scrolled to its top.
      await panel.evaluate((el) => (el.scrollTop = 0));
      await rows.first().getByRole("button").last().click();
      await expect(bar).toBeVisible();
      await inside("first row armed");
      await expectNoClippedText(tab, `bar over the top, ${where}`);
      await expectWholeWords(tab, `bar over the top, ${where}`);
      await expectNoPageScroll(page);
      // Scrolled to the end: the last row sits above the bar, whole and clickable.
      await panel.evaluate((el) => (el.scrollTop = el.scrollHeight));
      const b = await inside("list scrolled to the end");
      const last = (await rows.last().boundingBox())!;
      expect(
        last.y + last.height,
        `last row above the bar, ${where}`,
      ).toBeLessThanOrEqual(b.y + 0.5);
      if (vp.width <= 1920)
        await snap(page, `powers-bar-end-${lang}-${vp.width}x${vp.height}`);
      // The bar is the next stop after the last row's control, in tab order.
      await rows.last().getByRole("button").last().focus();
      await page.keyboard.press("Tab");
      expect(
        await bar.evaluate((el) => el.contains(document.activeElement)),
        `tab order, ${where}`,
      ).toBe(true);
      // Arming the last row keeps the bar whole too.
      await rows.last().getByRole("button").last().click();
      await expect(bar).toBeVisible();
      await inside("last row armed");
      await bar.getByRole("button").first().click();
      await expect(bar).toHaveCount(0);
    }
  }
}

// ADR 0043 step 9: the known powers stage (the allowance tiles, the picks with their costs and scores,
// the refusal, the picker with its search, discipline chips once and the add-a-discipline select), uncut in French and English, 1280 to
// 2560. The refusal is on screen: it is the longest text of the stage. Screenshots at 1920x1080 and
// 2560x1440, French first (ADR 0027 step 2).
async function knownPowersStep(page: Page, size: Viewport) {
  await openDashboard(page);
  const panel = await openSortsPowers(page);
  // The discipline names show once, as the filter chips; a discipline is added from the select.
  const addDiscipline = panel.getByRole("combobox", {
    name: "Add a discipline",
  });
  await expect(panel.getByRole("button", { name: "Telepathy" })).toHaveCount(1);
  for (const name of ["Telepathy", "Clairsentience"])
    await addDiscipline.selectOption({ label: name });
  await panel.getByRole("combobox", { name: "Search powers..." }).fill("mind");
  await panel
    .getByTestId("power-row")
    .filter({ hasText: "Mind Blank" })
    .getByRole("button", { name: "Add Mind Blank" })
    .click();
  await panel
    .getByTestId("power-row")
    .filter({ hasText: "Mind Thrust" })
    .getByRole("button", { name: "Add Mind Thrust" })
    .click();
  await addDiscipline.selectOption({ label: "Psychokinesis" });
  await expect(panel.getByTestId("powers-refusal")).toBeVisible();
  // The known list beside the browser, then a power's detail open in its place.
  for (const view of ["list", "detail"] as const) {
    if (view === "detail") {
      await panel.getByTestId("power-row").first().click();
      await expect(panel.getByTestId("power-detail")).toBeVisible();
    }
    for (const lang of ["fr", "en"] as const) {
      await chooseLanguage(page, lang);
      await page.keyboard.press("Escape");
      for (const vp of sizesAt(size, VIEWPORTS)) {
        await page.setViewportSize(vp);
        const where = `${view}, ${lang}, ${vp.width}x${vp.height}`;
        await expect(panel).toBeVisible();
        await expectNoClippedText(panel, `known powers, ${where}`);
        await expectWholeWords(panel, `known powers, ${where}`);
        await expectNoPageScroll(page);
        if (vp.width >= 1920)
          await snap(
            page,
            `known-powers-${view}-${lang}-${vp.width}x${vp.height}`,
          );
      }
    }
  }
}

// One page header on every full-screen page: Roster, character creation, Admin and Ruleset share the
// brand (mark and wordmark in full, with its accessible name), the title, the version and the way
// out, at the same height and the same place, uncut, fr and en, 1280x800 to 2560x1440 (ADR 0027
// step 4.6). Only a browser lays the header out, so no Go or unit test can hold this.
async function pageHeaderStep(page: Page, size: Viewport) {
  const sizes = sizesAt(size, [VIEWPORTS[0], VIEWPORTS[3], VIEWPORTS[4]]);
  const pages = [
    {
      name: "roster",
      open: async () => {
        await page.goto("/");
        await page.getByTestId("roster-open").click();
        await expect(page.getByTestId("roster")).toBeVisible();
      },
    },
    { name: "create", open: () => page.goto("/create") },
    { name: "admin", open: () => page.goto("/admin") },
    { name: "ruleset", open: () => page.goto("/ruleset") },
  ];
  await openDashboard(page);
  for (const lang of ["fr", "en"] as const) {
    await chooseLanguage(page, lang);
    const seen = new Map<string, { height: number; left: number }>();
    for (const p of pages) {
      await page.setViewportSize(sizes[0]);
      await p.open();
      const header = page.getByTestId("page-header");
      await expect(header).toBeVisible();
      for (const vp of sizes) {
        await page.setViewportSize(vp);
        const where = `${p.name}, ${lang}, ${vp.width}x${vp.height}`;
        // The mark has its accessible name and the wordmark shows in full at every width.
        await expect(
          header.getByRole("img", { name: "Critical!" }),
        ).toBeVisible();
        await expect(header.getByTestId("brand-icon")).toBeVisible();
        await expect(header.getByTestId("wordmark")).toHaveText("Critical!");
        await expect(header.getByRole("heading", { level: 1 })).toBeVisible();
        await expectNoClippedText(header, `page header, ${where}`);
        const m = await header.evaluate((el) => {
          const r = el.getBoundingClientRect();
          const brand = el
            .querySelector("[data-testid=brand]")!
            .getBoundingClientRect();
          const doc = document.documentElement;
          return {
            height: Math.round(r.height),
            left: Math.round(brand.left - r.left),
            right: r.right,
            vw: doc.clientWidth,
            wide: doc.scrollWidth > doc.clientWidth,
          };
        });
        expect(
          m.right,
          `header inside the screen, ${where}`,
        ).toBeLessThanOrEqual(m.vw);
        expect(m.wide, `page scrolls sideways, ${where}`).toBe(false);
        // Every page's header is the same box with the brand in the same place.
        const key = `${lang}-${vp.width}`;
        const first = seen.get(key);
        if (!first) seen.set(key, { height: m.height, left: m.left });
        else {
          expect(m.height, `header height, ${where}`).toBe(first.height);
          expect(m.left, `brand position, ${where}`).toBe(first.left);
        }
        await header.screenshot({
          path: `e2e-results/screens/layout/page-header-${p.name}-${lang}-${vp.width}.png`,
        });
      }
      await snap(page, `page-header-${p.name}-${lang}-full`);
    }
    await page.setViewportSize(sizes[0]);
    await page.goto("/");
  }
}

// The same header in every theme and mode (French, 1920x1080), on the Ruleset page.
async function pageHeaderThemesStep(page: Page, size: Viewport) {
  await page.setViewportSize(size);
  await openDashboard(page);
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
      await page.goto("/ruleset");
      const header = page.getByTestId("page-header");
      await expect(header.getByTestId("wordmark")).toBeVisible();
      await expectNoClippedText(header, `page header, ${which} ${name}`);
      await header.screenshot({
        path: `e2e-results/screens/layout/page-header-theme-${which}-${name.toLowerCase()}.png`,
      });
      await page.goto("/");
      await expect(page.getByTestId("brand")).toBeVisible();
    }
  }
}

/**
 * The full-screen pages walk: the pages that are not the dashboard, one size per walk. The roster step
 * retires every character, so it comes last; the steps that clear the session (login, the version badge's
 * login card) get it back from `runWalk`.
 */
const PAGES_STEPS: [string, Step][] = [
  ["page header", pageHeaderStep],
  ["page header themes", pageHeaderThemesStep],
  ["text floor, roster, admin, ruleset", textFloorPagesStep],
  ["ruleset, admin", rulesetAdminStep],
  ["ruleset, player", rulesetPlayerStep],
  ["admin section", adminStep],
  ["database banner", dbBannerStep],
  ["version badge", versionBadgeStep],
  ["creation route", creationStep],
  ["creation, multi-class", creationMultiStep],
  ["creation extras", creationExtrasStep],
  ["login page", loginStep],
  ["roster", rosterStep],
];

test.describe("pages walks", () => {
  test.use({ character: { alignment: "", sex: "" } });
  for (const size of [VIEWPORTS[0], VIEWPORTS[3], VIEWPORTS[4]]) {
    test(`pages walk at ${size.width}x${size.height}`, async ({
      page,
      stack,
    }) => {
      test.setTimeout(90_000);
      // Leaving a filled-in flow asks the browser's beforeunload question: accept it.
      page.on("dialog", (d) => void d.accept());
      await runWalk(page, size, PAGES_STEPS, stack);
    });
  }
});

/**
 * The class screens walk: the level-up prompt and its grants, then the fighter, mage, cleric and thief
 * screens, each step making its own character through the API, one size per walk.
 */
const CLASS_STEPS: [string, Step][] = [
  ["level-up prompt", levelUpPromptStep],
  ["level-up grants", levelUpGrantsStep],
  ["level-up free spell", levelUpFreeSpellStep],
  ["fighter", fighterStep],
  ["mage, Learn and Add", mageLearnStep],
  ["mage, above the Intelligence limit", mageLimitStep],
  ["cleric", clericStep],
  ["thief", thiefStep],
];

for (const size of [VIEWPORTS[0], VIEWPORTS[3], VIEWPORTS[4]]) {
  test(`class screens walk at ${size.width}x${size.height}`, async ({
    page,
    stack,
  }) => {
    test.setTimeout(90_000);
    await runWalk(page, size, CLASS_STEPS, stack);
  });
}

/**
 * The psionicist walk: the PSP block, the Powers tab, the Maintained card, the contact forms and the known
 * powers, one size per walk. The long list needs a level 20 character, so it has a walk of its own.
 */
// Each step starts with no picks and nothing maintained: an earlier step's would change what the
// disciplines select and the Powers tab offer.
const freshPicks =
  (step: Step): Step =>
  async (page, size, stack, context) => {
    const res = await page.request.fetch(`${stack.webUrl}/api/psionics/picks`, {
      method: "PUT",
      data: { picks: [] },
      headers: { Origin: stack.webUrl },
    });
    expect(res.ok(), await res.text()).toBe(true);
    // The PSP step keeps Intensify up: stopping it (a no-op when it is not kept) leaves nothing maintained.
    await page.request.fetch(
      `${stack.webUrl}/api/psionics/powers/intensify/stop`,
      {
        method: "POST",
        data: {},
        headers: { Origin: stack.webUrl },
      },
    );
    await step(page, size, stack, context);
  };

async function psionicistActTabsStep(page: Page, size: Viewport) {
  await openDashboard(page);
  expect(
    await apiCall(page, "PUT", "/api/psionics/picks", {
      picks: [
        { kind: "discipline", id: "telepathy" },
        { kind: "power", id: "mind_blank" },
        { kind: "power", id: "thought_shield" },
        { kind: "power", id: "mind_thrust" },
        { kind: "power", id: "contact" },
        { kind: "power", id: "mindlink" },
      ],
    }),
  ).toBe(true);
  await reloadSettled(page);
  await chooseLanguage(page, "fr");
  await page.keyboard.press("Escape");
  await expectJournalTopSteadyOnEveryTab(page, size, "psionicist");
  await expectWideStage(page, size, "psionicist", false);
}

const PSIONICIST_STEPS: [string, Step][] = (
  [
    ["PSP block and rest popover", pspStep],
    ["act tabs", psionicistActTabsStep],
    ["powers tab", powersStep],
    ["maintained card", maintainedStep],
    ["contact forms", contactFormsStep],
    ["known powers", knownPowersStep],
  ] as [string, Step][]
).map(([name, step]) => [name, freshPicks(step)]);

test.describe("psionicist walks", () => {
  test.use({ character: psionicist });
  for (const size of [VIEWPORTS[0], VIEWPORTS[3], VIEWPORTS[4]]) {
    test(`psionicist walk at ${size.width}x${size.height}`, async ({
      page,
      stack,
    }) => {
      test.setTimeout(90_000);
      await runWalk(page, size, PSIONICIST_STEPS, stack);
    });
  }
});

test.describe("psionicist long list walks", () => {
  test.use({
    character: { ...psionicist, level: 20, xp_total: 5_000_000, wis: 20 },
  });
  for (const size of [VIEWPORTS[0], VIEWPORTS[3], VIEWPORTS[4]]) {
    test(`powers long list walk at ${size.width}x${size.height}`, async ({
      page,
      stack,
    }) => {
      test.setTimeout(90_000);
      await runWalk(page, size, [["long list", longListStep]], stack);
    });
  }
});

/**
 * ADR 0009 step 3: below 1024 px the page shows the "needs a wider window" message instead of the
 * layout, follows a resize across the threshold with no reload, and scrolls with nothing overlapping.
 * Only a browser resizes a window and lays the page out.
 */
async function minWidthStep(page: Page) {
  await openDashboard(page);
  const message = page.getByTestId("min-width-message");
  const app = page.getByTestId("rail");
  for (const lang of ["en", "fr"] as const) {
    await page.setViewportSize({ width: 1280, height: 800 });
    await chooseLanguage(page, lang);
    await page.keyboard.press("Escape");
    for (const size of [
      { width: 1024, height: 768 },
      { width: 1023, height: 768 },
      { width: 390, height: 844 },
    ]) {
      await page.setViewportSize(size);
      const narrow = size.width < 1024;
      await expect(message, `message at ${size.width}, ${lang}`).toBeVisible({
        visible: narrow,
      });
      await expect(app).toBeVisible({ visible: !narrow });
      if (!narrow) continue;
      await expect(message.getByRole("heading")).toHaveText(
        lang === "en"
          ? "This app needs a wider window"
          : "Cette application a besoin d'une fenêtre plus large",
      );
      expect(
        await page.evaluate(() => {
          const root = document.scrollingElement ?? document.documentElement;
          const r = document
            .querySelector("[data-testid=min-width-message]")!
            .getBoundingClientRect();
          return {
            sideways: root.scrollWidth > root.clientWidth,
            inside: r.left >= 0 && r.right <= root.clientWidth,
          };
        }),
        `no sideways scroll, message inside the window, ${size.width}, ${lang}`,
      ).toEqual({ sideways: false, inside: true });
      await expectNoClippedText(page.locator("body"), `${size.width}, ${lang}`);
      await snap(page, `min-width-${size.width}-${lang}`);
    }
  }
}

/**
 * The browser behaviours walk: what only a browser proves (drag, focus, persistence in local storage, a
 * client store, a heartbeat), at 1440x900. The toast and the composer step set their own viewports.
 */
const BEHAVIOUR_STEPS: [string, Step][] = [
  ["journal split", journalSplitStep],
  ["rail", railStep],
  ["settings", settingsStep],
  ["workspaces to the dice composer", composerStep],
  ["new version toast", newVersionStep],
  ["minimum width message", minWidthStep],
];

test.describe("behaviours walk", () => {
  test.use({ extraSpells: COMPOSER_EXTRA_SPELLS });
  test("behaviours walk at 1440x900", async ({ page, stack }) => {
    test.setTimeout(90_000);
    await runWalk(page, VIEWPORTS[1], BEHAVIOUR_STEPS, stack);
  });
});
