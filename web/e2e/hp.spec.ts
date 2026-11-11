import { test, expect, snap } from "./fixtures";
import {
  currentHp,
  expectNoReload,
  logEntry,
  markNoReload,
  openDashboard,
  readsAfter,
  recordHp,
  roundGroup,
} from "./helpers";

test("damage then heal change current HP by the amounts entered, without reload", async ({
  page,
}) => {
  await openDashboard(page);
  const before = await currentHp(page);
  await markNoReload(page);

  // The buttons stay disabled until an amount is filled; the source is optional.
  const box = page.getByRole("group", { name: "HP change" });
  const apply = box.getByRole("button", { name: "Damage", exact: true });
  await expect(apply).toBeDisabled();
  await box.getByLabel("Amount").fill("3");
  await expect(apply).toBeEnabled();
  await box.getByLabel("Source").fill("e2e trap");

  // The answer carries the new HP and names "hp,log": only the journal is read again.
  const reads = await readsAfter(page, async () => {
    await recordHp(page, "damage", 4, "e2e goblin club");
    await expect.poll(() => currentHp(page)).toBe(before - 4);
    await expect(logEntry(page, "e2e goblin club")).toContainText("−4");
  });
  test.info().annotations.push({ type: "reads", description: reads.join(" ") });
  expect(reads).toEqual(["/api/log"]);

  await recordHp(page, "heal", 2, "e2e potion");
  await expect.poll(() => currentHp(page)).toBe(before - 2);
  await expect(logEntry(page, "e2e potion")).toContainText("+2");

  // Both land in the current round's group of the combat log.
  const round = (await page.getByTestId("round-value").textContent())!.trim();
  const group = roundGroup(page, round);
  await expect(group.getByRole("listitem")).toHaveCount(2);

  await expectNoReload(page);
  await snap(page, "damage-and-heal");
});

test("Quick HP: text typed while a change's refetch is pending is kept", async ({
  page,
}) => {
  await openDashboard(page);
  const box = page.getByRole("group", { name: "HP change" });
  const before = await currentHp(page);

  // The journal read after the change is held until the test releases it.
  let release!: () => void;
  const gate = new Promise<void>((r) => (release = r));
  let held!: () => void;
  const heldSeen = new Promise<void>((r) => (held = r));
  await page.route("**/api/log*", async (route) => {
    held();
    await gate;
    await route.continue().catch(() => {});
  });

  await box.getByLabel("Amount").fill("2");
  await box.getByLabel("Source").fill("e2e first");
  await box.getByRole("button", { name: "Damage", exact: true }).click();
  await heldSeen;
  await expect.poll(() => currentHp(page)).toBe(before - 2);

  // The next change is typed while the first one is still reading.
  await box.getByLabel("Amount").fill("5");
  await box.getByLabel("Source").fill("e2e second");
  release();
  await page.unroute("**/api/log*");
  await expect(logEntry(page, "e2e first")).toBeVisible();
  await expect(box.getByLabel("Amount")).toHaveValue("5");
  await expect(box.getByLabel("Source")).toHaveValue("e2e second");

  await box.getByRole("button", { name: "Damage", exact: true }).click();
  await expect.poll(() => currentHp(page)).toBe(before - 7);
});
