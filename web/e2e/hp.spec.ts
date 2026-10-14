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

test("a heal never takes HP above max: the excess is dropped and shown", async ({ page }) => {
  await openDashboard(page);
  const max = await currentHp(page); // a fresh character starts at full HP

  await recordHp(page, "damage", 5, "e2e arrow");
  await expect.poll(() => currentHp(page)).toBe(max - 5);

  await recordHp(page, "heal", 12, "e2e cure serious");
  await expect.poll(() => currentHp(page)).toBe(max);
  const cure = logEntry(page, "e2e cure serious");
  await expect(cure.getByTestId("log-delta")).toHaveText("+5");
  await expect(cure.getByTestId("log-capped")).toHaveText("+12 requested, capped at max HP");

  // At full HP the heal is still logged, as 0.
  await recordHp(page, "heal", 9, "e2e potion at full");
  await expect.poll(() => currentHp(page)).toBe(max);
  await expect(logEntry(page, "e2e potion at full").getByTestId("log-capped")).toHaveText(
    "+9 requested, capped at max HP",
  );

  await snap(page, "heal-capped-at-max");
});
