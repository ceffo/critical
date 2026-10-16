import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test, expect, snap, catalogueSpell } from "./fixtures";
import {
  chooseLanguage,
  chooseSetting,
  openDashboard,
  openWorkspace,
  receivePicker,
} from "./helpers";
import type { Locator, Page } from "@playwright/test";

// The frozen fixture catalogue and spellbook the stack runs on (never the campaign seeds/).
const testdata = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../api/testdata",
);
const fixtureSpells = (
  JSON.parse(readFileSync(path.join(testdata, "catalogue.json"), "utf8")) as {
    spells: unknown[];
  }
).spells;
const fixtureBook = JSON.parse(
  readFileSync(path.join(testdata, "seeds", "spellbook.json"), "utf8"),
) as { id: string }[];

/** A catalogue spell appended to the fixture for these tests; its id is the slug of its name. */
function row(
  name: string,
  cls: "Wizard" | "Priest",
  level: number,
  school: string,
  sphere = "",
  extra: Record<string, string> = {},
) {
  const id = name.toLowerCase().replace(/[^a-z0-9]+/g, "_");
  return catalogueSpell({
    id,
    name,
    class: cls,
    level,
    school,
    sphere,
    ...extra,
  });
}

// The fixture has Fireball, Magic Missile, Burning Hands...; these add what the criteria name.
const named = [
  row("Project Image", "Wizard", 6, "Illusion/Phantasm", "", {
    casting_time: "6",
    range: "10 yds./level",
    duration: "1 rd./level",
    area_of_effect: "Special",
  }),
  row("Fire Trap", "Wizard", 4, "Abjuration, Evocation"),
  row("Fire Shield", "Wizard", 4, "Evocation, Alteration"),
  row("Faerie Fire", "Priest", 1, "Alteration", "Elemental Fire"),
  row("Flame Strike", "Priest", 5, "Evocation", "Elemental Fire"),
  row("Cure Light Wounds", "Priest", 1, "Necromancy", "Healing"),
  row("Dimensional Folding", "Wizard", 7, "Dimension"),
  row("Quixotic Probe", "Priest", 2, "Divination", "Sun", {
    casting_time: "Special",
    range: "Touch",
    duration: "Instantaneous",
    area_of_effect: "The caster",
    saving_throw: "None",
  }),
  row(
    "Comprehensively Unreasonably Long Spell Name Of The Ancient Archmages",
    "Wizard",
    9,
    "Divination",
    "",
    {
      nameFr:
        "Nom de sortilège invraisemblablement long des anciens archimages",
    },
  ),
  row("Quixotic Oddity", "Wizard", 3, "Divination", "", {
    range: "Sight line",
    duration: "Instantaneous",
    area_of_effect: "A weird zone",
  }),
];

test.use({ extraSpells: { rows: named } });

async function openLookup(page: Page) {
  await openDashboard(page);
  await openWorkspace(page, "Spells");
  await expect(search(page)).toBeVisible();
  await expect(rows(page).first()).toBeVisible();
}

const search = (page: Page) =>
  page.getByRole("combobox", { name: "Search spells..." });
const count = (page: Page) => page.getByTestId("lookup-count");
const rows = (page: Page) => page.getByTestId("spell-row");
const selected = (page: Page) =>
  page
    .getByRole("listbox", { name: "Spells" })
    .getByRole("option", { selected: true });
const detail = (page: Page) => page.getByTestId("spell-detail");
const classButton = (page: Page, name: string) =>
  page
    .getByRole("group", { name: /^(Spell list|Liste de sorts)$/ })
    .getByRole("button", { name, exact: true });
const levelChip = (page: Page, n: number) =>
  page.getByRole("button", { name: `Level ${n}`, exact: true });

async function attr(list: Locator, name: string): Promise<string[]> {
  return list.evaluateAll(
    (els, a) => els.map((e) => e.getAttribute(a) ?? ""),
    name,
  );
}

async function names(list: Locator): Promise<string[]> {
  return list
    .getByTestId("spell-name")
    .evaluateAll((els) => els.map((e) => e.textContent ?? ""));
}

