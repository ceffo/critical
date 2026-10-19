import { test, expect, snap } from "./fixtures";
import { chooseLanguage, openDashboard } from "./helpers";
import { expectNoClippedText } from "./noClip";

const VIEWPORTS = [
  { width: 1280, height: 800 },
  { width: 1440, height: 900 },
  { width: 1920, height: 1080 },
  { width: 2560, height: 1440 },
];

// What only a browser proves: the name opens the popover, Edit switches it to a form whose first
// field is the name (Rename lives in the details too, ADR 0034 step 7.2), the server's
// field error shows under its field (the rule itself is a Go test in detailssvc), Save shows the
// stored values, and they survive a reload. Languages are catalogue names, picked from the
// catalogue with the race's own language ticked and fixed (the rules are Go tests); going over the
// Intelligence count warns in the form and on the card and still saves (ADR 0032 step 4).
test("character details: view, edit with a field error, save, reload", async ({
  page,
}) => {
  await openDashboard(page);
  await page.getByTestId("details-trigger").click();
  const pop = page.getByTestId("details-popover");
  await expect(pop).toBeVisible();
  await expect(pop.getByText("Bard", { exact: true })).toBeVisible();
  // The PHB race the rules read (ADR 0030 step 1); there is no free-text race detail any more.
  await expect(pop.getByText("Half-elf", { exact: true })).toBeVisible();
  await expect(pop.getByText("Not set").first()).toBeVisible();

  for (const vp of VIEWPORTS) {
    await page.setViewportSize(vp);
    await expect(pop).toBeVisible();
    await snap(page, `details-view-${vp.width}x${vp.height}`);
  }
  await page.setViewportSize({ width: 1440, height: 900 });

  await pop.getByRole("button", { name: "Edit" }).click();
  await pop.getByLabel("Hair", { exact: true }).fill("Chestnut");
  await pop.getByLabel("Age", { exact: true }).fill("10001");
  await pop.getByRole("button", { name: "Save" }).click();
  await expect(pop.getByRole("alert")).toHaveText(
    "Enter a whole age between 1 and 10,000.",
  );
  await expect(pop.getByLabel("Hair", { exact: true })).toHaveValue("Chestnut");
  await snap(page, "details-error-1440x900");

  await pop.getByLabel("Age", { exact: true }).fill("112");
  await pop.getByLabel("Eyes", { exact: true }).fill("green");
  await expect(pop.getByLabel("Name", { exact: true })).toHaveValue(
    "Test Bard",
  );
  await pop.getByLabel("Name", { exact: true }).fill("Test Bard the Bold");
  const common = pop.getByRole("checkbox", { name: "Common", exact: true });
  await expect(common).toBeChecked();
  await expect(common).toBeDisabled();
  for (const name of ["Elvish", "Gnomish", "Halfling", "Goblin"])
    await pop.getByRole("checkbox", { name, exact: true }).check();
  await pop.getByRole("checkbox", { name: "Orcish", exact: true }).check();
  await snap(page, "details-languages-over-1440x900");
  await pop.getByRole("button", { name: "Save" }).click();
  await expect(pop.getByRole("button", { name: "Edit" })).toBeVisible();
  await expect(pop.getByText("Chestnut")).toBeVisible();
  await expect(pop.getByText("112")).toBeVisible();
  await expect(page.getByTestId("details-trigger")).toHaveText(
    "Test Bard the Bold",
  );
  await expect(
    pop.getByText("Common, Elvish, Gnomish, Halfling, Goblin, Orcish"),
  ).toBeVisible();
  await expect(pop.getByTestId("details-languages-warning")).toHaveText(
    "5 languages beyond the native one, 4 for this Intelligence: the DM decides.",
  );

  // Cancel drops the draft, and an emptied age is cleared (null), not kept.
  await pop.getByRole("button", { name: "Edit" }).click();
  await pop.getByLabel("Hair", { exact: true }).fill("Dwarf");
  await pop.getByRole("button", { name: "Cancel" }).click();
  await pop.getByRole("button", { name: "Edit" }).click();
  await expect(pop.getByLabel("Hair", { exact: true })).toHaveValue("Chestnut");
  await pop.getByLabel("Age", { exact: true }).fill("0");
  await pop.getByRole("button", { name: "Save" }).click();
  await expect(pop.getByRole("alert")).toBeVisible();
  await pop.getByLabel("Age", { exact: true }).fill("");
  await pop.getByRole("button", { name: "Save" }).click();
  await expect(pop.getByRole("button", { name: "Edit" })).toBeVisible();
  await expect(pop.getByText("112")).toHaveCount(0);

  await page.reload();
  await expect(page.getByTestId("hp-value")).toHaveText(/\d+ \/ \d+/);
  await expect(page.getByTestId("details-trigger")).toHaveText(
    "Test Bard the Bold",
  );
  await page.getByTestId("details-trigger").click();
  await expect(pop.getByText("green")).toBeVisible();
  await expect(pop.getByText("112")).toHaveCount(0);
  await expect(
    pop.getByText("Common, Elvish, Gnomish, Halfling, Goblin, Orcish"),
  ).toBeVisible();
  await expect(pop.getByTestId("details-languages-warning")).toBeVisible();
});

