import type { Locator, Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import { openDashboard, chooseLanguage } from "./helpers";

// The identity step and the review of the creation route (ADR 0034 steps 3.5 and 3.6): what only a
// browser proves is the timing and the navigation around the server's replies (a late roll, a stale
// preview, a refusal landing on an earlier step, the API down). The rules behind them (the body
// sent, the blocked reasons, the step of a field, the reply gate) are unit tests; the preview's
// verdicts are Go tests (composition preview_test.go).

const scores = [
  ["FOR", "STR", "10"],
  ["DEX", "DEX", "15"],
  ["CON", "CON", "12"],
  ["INT", "INT", "14"],
  ["SAG", "WIS", "9"],
  ["CHA", "CHA", "16"],
] as const;

/** Opens the flow and fills a bard's first four steps (typed scores, HP 6); stops on the identity step. */
async function toIdentity(
  page: Page,
  lang: "fr" | "en" = "fr",
  extras?: (step: Locator) => Promise<void>,
) {
  await openDashboard(page);
  await chooseLanguage(page, lang);
  await page.goto("/create");
  const step = page.getByTestId("creation-step");
  const next = step.getByRole("button", {
    name: lang === "fr" ? "Passer à l'étape suivante" : "Go to the next step",
  });
  const tr = (fr: string, en: string) => (lang === "fr" ? fr : en);
  await step
    .getByRole("radio", { name: tr("Barde", "Bard"), exact: true })
    .click();
  await next.click();
  await step
    .getByRole("radio", { name: tr("Demi-elfe", "Half-elf"), exact: true })
    .click();
  await next.click();
  await step.getByRole("radio", { name: tr("Saisir", "Type") }).click();
  for (const [fr, en, score] of scores)
    await step.getByLabel(tr(fr, en), { exact: true }).fill(score);
  await next.click();
  await step.getByLabel(tr("PV max", "Max HP")).fill("6");
  await extras?.(step);
  await next.click();
  await expect(step).toHaveAttribute("data-step", "identity");
  return { step, next, tr };
}

test("identity step: forbidden alignment, a roll dropped after a race change, the range warning", async ({
  page,
}) => {
  page.on("dialog", (d) => void d.accept());
  const { step, next } = await toIdentity(page);
  const details = step.getByTestId("new-character-details");
  await details.getByLabel("Nom", { exact: true }).fill("Lyra");
  await details
    .getByRole("radiogroup", { name: "Sexe" })
    .getByRole("radio", { name: "Femme" })
    .click();
  // A forbidden alignment: refused at the field, Suivant blocked, the server's other warnings still
  // come (five languages over the count).
  await details
    .getByTestId("alignment-grid")
    .getByRole("radio", { name: "Loyal bon : interdit au barde" })
    .click();
  await expect(details.getByTestId("alignment-refusal")).toBeVisible();
  await expect(next).toBeDisabled();
  const chips = details.getByTestId("language-chips");
  for (const name of ["Elfe", "Gnome", "Petites-gens", "Gobelin", "Orque"])
    await chips.getByRole("checkbox", { name, exact: true }).click();
  await expect(details.getByTestId("languages-warning")).toBeVisible();
  await expect(details.getByTestId("alignment-refusal")).toBeVisible();

  // A double click on Tirer sends one reply that counts; a race changed while the roll is in
  // flight drops the late reply.
  let release: () => void = () => {};
  const held = new Promise<void>((r) => (release = r));
  await page.route("**/api/characters/rolls/identity", async (route) => {
    await held;
    await route.continue();
  });
  const roll = details.getByTestId("details-roll-height");
  await roll.click();
  await roll.click({ force: true });
  const back = step.getByRole("button", {
    name: /^Revenir à l'étape précédente/,
  });
  for (let i = 0; i < 3; i++) await back.click();
  await expect(step).toHaveAttribute("data-step", "race");
  await step.getByRole("radio", { name: "Humain", exact: true }).click();
  const answered = page.waitForResponse("**/api/characters/rolls/identity");
  release();
  await answered;
  await step.getByRole("button", { name: "Passer à l'étape suivante" }).click();
  await step.getByRole("button", { name: "Passer à l'étape suivante" }).click();
  await step.getByRole("button", { name: "Passer à l'étape suivante" }).click();
  await expect(step).toHaveAttribute("data-step", "identity");
  await expect(details.getByRole("textbox", { name: "Taille" })).toHaveValue(
    "",
  );
  await expect(
    details.getByTestId("details-roll-height-breakdown"),
  ).toHaveCount(0);

  // A typed height outside the race's range: the server's warning flags it, the web does not.
  await details.getByRole("textbox", { name: "Taille" }).fill("300");
  await expect(details.getByTestId("height-warning")).toBeVisible();
});

test("identity step in English: the exact roll is sent, a typed height replaces it, a changed sex hides the breakdown", async ({
  page,
}) => {
  page.on("dialog", (d) => void d.accept());
  const { step, next } = await toIdentity(page, "en");
  const details = step.getByTestId("new-character-details");
  await details.getByLabel("Name", { exact: true }).fill("Lyra");
  await details
    .getByRole("radiogroup", { name: "Sex" })
    .getByRole("radio", { name: "Female", exact: true })
    .click();
  await details
    .getByTestId("alignment-grid")
    .getByRole("radio", { name: /^[^:]+$/ })
    .first()
    .click();
  const height = details.getByRole("textbox", { name: "Height" });
  const rolled = page.waitForResponse("**/api/characters/rolls/identity");
  await details.getByTestId("details-roll-height").click();
  const exact = (await (await rolled).json()).rolls[0].value as number;
  await expect(height).toHaveValue(/^\d+'\d+"$/);

  // A sex changed after the roll hides its breakdown; the range check stays the server's.
  await expect(
    details.getByTestId("details-roll-height-breakdown"),
  ).toBeVisible();
  await details
    .getByRole("radiogroup", { name: "Sex" })
    .getByRole("radio", { name: "Male", exact: true })
    .click();
  await expect(
    details.getByTestId("details-roll-height-breakdown"),
  ).toHaveCount(0);
  await details
    .getByRole("radiogroup", { name: "Sex" })
    .getByRole("radio", { name: "Female", exact: true })
    .click();
  await expect(height).toHaveValue(/^\d+'\d+"$/);

  // Créer is answered by a refusal that names no field, so the body is captured twice and nothing
  // is created: the roll shown in feet and inches goes out as the exact figure the server rolled,
  // then a height typed by hand goes out as typed (5'10" is 177.8 cm).
  const sent: { details: { height_cm?: number } }[] = [];
  await page.route("**/api/characters", async (route) => {
    if (route.request().method() !== "POST") return route.continue();
    sent.push(route.request().postDataJSON());
    await route.fulfill({
      status: 400,
      contentType: "application/json",
      body: JSON.stringify({ error: "refused for the test" }),
    });
  });
  const create = step.getByRole("button", {
    name: "Create this character and play it",
  });
  await next.click();
  await create.click();
  await expect(step.getByText("refused for the test")).toBeVisible();
  expect(sent[0]!.details.height_cm).toBe(exact);
  await step.getByRole("button", { name: "Edit: Identity" }).click();
  await height.fill(`5'10"`);
  await next.click();
  await create.click();
  await expect.poll(() => sent.length).toBe(2);
  expect(sent[1]!.details.height_cm).toBe(177.8);
});

test("review: no stale figures after a change, a refusal from an earlier step lands there with the draft kept, the preview down", async ({
  page,
}) => {
  page.on("dialog", (d) => void d.accept());
  const { step, next } = await toIdentity(page);
  const details = step.getByTestId("new-character-details");
  await details.getByLabel("Nom", { exact: true }).fill("Lyra");
  await details
    .getByRole("radiogroup", { name: "Sexe" })
    .getByRole("radio", { name: "Femme" })
    .click();
  await details
    .getByTestId("alignment-grid")
    .getByRole("radio", { name: /^[^:]+$/ })
    .first()
    .click();
  await next.click();
  const thac0 = step.getByTestId("review-thac0");
  await expect(thac0).toHaveText(/^\d+$/);
  const level1 = await thac0.innerText();

  // A class level changed from the review: the old figures are gone until the new reply lands.
  await step.getByRole("button", { name: "Modifier : Classe" }).click();
  await step.getByRole("radio", { name: "Niveau 9", exact: true }).click();
  let release: () => void = () => {};
  const held = new Promise<void>((r) => (release = r));
  await page.route("**/api/characters/preview", async (route) => {
    await held;
    await route.continue();
  });
  for (let i = 0; i < 5; i++) await next.click();
  await expect(step).toHaveAttribute("data-step", "review");
  await expect(thac0).toHaveText("…");
  await expect(step.getByTestId("next-blocked")).not.toContainText(
    "rien de bloquant",
  );
  release();
  await expect(thac0).toHaveText(/^\d+$/);
  expect(await thac0.innerText()).not.toBe(level1);
  await page.unroute("**/api/characters/preview");

  // Créer refused on a field of an earlier step: the flow lands there with the error, and the
  // draft is intact when it goes forward again.
  let refuse = true;
  await page.route("**/api/characters", async (route) => {
    if (route.request().method() !== "POST" || !refuse) return route.continue();
    refuse = false;
    await route.fulfill({
      status: 400,
      contentType: "application/json",
      body: JSON.stringify({ error: "out_of_range", field: "hpMax" }),
    });
  });
  const create = step.getByRole("button", {
    name: "Créer ce personnage et le jouer",
  });
  await create.click();
  await expect(step).toHaveAttribute("data-step", "extras");
  await expect(step.getByLabel("PV max")).toHaveValue("6");
  await next.click();
  await expect(step).toHaveAttribute("data-step", "identity");
  await expect(details.getByLabel("Nom", { exact: true })).toHaveValue("Lyra");
  await next.click();

  // The preview unreachable: the review says so, and Créer still answers for itself.
  await page.route("**/api/characters/preview", (route) => route.abort());
  await step.getByRole("button", { name: "Modifier : Identité" }).click();
  await details.getByLabel("Nom", { exact: true }).fill("Lyra II");
  await next.click();
  await expect(thac0).toHaveText("…");
  await expect(step.getByTestId("next-blocked")).toBeVisible();
  await create.click();
  await expect(page.getByTestId("character")).toContainText("Lyra II");
});

test("identity and review: arrows choose, an unreadable weight blocks at its field, the review shows the skill choices", async ({
  page,
}) => {
  page.on("dialog", (d) => void d.accept());
  const { step, next } = await toIdentity(page, "fr", async (extras) => {
    await extras
      .getByLabel("Armure portée")
      .selectOption({ label: "Métal, jusqu’à la maille" });
    await extras.getByLabel("Escalade").fill("15");
  });
  const details = step.getByTestId("new-character-details");
  await details.getByLabel("Nom", { exact: true }).fill("Lyra");

  // Radio groups: one Tab stop, arrows choose and focus the neighbour.
  const sex = details.getByRole("radiogroup", { name: "Sexe" });
  const radios = sex.getByRole("radio");
  await expect(radios.first()).toHaveAttribute("tabindex", "0");
  await expect(radios.nth(1)).toHaveAttribute("tabindex", "-1");
  await radios.first().focus();
  await page.keyboard.press("ArrowRight");
  await expect(radios.nth(1)).toHaveAttribute("aria-checked", "true");
  await expect(radios.nth(1)).toBeFocused();
  await page.keyboard.press("ArrowLeft");
  await expect(radios.first()).toHaveAttribute("aria-checked", "true");
  const grid = details.getByTestId("alignment-grid").getByRole("radio");
  await grid.nth(1).click();
  await page.keyboard.press("ArrowDown");
  await expect(grid.nth(2)).toHaveAttribute("aria-checked", "true");
  await grid.nth(1).click();

  // A weight no figure: the error shows at its field now, and Suivant waits.
  await details.getByRole("textbox", { name: "Poids" }).fill("heavy");
  await expect(
    details.getByText("Saisissez le poids en nombre."),
  ).toBeVisible();
  await expect(next).toBeDisabled();
  await expect(step.getByTestId("next-blocked")).toHaveText(
    "Corrigez la valeur illisible pour continuer.",
  );
  await details.getByRole("textbox", { name: "Poids" }).fill("");
  await expect(next).toBeEnabled();
  await next.click();

  // The review shows the armour worn and the points beside the final percentage.
  const review = step.getByTestId("review");
  await expect(review.getByTestId("review-skill-armor")).toHaveText(
    "Métal, jusqu’à la maille",
  );
  await expect(review.getByTestId("review-skill-climb_walls")).toHaveText(
    /\d+ % \(15 points\)/,
  );
  await snap(page, "creation-review-skills-fr");
});

test("extras: skill points typed as no number show at their field and block Next there", async ({
  page,
}) => {
  page.on("dialog", (d) => void d.accept());
  const { step } = await toIdentity(page, "fr", async (extras) => {
    const next = extras.getByRole("button", {
      name: "Passer à l'étape suivante",
    });
    await extras.getByLabel("Escalade").fill("heavy");
    await expect(
      extras.getByText("Saisissez un nombre entier", { exact: false }).first(),
    ).toBeVisible();
    await expect(next).toBeDisabled();
    await expect(extras.getByTestId("next-blocked")).toHaveText(
      "Corrigez la valeur illisible pour continuer.",
    );
    await extras.getByLabel("Escalade").fill("15");
    await expect(next).toBeEnabled();
  });
  await expect(step).toHaveAttribute("data-step", "identity");
});