test("lookup: typing 'project im' selects Project Image with no click and shows its stats", async ({
  page,
}) => {
  await openLookup(page);
  await search(page).fill("project im");
  await expect(selected(page)).toHaveAttribute("data-id", "project_image");
  await expect(detail(page).getByRole("heading")).toHaveText("Project Image");
  // Values at the Bard's level (fixture: 15) next to the raw text.
  const range = detail(page).getByTestId("stat-range");
  await expect(range.getByTestId("stat-value")).toHaveText("150 yd");
  await expect(range.getByTestId("stat-raw")).toContainText("10 yds./level");
  await expect(
    detail(page).getByTestId("stat-duration").getByTestId("stat-value"),
  ).toHaveText("15 rd");
  await snap(page, "search-selected");
});

test("lookup in French: a French name finds the spell, its French text shows with its table, its stats in French and metres; a spell without French shows English", async ({
  page,
}) => {
  await openLookup(page);
  await chooseLanguage(page, "fr");
  const searchFr = page.getByRole("combobox", {
    name: "Rechercher des sorts...",
  });
  await searchFr.fill("orbe chrom");
  const first = rows(page).first();
  await expect(first).toHaveAttribute("data-id", "chromatic_orb");
  await expect(first).toContainText("Orbe chromatique");
  await expect(first).not.toContainText("Chromatic Orb");
  await expect(detail(page).getByRole("heading")).toHaveText(
    "Orbe chromatique",
  );
  const text = detail(page).getByTestId("spell-description-text");
  await expect(text).toContainText("Texte de test pour Orbe chromatique");
  const table = text.getByTestId("spell-table").getByRole("table");
  await expect(table.getByRole("columnheader")).toHaveText([
    "Niveau",
    "Couleur",
    "Effet",
  ]);
  await expect(table.getByRole("row")).toHaveCount(4);
  await chooseSetting(page, "Clair");
  await snap(page, "french-table-light");
  await chooseSetting(page, "Sombre");
  await snap(page, "french-table-dark");

  // The stats are worded in French from their typed values, lengths in metres (ADR 0021 step 8).
  await searchFr.fill("projectile magique");
  await expect(detail(page).getByRole("heading")).toContainText(
    "Projectile magique",
  );
  const stat = (key: string) => detail(page).getByTestId(`stat-${key}`);
  await expect(stat("range").getByTestId("stat-value")).toHaveText("210 m");
  await expect(stat("range").getByTestId("stat-raw")).toHaveText(
    "60 m + 10 m/niveau",
  );
  await expect(stat("duration")).toContainText("Instantanée");
  await expect(stat("area")).toContainText("1-5 cibles");
  await expect(stat("castingTime")).toContainText("1 segment");
  await expect(stat("save")).toContainText("Aucune");
  await expect(detail(page).getByTestId("spell-meta")).toContainText("V, S");
  await snap(page, "french-stats");

  // A long French area wraps in the detail grid at the narrowest width, never cut.
  await page.setViewportSize({ width: 1280, height: 900 });
  await searchFr.fill("rapidit");
  await expect(detail(page).getByRole("heading")).toContainText("Rapidité");
  const area = stat("area");
  await expect(area.getByTestId("stat-raw")).toHaveText(
    "cube de 12 m, 1 créature/niveau",
  );
  for (const el of await area.locator("*").all()) {
    const [scroll, client] = await el.evaluate((e) => [
      e.scrollWidth,
      e.clientWidth,
    ]);
    expect(scroll).toBeLessThanOrEqual(client);
  }
  await snap(page, "french-long-area");
  await page.setViewportSize({ width: 1440, height: 900 });

  await searchFr.fill("armor");
  await expect(detail(page).getByRole("heading")).toHaveText("Armor");
  await expect(text).toContainText("Fixture text for Armor");
  await expect(stat("castingTime")).toContainText("1 round");
  await expect(stat("range")).toContainText("Contact");
  await snap(page, "french-english-fallback");
});

const LONG =
  "Comprehensively Unreasonably Long Spell Name Of The Ancient Archmages";
const LONG_FR =
  "Nom de sortilège invraisemblablement long des anciens archimages";

