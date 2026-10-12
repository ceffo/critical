import { test, expect, snap } from "./fixtures";
import {
  currentHp,
  expectNoReload,
  logEntry,
  markNoReload,
  openDashboard,
  recordHp,
} from "./helpers";

test("damage then heal change current HP by the amounts entered, without reload", async ({
  page,
}) => {
  await openDashboard(page);
  const before = await currentHp(page);
  await markNoReload(page);

  await recordHp(page, "damage", 4, "e2e goblin club");
  await expect.poll(() => currentHp(page)).toBe(before - 4);
  await expect(logEntry(page, "e2e goblin club")).toContainText("−4");

  await recordHp(page, "heal", 2, "e2e potion");
  await expect.poll(() => currentHp(page)).toBe(before - 2);
  await expect(logEntry(page, "e2e potion")).toContainText("+2");

  // Both land in the current round's group of the combat log.
  const round = (await page.getByTestId("round-value").textContent())!.trim();
  const group = page.getByRole("region", { name: `Round ${round}` });
  await expect(group.getByRole("listitem")).toHaveCount(2);

  await expectNoReload(page);
  await snap(page, "damage-and-heal");
});

test("HP buttons stay disabled until amount and source are filled", async ({
  page,
}) => {
  await openDashboard(page);
  const box = page.getByRole("group", { name: "HP change" });
  const damage = box.getByRole("button", { name: "Apply Damage" });
  await expect(damage).toBeDisabled();
  await box.getByLabel("Amount").fill("3");
  await expect(damage).toBeDisabled();
  await box.getByLabel("Source").fill("e2e trap");
  await expect(damage).toBeEnabled();
});
