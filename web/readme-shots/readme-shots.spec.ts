import { mkdir } from "node:fs/promises";
import path from "node:path";
import type { Page } from "@playwright/test";
import { catalogueSpell, test, expect } from "../e2e/fixtures";
import { openDashboard } from "../e2e/helpers";
import {
  focus,
  pickTheme,
  seedTable,
  translator,
  webDir,
} from "../ui-review/table";

/**
 * The README screenshots (docs/frontend.md, "README screenshots"): the app as a player sees it, in the
 * Arcane theme, in English, at 1920x1080, on the e2e stack (in-memory API, the api/testdata fixture:
 * invented characters and spells only, no account, no e-mail, no avatar). Raw PNGs land in the
 * gitignored web/e2e-results/readme-shots/; `just readme-shots` then optimises them into docs/images/.
 * The file names are referenced by the README: keep them stable.
 */

export const rawDir = path.join(webDir, "e2e-results", "readme-shots");

// A character with a name that reads like a player's, not the fixture's "Test Bard".
test.use({
  character: { name: "Lyra Dawnsong" },
  // The fixture catalogue's Fireball carries placeholder text: the shot shows an invented description.
  extraSpells: {
    rows: [
      {
        ...catalogueSpell({
          id: "fireball",
          name: "Fireball",
          class: "Wizard",
          level: 3,
          school: "Evocation",
          casting_time: "3",
          range: "10 yds. + 10 yds./level",
          duration: "Instantaneous",
          area_of_effect: "20-ft. radius",
          components: "V, S, M",
          saving_throw: "1/2",
        }),
        description: {
          en: "A bright streak leaps from your fingertip to a point you choose, then blossoms into a roaring globe of flame. Everything in the burst takes the damage, and the fire spreads around corners to fill the space it can reach.",
        },
      },
    ],
  },
});

/** The build the badge shows: a release version, no "dev" pill and no commit hash of the local build. */
async function mockBuild(page: Page) {
  await page.route("**/api/health", async (route) => {
    const res = await route.fetch();
    const body = (await res.json()) as Record<string, unknown>;
    await route.fulfill({
      json: { ...body, version: VERSION, env: "production" },
    });
  });
}
const VERSION = "1.4.0";

async function shot(page: Page, name: string) {
  await mkdir(rawDir, { recursive: true });
  // No focus ring left from the last click or keystroke.
  await page.evaluate(() =>
    (document.activeElement as HTMLElement | null)?.blur(),
  );
  await page.waitForTimeout(300); // colour transitions, popover animations
  await page.screenshot({
    path: path.join(rawDir, `${name}.png`),
    animations: "disabled",
  });
}

/** Release notes as the changelog endpoint serves them (the fixture's one entry is a placeholder). */
async function mockChangelog(page: Page) {
  const versions = [
    {
      version: "1.4.0",
      features: [
        "Spell slots and prepared spells follow your level automatically",
        "A changelog panel opens from the version badge",
      ],
      fixes: ["The journal keeps its place when you undo an entry"],
    },
    {
      version: "1.3.0",
      features: [
        "Character creation walks you through class, race, scores and proficiencies",
        "Buffs and spell effects expire on their own as the rounds pass",
      ],
      fixes: [],
    },
    {
      version: "1.2.1",
      features: [],
      fixes: ["Healing never takes a character above their maximum HP"],
    },
    {
      version: "1.2.0",
      features: [
        "Roll attacks and saves with a full breakdown of every modifier",
      ],
      fixes: [],
    },
  ];
  await page.route("**/api/changelog**", (route) =>
    route.fulfill({ json: { versions } }),
  );
}

test("README screenshots", async ({ page, stack }) => {
  const t = await translator("en");
  await mockBuild(page);

  // The table in progress: turn 2, effects, damage and healing, a dice roll, a long sword attack.
  await seedTable(page, stack.apiUrl);
  await pickTheme(page, "Dark", "Arcane");
  await focus(page, t, "act");
  await shot(page, "dashboard-dark");

  // The same screen in the light variant of the Arcane theme.
  await pickTheme(page, "Light", "Arcane");
  await shot(page, "dashboard-light");
  await pickTheme(page, "Dark", "Arcane");

  // The spell lookup with a spell open.
  await focus(page, t, "spells");
  await page
    .getByTestId("spell-row")
    .filter({ hasText: "Fireball" })
    .first()
    .click();
  await expect(page.getByTestId("spell-detail")).toContainText("Fireball");
  await shot(page, "spellbook");

  // The character creation flow, on the step where the scores are typed.
  await page.goto("/create");
  const step = page.getByTestId("creation-step");
  await expect(step).toBeVisible();
  await step.getByRole("radio", { name: "Bard", exact: true }).click();
  await step.getByRole("button", { name: "Go to the next step" }).click();
  await step.getByRole("radio", { name: "Half-elf", exact: true }).click();
  await step.getByRole("button", { name: "Go to the next step" }).click();
  await step.getByRole("radio", { name: "Type" }).click();
  for (const [label, score] of [
    ["STR", "10"],
    ["DEX", "15"],
    ["CON", "12"],
    ["INT", "14"],
    ["WIS", "9"],
    ["CHA", "16"],
  ] as const)
    await step.getByLabel(label, { exact: true }).fill(score);
  await shot(page, "creation");

  // The changelog panel, opened from the version badge.
  await mockChangelog(page);
  await openDashboard(page);
  await page.getByTestId("version-open").click();
  await expect(page.getByTestId("changelog-panel")).toBeVisible();
  await expect(page.getByTestId("changelog-version")).toHaveCount(4);
  await shot(page, "changelog");
});