for (const size of [
  { width: 1440, height: 900 },
  { width: 1707, height: 1300 },
]) {
  test(`lookup: a long spell name is shown whole, in the UI language only, at ${size.width}x${size.height}`, async ({
    page,
  }) => {
    await page.setViewportSize(size);
    await openLookup(page);
    await classButton(page, "All").click();
    await search(page).fill("unreasonably");
    await expect(rows(page)).toHaveCount(1);
    await expect(detail(page).getByRole("heading")).toHaveText(LONG);

    // Every visible name (list rows and detail title) is whole: not wider than its box, full text.
    const whole = async (expected: string, lang: string) => {
      const els = page.getByTestId("spell-name");
      await expect(els.first()).toBeVisible();
      for (const el of await els.all()) {
        const [scroll, client] = await el.evaluate((e) => [
          e.scrollWidth,
          e.clientWidth,
        ]);
        expect(scroll).toBeLessThanOrEqual(client);
        await expect(el).toHaveText(expected);
        // A word is only ever broken by hyphenation at a syllable, never chopped: the name is
        // hyphenated in its own language, and with hyphenation off and no emergency break every
        // word still fits the box (so break-words is never what splits one).
        await expect(el).toHaveAttribute("lang", lang);
        await expect(el).toHaveCSS("hyphens", "auto");
        const overflows = await el.evaluate((e) => {
          const box = e as HTMLElement;
          const [h, w] = [box.style.hyphens, box.style.overflowWrap];
          box.style.hyphens = "manual";
          box.style.overflowWrap = "normal";
          const over = box.scrollWidth > box.clientWidth;
          box.style.hyphens = h;
          box.style.overflowWrap = w;
          return over;
        });
        expect(overflows).toBe(false);
      }
      await expect(page.getByTestId("spell-other-name")).toHaveCount(0);
    };
    await whole(LONG, "en");
    await snap(page, `long-name-en-${size.width}`);

    await chooseLanguage(page, "fr");
    await expect(detail(page).getByRole("heading")).toHaveText(LONG_FR);
    await snap(page, `long-name-fr-${size.width}`);
    await whole(LONG_FR, "fr");
    await snap(page, `long-name-fr-${size.width}`);
  });
}

test("lookup: the toggle defaults to Wizard; Mine is the spellbook; Priest and All work; class chip only on All", async ({
  page,
}) => {
  await openLookup(page);
  await expect(classButton(page, "Wizard")).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await expect(rows(page).first()).toBeVisible();
  expect(new Set(await attr(rows(page), "data-class"))).toEqual(
    new Set(["Wizard"]),
  );
  await expect(page.getByTestId("spell-class")).toHaveCount(0);

  await classButton(page, "Mine").click();
  await expect(count(page)).toHaveText(`${fixtureBook.length} spells`);
  expect((await attr(rows(page), "data-id")).sort()).toEqual(
    fixtureBook.map((e) => e.id).sort(),
  );
  await expect(page.getByTestId("spell-class")).toHaveCount(0);
  await snap(page, "mine");

  await classButton(page, "Priest").click();
  await expect(rows(page).filter({ hasText: "Faerie Fire" })).toHaveCount(1);
  expect(new Set(await attr(rows(page), "data-class"))).toEqual(
    new Set(["Priest"]),
  );
  await expect(page.getByTestId("spell-class")).toHaveCount(0);

  await classButton(page, "All").click();
  await expect(count(page)).toHaveText(
    `${fixtureSpells.length + named.length} spells`,
  );
  expect(new Set(await attr(rows(page), "data-class"))).toEqual(
    new Set(["Wizard", "Priest"]),
  );
  await expect(page.getByTestId("spell-class").first()).toBeVisible();
  await expect(page.getByTestId("spell-class")).toHaveCount(
    await rows(page).count(),
  );
});

