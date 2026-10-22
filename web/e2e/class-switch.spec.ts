import { test, expect, snap } from "./fixtures";
import { expectNoClippedText } from "./noClip";
import {
  apiCall,
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

// What only a browser proves for taking a new class: the details popover's action, the picker and
// the confirmation that says what changes, the sheet and the journal following the server's answer,
// and one Undo putting the character back. Who may switch, the starting level and the XP cut-off are
// Go tests (dualclasssvc, composition); the refusal wordings are the handler's table.
test.describe("a human fighter taking a new class", () => {
  test.use({
    character: {
      class: "fighter",
      race: "human",
      alignment: "neutral_good",
      str: 16,
      int: 17,
      level: 6,
      xp_total: 21000,
      skill_points: {},
      proficiencies: [],
      weapon_proficiencies: [],
    },
  });

  test("the character takes the class, the journal says so, Undo puts it back", async ({
    page,
  }) => {
    await openDashboard(page);
    const card = page.getByRole("complementary", { name: "Character" });
    await expect(card.getByTestId("class")).toHaveText("Fighter");

    // The XP card, open before the switch: it must follow the server's answer without a reload.
    await card.getByRole("button", { name: /^Level \d+$/ }).click();
    const xp = page
      .getByRole("region", { name: "Experience", exact: true })
      .getByTestId("xp-total");
    await expect(xp).toHaveText("21,000");

    await page.getByTestId("details-trigger").click();
    await page.getByTestId("take-class-open").click();
    const dialog = page.getByTestId("take-class-dialog");
    // The class the character already has is not offered.
    await expect(
      dialog.getByTestId("take-class-pick").locator("option", {
        hasText: /^Fighter$/,
      }),
    ).toHaveCount(0);
    await dialog.getByTestId("take-class-pick").selectOption("mage");
    await snap(page, "pick");
    await dialog.getByTestId("take-class-next").click();
    await expect(dialog.getByTestId("take-class-confirm")).toContainText(
      "Fighter is frozen at level 6",
    );
    await expect(dialog.getByTestId("take-class-confirm")).toContainText(
      "all XP goes to Mage",
    );
    for (const vp of VIEWPORTS) {
      await page.setViewportSize(vp);
      await expectNoClippedText(dialog, "the confirmation");
      await snap(page, `confirm-${vp.width}x${vp.height}`);
    }
    await page.setViewportSize({ width: 1440, height: 900 });
    await dialog.getByTestId("take-class-confirm-button").click();
    await expect(dialog).toHaveCount(0);

    await expect(card.getByTestId("class-entry-fighter")).toHaveAttribute(
      "data-state",
      "frozen",
    );
    await expect(card.getByTestId("class-level-mage")).toHaveText("Level 1");
    await expect(xp).toHaveText("0");
    // The journal lives in the Act workspace.
    await openWorkspace(page, "Act");
    const entry = logEntry(page, "Mage");
    await expect(entry).toContainText("Took a new class: Mage");
    for (const vp of VIEWPORTS) {
      await page.setViewportSize(vp);
      await snap(page, `switched-${vp.width}x${vp.height}`);
    }
    await page.setViewportSize({ width: 1440, height: 900 });

    await undoEntry(page, "Took a new class");
    await expect(card.getByTestId("class")).toHaveText("Fighter");
    await expect(card.getByTestId("class-entries")).toHaveCount(0);
    await expect(logEntry(page, "Took a new class")).toContainText("Undone");
    await openWorkspace(page, "Experience");
    await expect(xp).toHaveText("21,000");

    await openWorkspace(page, "Act");
    await page.getByTestId("redo").click();
    await expect(card.getByTestId("class-level-mage")).toHaveText("Level 1");
    await openWorkspace(page, "Experience");
    await expect(xp).toHaveText("0");
  });
});

// The refusals show in the dialog in the player's language (French first, ADR 0027).
test.describe("a character who may not take a class", () => {
  test.use({
    character: {
      class: "fighter",
      race: "elf",
      alignment: "neutral_good",
      str: 16,
      int: 17,
      level: 6,
      xp_total: 21000,
      skill_points: {},
      proficiencies: [],
      weapon_proficiencies: [],
    },
  });

  test("an elf is accepted with the lift, refused without it, in French", async ({
    page,
  }) => {
    await openDashboard(page);
    await chooseLanguage(page, "fr");
    const card = page.getByRole("complementary", { name: "Personnage" });
    // The racial lift is off: the server refuses and the dialog says why.
    expect(
      await apiCall(page, "PUT", "/api/ruleset/racial-lift", {
        multiclassAnyRace: true,
        dualclassAnyRace: false,
      }),
    ).toBe(true);
    await page.getByTestId("details-trigger").click();
    await page.getByTestId("take-class-open").click();
    const dialog = page.getByTestId("take-class-dialog");
    await dialog.getByTestId("take-class-pick").selectOption("mage");
    await dialog.getByTestId("take-class-next").click();
    await dialog.getByTestId("take-class-confirm-button").click();
    await expect(dialog.getByTestId("take-class-refusal")).toContainText(
      "ne peut pas devenir double-classé",
    );
    await snap(page, "refusal-fr");
    await expectNoClippedText(dialog, "the refusal");
    // Nothing changed.
    await dialog.getByRole("button", { name: "Annuler" }).click();
    await expect(card.getByTestId("class-entries")).toHaveCount(0);

    // The lift on again (the default): the same elf is accepted.
    expect(
      await apiCall(page, "PUT", "/api/ruleset/racial-lift", {
        multiclassAnyRace: true,
        dualclassAnyRace: true,
      }),
    ).toBe(true);
    await page.getByTestId("details-trigger").click();
    await page.getByTestId("take-class-open").click();
    await dialog.getByTestId("take-class-pick").selectOption("mage");
    await dialog.getByTestId("take-class-next").click();
    await snap(page, "confirm-fr");
    await dialog.getByTestId("take-class-confirm-button").click();
    await expect(dialog).toHaveCount(0);
    await expect(card.getByTestId("class-entry-fighter")).toHaveAttribute(
      "data-state",
      "frozen",
    );
  });
});

test.describe("a human whose new prime requisite is too low", () => {
  test.use({
    character: {
      class: "fighter",
      race: "human",
      alignment: "neutral_good",
      str: 16,
      int: 16,
      level: 6,
      xp_total: 21000,
      skill_points: {},
      proficiencies: [],
      weapon_proficiencies: [],
    },
  });

  test("the dialog shows the refusal and nothing changes", async ({ page }) => {
    await openDashboard(page);
    const card = page.getByRole("complementary", { name: "Character" });
    await page.getByTestId("details-trigger").click();
    await page.getByTestId("take-class-open").click();
    const dialog = page.getByTestId("take-class-dialog");
    await dialog.getByTestId("take-class-pick").selectOption("mage");
    await dialog.getByTestId("take-class-next").click();
    await dialog.getByTestId("take-class-confirm-button").click();
    await expect(dialog.getByTestId("take-class-refusal")).toBeVisible();
    await snap(page, "refusal-en");
    await dialog.getByRole("button", { name: "Cancel" }).click();
    await expect(card.getByTestId("class")).toHaveText("Fighter");
    await expect(card.getByTestId("class-entries")).toHaveCount(0);
    await expect(logEntry(page, "Took a new class")).toHaveCount(0);
  });
});
