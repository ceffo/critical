import { test, expect, snap } from "./fixtures";
import { expectNoClippedText } from "./noClip";
import { chooseLanguage, expectSlotDots, openDashboard } from "./helpers";

const VIEWPORTS = [
  { width: 1920, height: 1080 },
  { width: 2560, height: 1440 },
];

// What only a browser proves for a specialist's school slot (ADR 0050 steps 10 and 17.5): the
// level's "1 + 1" from the server's `school`, the preparation editor holding one more spell and the
// cast landing in a slot. Which spell the school slot accepts, the opposed spells and the learn
// chance are Go and component tests (prepsvc, LearnSpell, PrepareEditor); here the wiring, in
// French first, with no cut text at the wide sizes.
test.describe("a human transmuter", () => {
  test.use({
    character: {
      class: "transmuter",
      race: "human",
      alignment: "neutral_good",
      int: 17,
      dex: 16,
      level: 1,
      xp_total: 0,
      skill_points: {},
      proficiencies: [],
      weapon_proficiencies: [],
    },
  });

  test("prepares two level 1 spells, one of his school, and casts into the school slot", async ({
    page,
  }) => {
    await openDashboard(page);
    await chooseLanguage(page, "fr");
    await page.getByRole("tab", { name: "Sorts", exact: true }).click();
    await page.getByRole("button", { name: "Préparer les sorts" }).click();
    const form = page.getByRole("form", { name: "Préparer les sorts du jour" });
    await form
      .getByRole("button", { name: "Ajouter Détection de la magie" })
      .click();
    await form.getByRole("button", { name: "Ajouter Enlarge" }).click();
    // The fixture's book holds Alarm, a spell of an abjuration, which a transmuter opposes: marked, not addable.
    await expect(form.getByTestId("opposed-alarm")).toHaveText("École opposée");
    await expect(
      form.getByRole("button", { name: "Ajouter Alarme" }),
    ).toBeDisabled();
    // The level holds 1 + 1: the school slot is the server's figure, never added by the page.
    await expect(form.getByTestId("school-slot-1")).toHaveText("1 + 1");
    await expect(form.getByTestId("over-budget")).toHaveCount(0);
    for (const vp of VIEWPORTS) {
      await page.setViewportSize(vp);
      await expectNoClippedText(form, "the preparation in French");
      await snap(page, `prepare-fr-${vp.width}x${vp.height}`);
    }
    await page.setViewportSize({ width: 1440, height: 900 });
    await form.getByRole("button", { name: "Enregistrer" }).click();

    const level1 = page.getByRole("region", { name: "Niveau 1" });
    await expect(level1.getByTestId("school-slot-1")).toHaveText("1 + 1");
    await expectSlotDots(level1, 1, 2, 2);
    for (const vp of VIEWPORTS) {
      await page.setViewportSize(vp);
      await expectNoClippedText(level1, "the slots in French");
      await snap(page, `slots-fr-${vp.width}x${vp.height}`);
    }
    await page.setViewportSize({ width: 1440, height: 900 });
    await chooseLanguage(page, "en");
    await page
      .getByRole("region", { name: "Level 1" })
      .getByRole("button", { name: "Cast Enlarge" })
      .click();
    await expectSlotDots(
      page.getByRole("region", { name: "Level 1" }),
      1,
      1,
      2,
    );
    await snap(page, "cast-en");
  });
});