// ADR 0032 step 8: a character that lacks its alignment or sex (a migrated one) is asked to pick
// them, in French without cut text; it still saves an unrelated edit, and the picks stay after a reload.
test.describe("a character lacking its alignment and sex", () => {
  test.use({ character: { alignment: "", sex: "" } });

  test("the card asks for them, saves an unrelated edit, then the picks", async ({
    page,
  }) => {
    await openDashboard(page);
    await chooseLanguage(page, "fr");
    await page.getByTestId("details-trigger").click();
    const pop = page.getByTestId("details-popover");
    await expect(pop.getByTestId("details-missing")).toHaveCount(2);
    await expect(pop.getByTestId("details-missing").first()).toHaveText(
      "À choisir",
    );
    await expectNoClippedText(pop, "the card with its identity to pick");
    await snap(page, "details-to-pick-fr");

    await pop.getByRole("button", { name: "Modifier" }).click();
    await pop.getByLabel("Cheveux", { exact: true }).fill("châtain");
    await pop.getByRole("button", { name: "Enregistrer" }).click();
    await expect(pop.getByText("châtain")).toBeVisible();
    await expect(pop.getByTestId("details-missing")).toHaveCount(2);

    await pop.getByRole("button", { name: "Modifier" }).click();
    await pop.getByLabel("Alignement").selectOption({ label: "Neutre bon" });
    await pop.getByLabel("Sexe").selectOption({ label: "Femme" });
    await pop.getByRole("button", { name: "Enregistrer" }).click();
    await expect(pop.getByTestId("details-missing")).toHaveCount(0);

    await page.reload();
    await page.getByTestId("details-trigger").click();
    await expect(pop).toContainText("Neutre bon");
    await expect(pop).toContainText("Femme");
    await expectNoClippedText(pop, "the card with its identity picked");
  });
});

// A rolling deploy (ADR 0024): the page may meet an old server whose character answer has no
// `warnings`/`missing`, and the choices' request can fail. The card still opens; the form says the
// choices did not load and Retry fetches them.
test("details card: an answer without warnings opens, failed choices show an error and retry", async ({
  page,
}) => {
  await page.route("**/api/character", async (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    const res = await route.fetch();
    const body = await res.json();
    delete body.warnings;
    delete body.missing;
    await route.fulfill({ response: res, json: body });
  });
  let fail = true;
  await page.route("**/api/characters/options", (route) =>
    fail ? route.abort() : route.fallback(),
  );
  await openDashboard(page);
  await page.getByTestId("details-trigger").click();
  const pop = page.getByTestId("details-popover");
  await expect(pop.getByText("Bard", { exact: true })).toBeVisible();
  await pop.getByRole("button", { name: "Edit" }).click();
  await expect(pop.getByTestId("details-options-error")).toBeVisible();
  fail = false;
  await pop.getByRole("button", { name: "Retry" }).click();
  await expect(pop.getByTestId("details-options-error")).toHaveCount(0);
  await expect(
    pop.getByLabel("Alignment").getByRole("option", { name: "Neutral good" }),
  ).toHaveCount(1);
});

// A rolling deploy (ADR 0024): an old server's options carry no alignment or sex lists. The
// creation route says the choices did not load, never shows empty pickers, and Retry loads them.
test("creation route: options without the identity lists show an error, retry loads them", async ({
  page,
}) => {
  let old = true;
  await page.route("**/api/characters/options", async (route) => {
    if (!old) return route.fallback();
    const res = await route.fetch();
    const body = await res.json();
    delete body.sexes;
    for (const c of body.classes) delete c.alignments;
    await route.fulfill({ response: res, json: body });
  });
  await openDashboard(page);
  await page.goto("/create");
  await expect(page.getByRole("alert")).toContainText(
    "The choices could not be loaded.",
  );
  old = false;
  await page.getByRole("button", { name: "Retry" }).click();
  await expect(
    page.getByRole("radio", { name: "Bard", exact: true }),
  ).toBeVisible();
});

// A rolling deploy (ADR 0024): the options of a release that predates the language catalogue carry
// none of it. The card still opens and names the stored codes as they are, and its edit offers no
// language picker.
test("details card: options without the language catalogue show no picker", async ({
  page,
}) => {
  await page.route("**/api/characters/options", async (route) => {
    const res = await route.fetch();
    const body = await res.json();
    delete body.languages;
    delete body.raceLanguages;
    await route.fulfill({ response: res, json: body });
  });
  await openDashboard(page);
  await page.getByTestId("details-trigger").click();
  const pop = page.getByTestId("details-popover");
  await expect(pop.getByText("common", { exact: true })).toBeVisible();
  await pop.getByRole("button", { name: "Edit" }).click();
  await expect(pop.getByLabel("Alignment")).toBeVisible();
  await expect(pop.getByTestId("language-picker")).toHaveCount(0);
});