test("lookup: up/down in the search box moves the selection, a click selects; values at level with raw text; description always shown, scrolling in its own area", async ({
  page,
}) => {
  await openLookup(page);
  await search(page).fill("magic");
  // Prefix matches (Magic Missile, Magic Jar) before contains (Detect Magic...).
  await expect(selected(page)).toHaveAttribute("data-id", "magic_missile");
  const ids = await attr(rows(page), "data-id");
  expect(ids.length).toBeGreaterThan(2);
  await search(page).press("ArrowDown");
  await expect(selected(page)).toHaveAttribute("data-id", ids[1]);
  await search(page).press("ArrowUp");
  await expect(selected(page)).toHaveAttribute("data-id", ids[0]);

  await rows(page).filter({ hasText: "Dispel Magic" }).click();
  await expect(selected(page)).toHaveAttribute("data-id", "dispel_magic");
  await rows(page).filter({ hasText: "Magic Missile" }).click();
  await expect(selected(page)).toHaveAttribute("data-id", "magic_missile");
  const range = detail(page).getByTestId("stat-range");
  await expect(range.getByTestId("stat-value")).toHaveText("210 yd");
  await expect(range.getByTestId("stat-raw")).toHaveText(
    "60 yds. + 10 yds./level",
  );
  await expect(
    detail(page)
      .getByTestId("spell-dice-damage")
      .getByTestId("spell-dice-formula"),
  ).toHaveText("5 × 1d4+1 force");
  await snap(page, "damage-dice");

  // A damage whose dice change from one strike to the next: every step, in order.
  await search(page).fill("chain lightning");
  await rows(page).filter({ hasText: "Chain Lightning" }).click();
  await expect(
    detail(page)
      .getByTestId("spell-dice-damage")
      .getByTestId("spell-dice-formula"),
  ).toHaveText(
    "in turn: 12d6, 11d6, 10d6, 9d6, 8d6, 7d6, 6d6, 5d6, 4d6, 3d6, 2d6, 1d6 electricity",
  );
  await snap(page, "damage-dice-steps");
  await search(page).fill("incendiary cloud");
  await rows(page).filter({ hasText: "Incendiary Cloud" }).click();
  const cloud = detail(page).getByTestId("spell-dice-damage");
  await expect(cloud.getByTestId("spell-dice-formula")).toHaveText(
    "by round: —, —, 15d2, 15d4, 15d2 fire",
  );
  await cloud.getByRole("button", { name: "To the dice" }).click();
  await expect(
    page.getByRole("form", { name: "Dice" }).getByLabel("Formula"),
  ).toHaveValue("15d2");
  await snap(page, "damage-dice-steps-round");
  await search(page).fill("magic");
  await rows(page).filter({ hasText: "Magic Missile" }).click();

  const description = detail(page).getByTestId("spell-description");
  const text = description.getByTestId("spell-description-text");
  await expect(text).toBeVisible();
  await expect(description.locator("summary")).toHaveCount(0);
  const full = (await text.textContent()) ?? "";
  expect(full.length).toBeGreaterThan(200); // not cut
  expect(full.trimEnd().endsWith("as desired.")).toBe(true);
  // Bounded: a long text scrolls inside its own area.
  expect(await text.evaluate((el) => el.scrollHeight > el.clientHeight)).toBe(
    true,
  );
  await text.evaluate((el) => el.scrollTo(0, el.scrollHeight));
  await snap(page, "description-scroll");
});

test("lookup: tab switch keeps search, filters and selection; Reset only when set; reload clears", async ({
  page,
}) => {
  await openLookup(page);
  const reset = page.getByRole("button", { name: "Reset", exact: true });
  await expect(reset).toHaveCount(0);

  await search(page).fill("fire");
  await classButton(page, "All").click();
  await levelChip(page, 4).click();
  await page
    .getByRole("combobox", { name: "School" })
    .selectOption({ label: "Abjuration" });
  await expect(rows(page)).toHaveCount(1);
  await expect(selected(page)).toHaveAttribute("data-id", "fire_trap");
  await expect(reset).toBeVisible();

  await openWorkspace(page, "Act");
  await page.getByRole("tab", { name: "Weapons" }).click();
  await openWorkspace(page, "Spells");
  await expect(search(page)).toHaveValue("fire");
  await expect(classButton(page, "All")).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await expect(levelChip(page, 4)).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByRole("combobox", { name: "School" })).toHaveValue(
    "abjuration",
  );
  await expect(selected(page)).toHaveAttribute("data-id", "fire_trap");

  await reset.click();
  await expect(search(page)).toHaveValue("");
  await expect(classButton(page, "Wizard")).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await expect(levelChip(page, 4)).toHaveAttribute("aria-pressed", "false");
  await expect(page.getByRole("combobox", { name: "School" })).toHaveValue("");
  await expect(reset).toHaveCount(0);

  await search(page).fill("magic");
  await levelChip(page, 1).click();
  await page.reload();
  await openWorkspace(page, "Spells");
  await expect(search(page)).toHaveValue("");
  await expect(levelChip(page, 1)).toHaveAttribute("aria-pressed", "false");
  await expect(reset).toHaveCount(0);
});

