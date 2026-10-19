import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import type { Page } from "@playwright/test";
import { expect } from "../e2e/fixtures";
import {
  addBuff,
  attackForm,
  chooseSetting,
  chooseTheme,
  chooseWeapon,
  logEntry,
  nextRound,
  openDashboard,
  openWeapons,
  recordHp,
} from "../e2e/helpers";

/**
 * The table the UI-review capture starts from (screens.spec.ts): the seeded game state, the
 * UI-language translator and small navigation helpers.
 */

export const webDir = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
);

type Catalogue = Record<string, unknown>;
/** The UI string at `key` ("a.b.c") in the language's catalogue, with `{{n}}` interpolation. */
export async function translator(lang: "en" | "fr") {
  const file = path.join(webDir, "src", "i18n", "locales", lang, "common.json");
  const cat = JSON.parse(await readFile(file, "utf8")) as Catalogue;
  return (key: string, vars: Record<string, string | number> = {}) => {
    let v: unknown = cat;
    for (const k of key.split(".")) v = (v as Catalogue)[k];
    if (typeof v !== "string") throw new Error(`no ${lang} string for ${key}`);
    return v.replace(/\{\{(\w+)\}\}/g, (_, k: string) => String(vars[k]));
  };
}

export const VIEWPORTS = [
  { width: 1707, height: 1300 },
  { width: 1440, height: 900 },
];
/**
 * Every theme in each mode it exists in (ADR 0026 step 3.1), by its English display name, in the
 * settings popover's order.
 */
export const THEME_MODES = [
  { mode: "Dark", theme: "Arcane" },
  { mode: "Dark", theme: "Deep" },
  { mode: "Dark", theme: "Plain" },
  { mode: "Light", theme: "Arcane" },
  { mode: "Light", theme: "Parchment" },
  { mode: "Light", theme: "Plain" },
] as const;

export const resultCard = (page: Page) =>
  page.getByRole("region", { name: "Detail", exact: true });

async function prepare(apiUrl: string, spells: string[]) {
  const res = await fetch(`${apiUrl}/api/spells/prepared`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ spells }),
  });
  expect(res.ok).toBe(true);
}

/**
 * A table in progress, the same for every theme: turn 2, three effects, a long sword attack as the
 * latest result, damage taken, a dice roll in the journal.
 */
export async function seedTable(page: Page, apiUrl: string) {
  await prepare(apiUrl, [
    "magic_missile",
    "burning_hands",
    "fireball",
    "haste",
    "stoneskin",
    "cloudkill",
  ]);
  await openDashboard(page);
  await page.getByRole("tab", { name: "Spells" }).click();
  await expect(
    page.getByRole("button", { name: "Prepare spells" }),
  ).toBeVisible();
  for (const [name, rounds] of [
    ["Bless", 6],
    ["Haste", 3],
    ["Stoneskin", 0],
    ["Protection From Evil", 10],
    ["Mirror Image", 4],
  ] as const)
    await addBuff(page, {
      name,
      rounds,
      charges: name === "Stoneskin" ? 7 : 0,
    });
  await page.keyboard.press("Escape");
  // addBuff leaves the effects expander open past three rows: back to the header at rest.
  const expander = page.getByTestId("effects-expander");
  if ((await expander.getAttribute("aria-expanded")) === "true")
    await expander.click();
  await expect(page.getByTestId("buff")).toHaveCount(3);
  await nextRound(page);
  // The amount field is cleared when a change lands: wait for each entry before the next.
  await recordHp(page, "damage", 12, "Orc axe");
  await expect(logEntry(page, "Orc axe")).toBeVisible();
  await recordHp(page, "heal", 4, "Potion of healing");
  await expect(logEntry(page, "Potion of healing")).toBeVisible();
  // A free dice roll from the footer.
  const composer = page.getByRole("form", { name: "Dice" });
  await composer.getByTestId("die-6").click();
  await composer.getByTestId("die-6").click();
  await composer.getByRole("button", { name: "Roll", exact: true }).click();
  await expect(resultCard(page).getByTestId("dice-result")).toBeVisible();
  // The attack, latest result.
  await openWeapons(page);
  await chooseWeapon(page, "Long Sword +2");
  await attackForm(page).getByLabel("Target AC").fill("5");
  await attackForm(page)
    .getByRole("button", { name: "Attack", exact: true })
    .click();
  await expect(resultCard(page).getByTestId("attack-result")).toBeVisible();
  await expect(
    page
      .getByRole("region", { name: "Journal", exact: true })
      .getByRole("listitem")
      .first(),
  ).toBeVisible();
}

export type T = Awaited<ReturnType<typeof translator>>;

/** Focus an activity from the rail, in the UI language. */
export async function focus(
  page: Page,
  t: T,
  activity: "act" | "spells" | "xp" | "inventory",
) {
  await page
    .getByRole("navigation", { name: t("shell.activities") })
    .getByRole("button", { name: t(`shell.activity.${activity}`), exact: true })
    .click();
}

/** Record damage or healing through the quick HP entry, in the UI language. */
export async function recordHpT(
  page: Page,
  t: T,
  kind: "damage" | "heal",
  amount: number,
  source: string,
) {
  const box = page.getByRole("group", { name: t("hp.change") });
  await box.getByLabel(t("hp.amount")).fill(String(amount));
  await box.getByLabel(t("hp.source")).fill(source);
  await box
    .getByRole("button", {
      name: t(kind === "damage" ? "hp.applyDamage" : "hp.applyHeal"),
      exact: true,
    })
    .click();
}

export const journal = (page: Page, t: T) =>
  page.getByRole("region", { name: t("log.title"), exact: true });

/** Apply a mode ("Dark" | "Light") and that mode's theme, by their English names. */
export async function pickTheme(
  page: Page,
  mode: "Dark" | "Light",
  theme: string,
) {
  await chooseSetting(page, mode);
  await chooseTheme(page, theme);
  await page.waitForTimeout(150);
}
