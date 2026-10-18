import { test, expect, snap } from "./fixtures";
import { chooseLanguage, openDashboard } from "./helpers";
import { expectNoClippedText } from "./noClip";

const VIEWPORTS = [
  { width: 1280, height: 800 },
  { width: 1440, height: 900 },
  { width: 1920, height: 1080 },
  { width: 2560, height: 1440 },
];

// What only a browser proves: the name opens the popover, Edit switches it to a form, the server's
// field error shows under its field (the rule itself is a Go test in detailssvc), Save shows the
// stored values, and they survive a reload. Languages are shown translated
// where the catalogue has the name, as entered otherwise (the seeded names are a Go test).
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
  await pop
    .getByLabel("Languages", { exact: true })
    .fill("Commun, Nain, Sylvain");
  await pop.getByRole("button", { name: "Save" }).click();
  await expect(pop.getByRole("button", { name: "Edit" })).toBeVisible();
  await expect(pop.getByText("Chestnut")).toBeVisible();
  await expect(pop.getByText("112")).toBeVisible();
  await expect(pop.getByText("Common, Dwarvish, Sylvain")).toBeVisible();

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
  await page.getByTestId("details-trigger").click();
  await expect(pop.getByText("green")).toBeVisible();
  await expect(pop.getByText("112")).toHaveCount(0);
  await expect(pop.getByText("Common, Dwarvish, Sylvain")).toBeVisible();
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
