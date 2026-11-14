import { test, expect, snap } from "./fixtures";
import { expectNoClippedText } from "./noClip";
import {
  chooseLanguage,
  logEntry,
  openDashboard,
  openWorkspace,
  undoEntry,
} from "./helpers";

const VIEWPORTS = [
  { width: 1920, height: 1080 },
  { width: 2560, height: 1440 },
];

// What only a browser proves for switching an entry's class (ADR 0050 step 15): the details
// popover's one-word action, the dialog's picker and its confirmation (in French first, with no cut
// text at the wide sizes), the sheet and the journal following the server's answer, and one Undo
// putting the class back. Who may switch, what is kept and the opposed spells' slots are Go tests
// (classentrysvc, composition); the refusal wordings are the handler's table.
test.describe("an elf mage becoming a transmuter", () => {
  test.use({
    character: {
      class: "mage",
      race: "elf",
      alignment: "neutral_good",
      int: 17,
      level: 6,
      xp_total: 40000,
      skill_points: {},
      proficiencies: [],
      weapon_proficiencies: [],
    },
  });

  test("the class changes with the race warned, the journal says so, Undo puts it back", async ({
    page,
  }) => {
    await openDashboard(page);
    const card = page.getByRole("complementary", { name: "Character" });
    await expect(card.getByTestId("class")).toHaveText("Mage");

    // The French wording first: the picker, then the confirmation with the race warning.
    await chooseLanguage(page, "fr");
    await page.getByTestId("details-trigger").click();
    await expect(page.getByTestId("switch-class-open")).toHaveText("Changer");
    await page.getByTestId("switch-class-open").click();
    const dialog = page.getByTestId("switch-class-dialog");
    await dialog.getByTestId("switch-class-pick").selectOption("transmuter");
    await snap(page, "pick-fr");
    await dialog.getByTestId("switch-class-next").click();
    await expect(dialog.getByTestId("switch-class-confirm")).toContainText(
      "Magicien devient Transmutateur",
    );
    await expect(dialog.getByTestId("switch-class-race-warning")).toContainText(
      "table 22",
    );
    for (const vp of VIEWPORTS) {
      await page.setViewportSize(vp);
      await expectNoClippedText(dialog, "the confirmation in French");
      await snap(page, `confirm-fr-${vp.width}x${vp.height}`);
    }
    await page.setViewportSize({ width: 1440, height: 900 });
    await dialog.getByRole("button", { name: "Retour" }).click();
    await dialog.getByRole("button", { name: "Annuler" }).click();
    await expect(dialog).toHaveCount(0);
    await chooseLanguage(page, "en");

    await page.getByTestId("details-trigger").click();
    await page.getByTestId("switch-class-open").click();
    await dialog.getByTestId("switch-class-pick").selectOption("transmuter");
    await dialog.getByTestId("switch-class-next").click();
    await expect(dialog.getByTestId("switch-class-confirm")).toContainText(
      "Mage becomes Transmuter",
    );
    for (const vp of VIEWPORTS) {
      await page.setViewportSize(vp);
      await expectNoClippedText(dialog, "the confirmation");
      await snap(page, `confirm-${vp.width}x${vp.height}`);
    }
    await page.setViewportSize({ width: 1440, height: 900 });
    await dialog.getByTestId("switch-class-confirm-button").click();
    await expect(dialog).toHaveCount(0);

    await expect(card.getByTestId("class")).toHaveText("Transmuter");
    // The journal lives in the Act workspace.
    await openWorkspace(page, "Act");
    await expect(logEntry(page, "Transmuter")).toContainText(
      "Class changed: Mage to Transmuter",
    );
    for (const vp of VIEWPORTS) {
      await page.setViewportSize(vp);
      await snap(page, `switched-${vp.width}x${vp.height}`);
    }
    await page.setViewportSize({ width: 1440, height: 900 });

    await undoEntry(page, "Class changed");
    await expect(card.getByTestId("class")).toHaveText("Mage");
    await expect(logEntry(page, "Class changed")).toContainText("Undone");
  });
});