test("lookup: the Receive picker finds spells by French and English names with the lookup's ranking", async ({
  page,
}) => {
  await openLookup(page);
  await classButton(page, "All").click();
  await search(page).fill("fire");
  await expect(rows(page)).toHaveCount(5);
  const lookupOrder = await attr(rows(page), "data-id");

  await openWorkspace(page, "Act");
  const picker = await receivePicker(page);
  const box = picker.getByLabel("Search for a spell cast on me");
  await box.fill("fire");
  await expect(picker.getByRole("listitem")).toHaveCount(5);
  await expect(picker.getByRole("listitem").first()).toContainText("Fireball");
  const receiveNames = await picker
    .getByRole("listitem")
    .evaluateAll((els) =>
      els.map((e) => (e.textContent ?? "").split(" · ")[0]),
    );
  expect(receiveNames).toEqual([
    "Fireball",
    "Fire Shield",
    "Fire Trap",
    "Faerie Fire",
    "Wall of Fire",
  ]);
  expect(lookupOrder).toEqual([
    "fireball",
    "fire_shield",
    "fire_trap",
    "faerie_fire",
    "wall_of_fire",
  ]);

  await box.fill("mains");
  await expect(picker.getByRole("listitem").first()).toContainText(
    "Burning Hands",
  );
});

// Enough synthetic Wizard rows that the list needs several 200-row pages.
const many = Array.from({ length: 450 }, (_, i) =>
  row(
    `Scroll Test ${String(i).padStart(3, "0")}`,
    "Wizard",
    (i % 9) + 1,
    "Alteration",
  ),
);

test.describe("infinite scroll", () => {
  test.use({ extraSpells: { rows: [...named, ...many] } });

  test("lookup: the header shows the true total; scrolling to the bottom loads every spell, the last a level 9", async ({
    page,
  }) => {
    await openLookup(page);
    await classButton(page, "All").click();
    const total = fixtureSpells.length + named.length + many.length;
    await expect(count(page)).toHaveText(`${total} spells`);
    await expect(rows(page)).toHaveCount(200);
    await expect(
      page.getByRole("button", { name: /^(Next|Previous)$/ }),
    ).toHaveCount(0);

    const list = page.getByRole("listbox", { name: "Spells" });
    for (let i = 0; i < 10 && (await rows(page).count()) < total; i++) {
      const before = await rows(page).count();
      await list.evaluate((el) => el.scrollTo({ top: el.scrollHeight }));
      await expect.poll(() => rows(page).count()).toBeGreaterThan(before);
    }
    await expect(rows(page)).toHaveCount(total);
    await expect(count(page)).toHaveText(`${total} spells`);
    await expect(rows(page).last()).toHaveAttribute("data-level", "9");
    await list.evaluate((el) => el.scrollTo({ top: el.scrollHeight }));
    await snap(page, "scrolled-to-end");

    // A selection deep in the list is still in view after a tab switch.
    const deep = rows(page).nth(350);
    const deepId = (await deep.getAttribute("data-id")) ?? "";
    await deep.scrollIntoViewIfNeeded();
    await deep.click();
    await openWorkspace(page, "Act");
    await page.getByRole("tab", { name: "Weapons" }).click();
    await openWorkspace(page, "Spells");
    await expect(selected(page)).toHaveAttribute("data-id", deepId);
    await expect(selected(page)).toBeInViewport();
    await snap(page, "deep-selection-after-tab-switch");
  });
});

// "Force" is Strength's French name; the synthetic rows match it in English too.
const forceRows = Array.from({ length: 260 }, (_, i) =>
  row(
    `Force Test ${String(i).padStart(3, "0")}`,
    "Wizard",
    (i % 9) + 1,
    "Alteration",
  ),
);

