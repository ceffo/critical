import type { Locator, Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import { openDashboard, chooseLanguage, combatLog } from "./helpers";

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

/** Opens the flow and fills a bard's first five steps (typed scores, HP 6, the picks `proficiencies`
 * makes); stops on the identity step. */
async function toIdentity(
  page: Page,
  lang: "fr" | "en" = "fr",
  extras?: (step: Locator) => Promise<void>,
  proficiencies?: (step: Locator) => Promise<void>,
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
  await expect(step).toHaveAttribute("data-step", "proficiencies");
  await proficiencies?.(step);
  await next.click();
  await expect(step).toHaveAttribute("data-step", "identity");
  return { step, next, tr };
}

test("identity step: forbidden alignment, a roll dropped after a race change, the range warning", async ({
  page,
}) => {
  page.on("dialog", (d) => void d.accept());
  // The languages are picked on the proficiency step (ADR 0034 step 3.5 as amended by ADR 0037
  // step 18), next to the non-weapon picks whose pool they spend: five spend five slots.
  const { step, next } = await toIdentity(page, "fr", undefined, async (s) => {
    const chips = s.getByTestId("language-chips");
    for (const name of ["Elfe", "Gnome", "Petites-gens", "Gobelin", "Orque"])
      await chips.getByRole("checkbox", { name, exact: true }).click();
    await expect(s.getByTestId("languages-slots")).toContainText(
      "5 emplacements",
    );
    await expect(s.getByTestId("proficiency-pool-languages")).toHaveText(
      "Langues : 5 emplacements",
    );
  });
  const details = step.getByTestId("new-character-details");
  await expect(step.getByTestId("language-chips")).toHaveCount(0);
  await details.getByLabel("Nom", { exact: true }).fill("Lyra");
  await details
    .getByRole("radiogroup", { name: "Sexe" })
    .getByRole("radio", { name: "Femme" })
    .click();
  // A forbidden alignment: refused at the field, Suivant blocked.
  await details
    .getByTestId("alignment-grid")
    .getByRole("radio", { name: "Loyal bon : interdit au barde" })
    .click();
  await expect(details.getByTestId("alignment-refusal")).toBeVisible();
  await expect(next).toBeDisabled();

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
  for (let i = 0; i < 4; i++) await back.click();
  await expect(step).toHaveAttribute("data-step", "race");
  await step.getByRole("radio", { name: "Humain", exact: true }).click();
  const answered = page.waitForResponse("**/api/characters/rolls/identity");
  release();
  await answered;
  for (let i = 0; i < 4; i++)
    await step
      .getByRole("button", { name: "Passer à l'étape suivante" })
      .click();
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
  for (let i = 0; i < 6; i++) await next.click();
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

  // The review shows the points beside the final percentage; no armour is entered (the skills
  // read the one worn, ADR 0033 step 14).
  const review = step.getByTestId("review");
  await expect(review.getByTestId("review-skill-armor")).toHaveCount(0);
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

// ADR 0037 steps 12 to 14, what only a browser proves: the picks made in the creation flow reach
// the create request and the new character's Skills tab, the tab's editor saves one more, the
// journal words the move and Undo takes it back. The slots, costs and refusals are Go tests
// (charactersvc, composition), the picker's wording of the server's figures a unit test.
test("proficiencies: create a bard with picks, add one from the Skills tab, undo restores", async ({
  page,
}) => {
  page.on("dialog", (d) => void d.accept());
  const { step, next } = await toIdentity(page, "fr", undefined, async (s) => {
    // A level 1 bard with Intelligence 14: 3 + 0 + 4 slots, the server's terms.
    await expect(s.getByTestId("proficiency-pool-terms")).toContainText("= 7");
    // The server gives Languages, Modern no cost (a language of the list): not offered.
    await s.getByTestId("proficiency-search").fill("langues");
    await expect(s.getByTestId("proficiency-add-languages_modern")).toHaveCount(
      0,
    );
    await s.getByTestId("proficiency-search").fill("acro");
    await s.getByRole("button", { name: /^Ajouter Acrobatie/ }).click();
    await s.getByTestId("proficiency-search").fill("");
    await s
      .getByTestId("proficiency-own")
      .getByRole("button", { name: /^Ajouter Chant/ })
      .click();
    await expect(s.getByTestId("proficiency-to-spend")).toHaveText(
      "5 emplacements à dépenser",
    );
    await expect(page.getByTestId("building-proficiencies")).toHaveText(
      "Acrobatie, Chant",
    );
  });
  const details = step.getByTestId("new-character-details");
  await details.getByLabel("Nom", { exact: true }).fill("Mirel");
  await details
    .getByRole("radiogroup", { name: "Sexe" })
    .getByRole("radio", { name: "Femme" })
    .click();
  await details
    .getByTestId("alignment-grid")
    .getByRole("radio", { name: "Neutre bon", exact: true })
    .click();
  await next.click();
  await step
    .getByRole("button", { name: "Créer ce personnage et le jouer" })
    .click();
  await expect(page.getByTestId("character")).toContainText("Mirel");

  // The new character's Skills tab lists the two picks; its editor adds Juggling.
  await page.getByRole("tab", { name: "Compétences" }).click();
  const panel = page.getByRole("tabpanel", { name: "Compétences" });
  await expect(panel.getByTestId("skill-tumbling")).toBeVisible();
  await expect(panel.getByTestId("skill-singing")).toBeVisible();
  await expect(panel.getByTestId("proficiency-slots-line")).toHaveText(
    "Emplacements : 2 sur 7 · 5 emplacements à dépenser",
  );
  await panel.getByTestId("proficiency-edit").click();
  const editor = panel.getByTestId("proficiency-editor");
  await editor.getByTestId("proficiency-search").fill("jongl");
  await editor.getByRole("button", { name: /^Ajouter Jonglerie/ }).click();
  const spent = editor.getByTestId("proficiency-pool-spent");
  await expect(spent).toHaveText("Dépensés 3, restants 4");
  const juggling = editor.getByTestId("proficiency-pick-juggling");
  const figures = editor.locator("[data-stale]");

  // Previews answer out of order: the one for 2 slots is held back until the one for 3 has
  // answered, and the late answer is not shown.
  const preview = "**/api/proficiencies/preview";
  const isTwo = (body: unknown) =>
    JSON.stringify(body).includes('{"id":"juggling","slots":2}');
  await page.route(preview, async (route) => {
    if (isTwo(route.request().postDataJSON()))
      await new Promise((r) => setTimeout(r, 800));
    await route.continue();
  });
  const late = page.waitForResponse(
    (r) => r.url().endsWith("/preview") && isTwo(r.request().postDataJSON()),
  );
  await juggling.getByTestId("proficiency-more").click();
  await page.waitForRequest(
    (r) => r.url().endsWith("/preview") && isTwo(r.postDataJSON()),
  );
  await juggling.getByTestId("proficiency-more").click();
  await expect(spent).toHaveText("Dépensés 5, restants 2");
  await late;
  await page.waitForTimeout(300); // time for a late answer to land, were it applied
  await expect(spent).toHaveText("Dépensés 5, restants 2");
  await page.unroute(preview);

  // Growing past the budget: the server's refusal, the figures marked out of date, and a save
  // refused the same way keeps the editor open.
  for (let i = 0; i < 3; i++)
    await juggling.getByTestId("proficiency-more").click();
  await expect(editor.getByRole("alert").first()).toContainText(
    "Budget dépassé",
  );
  await expect(figures).toHaveAttribute("data-stale", "true");
  await editor.getByTestId("proficiency-save").click();
  await expect(editor.getByRole("alert").first()).toContainText(
    "Budget dépassé",
  );
  // Below the cost: the refusal lands on Juggling's row, for the preview and the save alike.
  for (let i = 0; i < 6; i++)
    await juggling.getByTestId("proficiency-fewer").click();
  await expect(juggling.getByTestId("proficiency-slots")).toHaveText("0");
  await expect(juggling.getByRole("alert")).toHaveText(
    "Moins d’emplacements que son coût, ou trop : entrez de son coût à 99.",
  );
  await editor.getByTestId("proficiency-save").click();
  await expect(juggling.getByRole("alert")).toHaveText(
    "Moins d’emplacements que son coût, ou trop : entrez de son coût à 99.",
  );
  await expect(editor).toBeVisible();
  // Back at its cost: the refusal and the out-of-date mark go with the next answer.
  await juggling.getByTestId("proficiency-more").click();
  await expect(juggling.getByRole("alert")).toHaveCount(0);
  await expect(figures).toHaveCount(0);
  await expect(spent).toHaveText("Dépensés 3, restants 4");
  await editor.getByTestId("proficiency-save").click();
  await expect(editor).toHaveCount(0);
  await expect(panel.getByTestId("skill-juggling")).toBeVisible();
  await expect(combatLog(page)).toContainText(
    "Emplacements de compétence : Jonglerie 0 → 1",
  );

  // Undo in the journal restores the picks before the save.
  await page.getByTestId("undo").click();
  await expect(panel.getByTestId("skill-juggling")).toHaveCount(0);
  await expect(panel.getByTestId("skill-tumbling")).toBeVisible();
  await expect(panel.getByTestId("proficiency-slots-line")).toHaveText(
    "Emplacements : 2 sur 7 · 5 emplacements à dépenser",
  );
});
