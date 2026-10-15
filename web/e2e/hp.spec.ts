import { test, expect, snap } from "./fixtures";
import {
  addBuff,
  currentHp,
  expectNoReload,
  logEntry,
  markNoReload,
  openDashboard,
  recordHp,
  turnGroup,
} from "./helpers";

test("damage then heal change current HP by the amounts entered, without reload", async ({
  page,
}) => {
  await openDashboard(page);
  const before = await currentHp(page);
  await markNoReload(page);

  // The buttons stay disabled until amount and source are both filled.
  const box = page.getByRole("group", { name: "HP change" });
  const apply = box.getByRole("button", { name: "Damage", exact: true });
  await expect(apply).toBeDisabled();
  await box.getByLabel("Amount").fill("3");
  await expect(apply).toBeDisabled();
  await box.getByLabel("Source").fill("e2e trap");
  await expect(apply).toBeEnabled();

  await recordHp(page, "damage", 4, "e2e goblin club");
  await expect.poll(() => currentHp(page)).toBe(before - 4);
  await expect(logEntry(page, "e2e goblin club")).toContainText("−4");

  await recordHp(page, "heal", 2, "e2e potion");
  await expect.poll(() => currentHp(page)).toBe(before - 2);
  await expect(logEntry(page, "e2e potion")).toContainText("+2");

  // Both land in the current round's group of the combat log.
  const round = (await page.getByTestId("round-value").textContent())!.trim();
  const group = turnGroup(page, round);
  await expect(group.getByRole("listitem")).toHaveCount(2);

  await expectNoReload(page);
  await snap(page, "damage-and-heal");
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

test("Moi column: Enter applies damage, Shift+Enter healing; last change, ghost and last save follow", async ({
  page,
}) => {
  await openDashboard(page);
  const moi = page.getByRole("complementary", { name: "Character" });
  const box = moi.getByRole("group", { name: "HP change" });
  const before = await currentHp(page);
  await addBuff(page, { name: "e2e kbd Stoneskin", charges: 3 });

  // Enter in the source field: damage.
  await box.getByLabel("Amount").fill("6");
  await box.getByLabel("Source").fill("e2e kbd orc");
  await box.getByLabel("Source").press("Enter");
  await expect.poll(() => currentHp(page)).toBe(before - 6);
  await expect(moi.getByTestId("hp-last-change")).toHaveText("−6 e2e kbd orc");
  await expect(moi.getByTestId("hp-ghost")).toBeVisible();
  await expect(box.getByLabel("Amount")).toHaveValue("");

  // Shift+Enter in the amount field: healing.
  await box.getByLabel("Amount").fill("4");
  await box.getByLabel("Source").fill("e2e kbd potion");
  await box.getByLabel("Amount").press("Shift+Enter");
  await expect.poll(() => currentHp(page)).toBe(before - 2);
  await expect(moi.getByTestId("hp-last-change")).toHaveText("+4 e2e kbd potion");
  await expect(moi.getByTestId("hp-ghost")).toHaveCount(0);

  // A save rolled from its row shows its last roll there and its card in the Fil.
  const spells = moi.getByRole("button", { name: "Roll save vs Spells" });
  await spells.click();
  await expect(page.getByRole("region", { name: "Detail", exact: true }).getByTestId("save-result")).toBeVisible();
  await expect(spells.getByTestId("save-last")).toHaveText(/^\d+$/);

  for (const vp of [
    { width: 1280, height: 800 },
    { width: 1440, height: 900 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    await expect(moi.getByRole("group", { name: "e2e kbd Stoneskin" })).toBeVisible();
    await snap(page, `moi-${vp.width}x${vp.height}`);
  }
});

test("below 0 HP: the condition and the dying zone show, and HP stops at the floor", async ({
  page,
}) => {
  await openDashboard(page);
  const moi = page.getByRole("complementary", { name: "Character" });
  const max = await currentHp(page); // a fresh character starts at full HP
  await expect(moi.getByTestId("hp-condition")).toHaveCount(0);
  await expect(moi.getByTestId("hp-dying-fill")).toHaveCount(0);

  // Down to 4 below 0: unconscious, the dying zone fills from 0.
  await recordHp(page, "damage", max + 4, "e2e dragon breath");
  await expect.poll(() => currentHp(page)).toBe(-4);
  await expect(moi.getByTestId("hp-condition")).toHaveText("Unconscious");
  await expect(moi.getByTestId("hp-dying-fill")).toBeVisible();
  await expect(moi.getByTestId("hp-scale")).toContainText("0");
  for (const vp of [
    { width: 1280, height: 800 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    await snap(page, `hp-unconscious-${vp.width}x${vp.height}`);
  }

  // Further damage stops at the floor; the log shows what was asked.
  await recordHp(page, "damage", 20, "e2e ogre club");
  await expect.poll(() => currentHp(page)).toBe(-10);
  await expect(moi.getByTestId("hp-condition")).toHaveText("Dead");
  const club = logEntry(page, "e2e ogre club");
  await expect(club.getByTestId("log-delta")).toHaveText("−6");
  await expect(club.getByTestId("log-capped")).toHaveText("−20 requested, stopped at the HP floor");

  for (const vp of [
    { width: 1280, height: 800 },
    { width: 1440, height: 900 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    await expect(moi.getByTestId("hp-dying-zone")).toBeVisible();
    await snap(page, `hp-floor-${vp.width}x${vp.height}`);
  }
});