test.describe("keyboard past a page", () => {
  test.use({ extraSpells: { rows: [...named, ...forceRows] } });

  test("lookup: a French name, then arrow-down past the 200-row page loads more and keeps the selection", async ({
    page,
  }) => {
    await openLookup(page);
    await search(page).fill("force");
    // The synthetic rows, Strength (French "Force") and the fixture's Mordenkainen's Force Missiles.
    const total = forceRows.length + 2;
    await expect(count(page)).toHaveText(`${total} spells`);
    // The French exact match ranks first.
    await expect(selected(page)).toHaveAttribute("data-id", "strength");
    await expect(rows(page)).toHaveCount(200);
    // Near the end of the first page, then down across the boundary from the search box.
    await rows(page).nth(195).scrollIntoViewIfNeeded();
    await rows(page).nth(195).click();
    for (let i = 0; i < 10; i++) await search(page).press("ArrowDown");
    await expect(rows(page)).toHaveCount(total);
    const ids = await attr(rows(page), "data-id");
    await expect(selected(page)).toHaveAttribute("data-id", ids[205]);
    await expect(selected(page)).toBeInViewport();
  });
});

test("lookup: a slow response overtaken by a newer query is not written", async ({
  page,
}) => {
  // The "fir" request is held until the test releases it: no timers, no guessing when it lands.
  let release!: () => void;
  const gate = new Promise<void>((r) => (release = r));
  let held!: () => void;
  const heldSeen = new Promise<void>((r) => (held = r));
  let delivered!: () => void;
  const slowDelivered = new Promise<void>((r) => (delivered = r));
  await page.route("**/api/spells?*", async (route) => {
    if (new URL(route.request().url()).searchParams.get("q") === "fir") {
      held();
      await gate;
      await route.continue().catch(() => {}); // the page may have aborted it
      delivered();
      return;
    }
    await route.continue().catch(() => {});
  });
  await openLookup(page);
  await search(page).fill("fir");
  await heldSeen;
  await search(page).fill("fireball");
  await expect(rows(page)).toHaveCount(1);
  await expect(selected(page)).toHaveAttribute("data-id", "fireball");
  // Let the slow "fir" response land: the rows stay those of "fireball".
  release();
  await slowDelivered;
  // Two frames: whatever the page does with that response has been rendered.
  await page.evaluate(
    () =>
      new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))),
  );
  await expect(rows(page)).toHaveCount(1);
  await expect(count(page)).toHaveText("1 spell");
});

test("lookup: level chips and rows show the bare number; French labels classes, spheres and common values", async ({
  page,
}) => {
  await openLookup(page);
  // The bare level number everywhere, in the chips and in the rows.
  for (let n = 0; n <= 9; n++)
    await expect(levelChip(page, n)).toHaveText(`${n}`);
  await search(page).fill("quixotic");
  await expect(rows(page)).toHaveCount(1);
  await expect(rows(page).first().locator("span").first()).toHaveText("3");
  await expect(detail(page).getByTestId("spell-meta")).toContainText(
    "Wizard · Level 3",
  );
  await snap(page, "bare-levels-en");

  await chooseLanguage(page, "fr");
  for (let n = 0; n <= 9; n++) {
    await expect(
      page.getByRole("button", { name: `Niveau ${n}`, exact: true }),
    ).toHaveText(`${n}`);
  }
  await expect(detail(page).getByTestId("spell-meta")).toContainText(
    "Magicien · Niveau 3",
  );

  // All: the class chip is French; the Priest spell shows its values and sphere in French.
  await classButton(page, "Tous").click();
  await expect(rows(page)).toHaveCount(2);
  await expect(page.getByTestId("spell-class")).toHaveText([
    "Prêtre",
    "Magicien",
  ]);
  await rows(page).filter({ hasText: "Quixotic Probe" }).click();
  const meta = detail(page).getByTestId("spell-meta");
  await expect(meta).toContainText("Prêtre · Niveau 2");
  await expect(meta).toContainText("Soleil");
  await expect(detail(page).getByTestId("stat-range")).toContainText("Contact");
  await snap(page, "french-values");

  // The sphere dropdown: French labels, still filtering on the English API value.
  await classButton(page, "Prêtre").click();
  const sphere = page.getByRole("combobox", { name: "Sphère" });
  await expect(
    sphere.locator("option", { hasText: "Élémentaire (Feu)" }),
  ).toHaveCount(1);
  await expect(
    sphere.locator("option", { hasText: "Elemental Fire" }),
  ).toHaveCount(0);
  await sphere.selectOption({ label: "Soleil" });
  await expect(sphere).toHaveValue("Sun");
  await expect(rows(page)).toHaveCount(1);
  expect(await names(rows(page))).toEqual(["Quixotic Probe"]);
  await snap(page, "french-spheres");
});
