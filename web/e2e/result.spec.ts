import { test, expect, snap } from "./fixtures";
import { openDashboard, openWorkspace } from "./helpers";

test("an attack shows its full breakdown in the result card", async ({
  page,
}) => {
  await openDashboard(page);
  const result = page.getByRole("region", { name: "Last Result" });
  await expect(result).toContainText("Roll an attack or a save");

  await openWorkspace(page, "Act");
  await page.getByRole("tab", { name: "Attack" }).click();
  await page.getByLabel("Target AC").fill("4");
  await page.getByRole("button", { name: "Roll Attack" }).click();

  const card = result.getByTestId("attack-result");
  await expect(card).toContainText(/Hit|Miss/);
  await expect(card).toContainText("vs AC 4");
  for (const label of [
    "d20",
    "Crit check d100",
    "Multiplier d100",
    "Margin",
    "Base THAC0",
    "Effective THAC0",
    "Hits AC",
    "Modifiers",
  ]) {
    await expect(card).toContainText(label);
  }
  await snap(page, "attack-result");
});

test("a saving throw shows target, roll and margin", async ({ page }) => {
  await openDashboard(page);
  await openWorkspace(page, "Act");
  await page.getByRole("tab", { name: "Saves" }).click();
  await page.getByRole("button", { name: "Roll save vs Spells" }).click();

  const card = page
    .getByRole("region", { name: "Last Result" })
    .getByTestId("save-result");
  await expect(card).toContainText("Save vs Spells");
  await expect(card).toContainText(/Passed|Failed/);
  for (const label of ["d20", "Target", "Margin"]) {
    await expect(card).toContainText(label);
  }
  await snap(page, "save-result");
});

test("spells tab and inventory workspace render their panels", async ({ page }) => {
  await openDashboard(page);
  await openWorkspace(page, "Act");
  await page.getByRole("tab", { name: "Spells" }).click();
  await expect(page.getByRole("tabpanel")).toContainText("Spell Slots");
  await snap(page, "spells-tab");
  await openWorkspace(page, "Inventory");
  await expect(page.getByRole("region", { name: "Inventory" })).toContainText("Wand of lightning");
});
