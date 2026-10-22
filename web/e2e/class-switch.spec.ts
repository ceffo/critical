import { test, expect, snap } from "./fixtures";
import { expectNoClippedText } from "./noClip";
import { logEntry, openDashboard, undoEntry } from "./helpers";

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
  });
});
