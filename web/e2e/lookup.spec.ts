import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import {
  test,
  expect,
  snap,
  catalogueSpell,
  type ExtraSpell,
} from "./fixtures";
import {
  buffBadge,
  chooseLanguage,
  chooseSetting,
  openDashboard,
  openSortsPowers,
  openWorkspace,
} from "./helpers";
import { psionicist } from "./psionicist";
import type { Page } from "@playwright/test";
import { expectWholeWords } from "./noClip";

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
/** How many spells the catalogue under test holds: an injected spell replaces the fixture's of the
 * same id (fixtures.ts catalogueWith). */
const spellCount = (...lists: unknown[][]) =>
  new Set(lists.flat().map((s) => (s as { id: string }).id)).size;

/** A catalogue spell appended to the fixture for these tests; its id is the slug of its name. */
function row(
  name: string,
  cls: "Wizard" | "Priest",
  level: number,
  school: string,
  sphere = "",
  extra: Partial<ExtraSpell> = {},
) {
  const codes: Record<string, string> = {
    Abjuration: "abjuration",
    Alteration: "alteration",
    Dimension: "alteration",
    Divination: cls === "Priest" ? "lesser_divination" : "greater_divination",
    Evocation: "invocation_evocation",
    "Illusion/Phantasm": "illusion",
    Necromancy: "necromancy",
  };
  const schools = school.split(", ").map((n) => codes[n]);
  const id = name.toLowerCase().replace(/[^a-z0-9]+/g, "_");
  return catalogueSpell({
    id,
    name,
    class: cls,
    level,
    school,
    schools,
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
      names: {
        fr: "Nom de sortilège invraisemblablement long des anciens archimages",
      },
    },
  ),
  row("Quixotic Oddity", "Wizard", 3, "Divination", "", {
    range: "Sight line",
    duration: "Instantaneous",
    area_of_effect: "A weird zone",
  }),
];

test.use({ extraSpells: { rows: named } });

// The fixture character is a bard with a spellbook, so the lookup opens on "Mine"; the specs that
// browse the catalogue switch to Wizard here (`opening` keeps the opening view).
async function openLookup(page: Page, opening = false) {
  await openDashboard(page);
  await openWorkspace(page, "Spells");
  await expect(search(page)).toBeVisible();
  await expect(rows(page).first()).toBeVisible();
  if (opening) return;
  await classButton(page, "Wizard").click();
  await expect(classButton(page, "Wizard")).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await expect(count(page)).toBeVisible();
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

  // Add as effect (ADR 0004 steps 19 and 20): offered for a spell that gets a row, as Project
  // Image does (it lasts rounds); Mirror Image goes on the Bard at his level and shows in the
  // header's effects timeline.
  await expect(detail(page).getByTestId("spell-add-effect")).toHaveCount(1);
  await search(page).fill("mirror image");
  await expect(selected(page)).toHaveAttribute("data-id", "mirror_image");
  await detail(page)
    .getByRole("button", { name: "Track as effect: Mirror Image, level 15" })
    .click();
  await expect(buffBadge(page, "Mirror Image")).toBeVisible();
  await snap(page, "add-as-effect");
});

test("lookup in French: a French name finds the spell, its French text shows with its table, its stats in French and metres; a spell without French shows English", async ({
  page,
}) => {
  await openLookup(page);
  await chooseLanguage(page, "fr");
  // A language change reloads the page, which opens on Mine again.
  await classButton(page, "Magicien").click();
  const searchFrench = page.getByRole("combobox", {
    name: "Rechercher des sorts...",
  });
  await searchFrench.fill("orbe chrom");
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
  await searchFrench.fill("projectile magique");
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
  await searchFrench.fill("rapidit");
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

  await searchFrench.fill("armor");
  await expect(detail(page).getByRole("heading")).toHaveText("Armor");
  await expect(text).toContainText("Fixture text for Armor");
  await expect(stat("castingTime")).toContainText("1 tour");
  await expect(stat("range")).toContainText("Contact");
  await snap(page, "french-english-fallback");
});

const LONG =
  "Comprehensively Unreasonably Long Spell Name Of The Ancient Archmages";
const LONG_FR =
  "Nom de sortilège invraisemblablement long des anciens archimages";

for (const size of [{ width: 1440, height: 900 }]) {
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
        // No word is broken at all, by hyphenation or otherwise (ADR 0022 step 1: a word split
        // mid-word reads as cut): the name wraps between words, and with no emergency break every
        // word still fits the box (so break-words is never what splits one).
        await expect(el).toHaveAttribute("lang", lang);
        await expect(el).toHaveCSS("hyphens", "manual");
        await expectWholeWords(el, "spell name");
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
    const total = spellCount(fixtureSpells, named, many);
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

// ADR 0043 step 8: Sorts > Powers browses the psionic powers (a psionicist only). Only a browser proves the
// tab, the search over both languages and the detail in the player's
// language; the ranking, the filters' rules and the costs are the API's Go tests.
test.describe("lookup powers", () => {
  test.use({ character: psionicist });
  test("a power is found by its English or French name and its detail follows the language", async ({
    page,
  }) => {
    await openDashboard(page);
    await openSortsPowers(page);
    const powerRows = page.getByTestId("power-row");
    await expect(powerRows).toHaveCount(19);
    await expect(page.getByTestId("lookup-count")).toHaveText("19 powers");
    await powerRows.first().click();

    await page
      .getByRole("combobox", { name: "Search powers..." })
      .fill("aura sight");
    await expect(powerRows).toHaveCount(1);
    await powerRows.first().click();
    const detail = page.getByTestId("power-detail");
    await expect(detail.getByRole("heading")).toHaveText("Aura Sight");
    await expect(detail.getByTestId("power-meta")).toHaveText(
      "Clairsentience · Science",
    );
    await expect(detail.getByTestId("power-stat-initialCost")).toContainText(
      "9 PSP",
    );
    await expect(detail.getByTestId("power-stat-score")).toContainText(
      "WIS −5",
    );
    await expect(detail.getByTestId("power-description-text")).toContainText(
      "Placeholder text for the test catalogue: what Aura Sight does",
    );
    await snap(page, "power-detail");

    // A discipline narrows the list; the French name finds the power in French.
    await chooseLanguage(page, "fr");
    await page
      .getByRole("combobox", { name: "Rechercher des pouvoirs..." })
      .fill("vision");
    await expect(powerRows).toHaveCount(1);
    await powerRows.first().click();
    await expect(detail.getByRole("heading")).toHaveText("Vision de l'Aura");
    await expect(detail.getByTestId("power-meta")).toHaveText(
      "Clairsentience · Science",
    );
    await expect(detail.getByTestId("power-stat-initialCost")).toContainText(
      "9 PFP",
    );
    await expect(detail.getByTestId("power-description-text")).toContainText(
      "Texte de test du catalogue : ce que fait Vision de l'Aura",
    );
    await snap(page, "power-detail-fr");

    // A power with an effect shows it in the buff form's words, with its condition.
    await page
      .getByRole("combobox", { name: "Rechercher des pouvoirs..." })
      .fill("greffe");
    await expect(powerRows).toHaveCount(1);
    await powerRows.first().click();
    await expect(detail.getByRole("heading")).toHaveText("Greffe d'arme");
    await expect(detail.getByTestId("power-effect")).toHaveText(
      "Effet sur vous Toucher +1, Dégâts +1 (attaques au corps à corps seulement)",
    );
    await snap(page, "power-effect-fr");
  });
});
