import { test, expect, snap } from "./fixtures";
import { openDashboard, openSpend } from "./helpers";

// What only a browser proves for the proficiency pop-up (ADR 0037 step 19.4): the name and the
// ability part of a Skills tab row are two real click targets (the name rolls nothing, the other
// part opens nothing), the description comes from the server's lookup with its
// table rendered as a table, Escape closes it, and the picker's names open the same pop-up with
// the class's cost while adding still works. The text is the fixture's invented one; the lookup
// itself is a Go test, the pop-up's body and the row's two buttons are unit tests.
test("proficiency description: from the Skills tab row and the picker", async ({
  page,
}) => {
  await openDashboard(page);
  await page.getByRole("tab", { name: "Skills" }).click();
  const panel = page.getByRole("tabpanel", { name: "Skills" });
  const card = page.getByRole("region", { name: "Detail", exact: true });
  const dialog = page.getByTestId("proficiency-details");

  // The name opens the pop-up and rolls nothing.
  await panel.getByTestId("proficiency-name-musical_instrument").click();
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole("heading")).toContainText("Musical instrument");
  await expect(dialog.getByTestId("proficiency-description")).toContainText(
    "Invented fixture text for musical instrument.",
  );
  await expect(
    dialog.getByTestId("proficiency-description").getByRole("table"),
  ).toBeVisible();
  await expect(
    dialog.getByTestId("proficiency-figure-ability").locator("dd"),
  ).toHaveText("DEX");
  await expect(
    dialog.getByTestId("proficiency-figure-modifier").locator("dd"),
  ).toHaveText("−1");
  await expect(dialog.getByTestId("proficiency-figure-slots")).toBeVisible();
  await expect(dialog.getByTestId("proficiency-figure-groups")).toBeVisible();
  await expect(card.getByTestId("skill-result")).toHaveCount(0);
  await snap(page, "proficiency-popup-en");
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);

  // The ability part rolls and opens nothing; both targets are reachable by Tab.
  await panel.getByTestId("skill-musical_instrument").click();
  await expect(card.getByTestId("skill-result")).toBeVisible();
  await expect(dialog).toHaveCount(0);
  await panel.getByTestId("proficiency-name-musical_instrument").focus();
  await page.keyboard.press("Tab");
  await expect(panel.getByTestId("skill-musical_instrument")).toBeFocused();

  // The editor's picker: the catalogue's name opens it with the class's cost; adding still adds.
  await openSpend(page, "nonWeapon");
  const editor = page.getByTestId("proficiency-editor");
  await editor.getByTestId("proficiency-search").fill("agri");
  await editor.getByTestId("proficiency-name-agriculture").click();
  await expect(dialog.getByTestId("proficiency-description")).toContainText(
    "Invented fixture text for agriculture.",
  );
  await expect(dialog.getByRole("listitem")).toHaveCount(2);
  await expect(
    dialog.getByTestId("proficiency-figure-cost").locator("dd"),
  ).toHaveText(/^\d+$/);
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
  await editor.getByTestId("proficiency-add-agriculture").click();
  await expect(
    editor.getByTestId("proficiency-pick-agriculture"),
  ).toBeVisible();
  // A pick's own name opens it too.
  await editor.getByTestId("proficiency-name-agriculture").last().click();
  await expect(dialog).toBeVisible();
  await page.keyboard.press("Escape");
  await editor.getByTestId("proficiency-cancel").click();
});

// What only a browser proves for the skill pop-up (ADR 0037 step 19.6): the name and the
// percentage part of a skill-set row in the Skills tab are two real click targets (the name rolls
// nothing, the percentage opens nothing), the description comes from the server's lookup for the
// bard's own set, and the points editor's names open the same pop-up. The text is the fixture's
// invented one; the lookup per set is a Go test, the pop-up's body and the row's two buttons are
// unit tests.
test("skill description: from the Skills tab row and the points editor", async ({
  page,
}) => {
  await openDashboard(page);
  await page.getByRole("tab", { name: "Skills" }).click();
  const panel = page.getByRole("tabpanel", { name: "Skills" });
  const card = page.getByRole("region", { name: "Detail", exact: true });
  const dialog = page.getByTestId("skill-details");

  // The name opens the pop-up and rolls nothing: the bard's text, then the terms of the percentage.
  await panel.getByTestId("skill-name-bard-climb_walls").click();
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole("heading")).toContainText("Climb walls");
  await expect(dialog.getByTestId("skill-description")).toContainText(
    "Invented fixture text for climb_walls of the bard set.",
  );
  await expect(dialog.getByTestId("skill-figure-base")).toBeVisible();
  await expect(dialog.getByTestId("skill-figure-dexterity")).toBeVisible();
  await expect(dialog.getByTestId("skill-figure-armor")).toBeVisible();
  await expect(
    dialog.getByTestId("skill-figure-total").locator("dd"),
  ).toHaveText(/^\d+ ?%$/);
  await expect(card.getByTestId("skill-result")).toHaveCount(0);
  await snap(page, "skill-popup-en");
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);

  // The percentage part still rolls and opens nothing.
  await panel
    .getByTestId("skill-set-bard")
    .getByTestId("skill-climb_walls")
    .click();
  await expect(card.getByTestId("skill-result")).toBeVisible();
  await expect(dialog).toHaveCount(0);

  // The points editor's names open the same pop-up.
  await openSpend(page, "skillPoints");
  await page
    .getByTestId("thief-points-editor")
    .getByTestId("skill-name-bard-detect_noise")
    .click();
  await expect(dialog.getByTestId("skill-description")).toContainText(
    "Invented fixture text for detect_noise of the bard set.",
  );
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
});
