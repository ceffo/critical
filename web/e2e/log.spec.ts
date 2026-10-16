import { test, expect, snap } from "./fixtures";
import {
  combatLog,
  expectUndoOn,
  currentHp,
  undoEntry,
  expectNoReload,
  logEntry,
  markNoReload,
  openDashboard,
  recordHp,
} from "./helpers";

// The log is a stack: entries are popped, never edited, and the journal offers no note
// (ADR 0022 step 5.2).
test("the journal Undo names only the newest entry: damage 10, heal 4, pop the heal, pop the damage", async ({
  page,
}) => {
  await openDashboard(page);
  const max = await currentHp(page);
  await recordHp(page, "damage", 10, "e2e ogre club");
  await expect.poll(() => currentHp(page)).toBe(max - 10);
  await recordHp(page, "heal", 4, "e2e potion");
  await expect.poll(() => currentHp(page)).toBe(max - 6);

  await expectUndoOn(page, "e2e potion");
  await expectUndoOn(page, "e2e ogre club", true);
  await snap(page, "undo-names-the-top");

  await undoEntry(page, "e2e potion");
  await expect.poll(() => currentHp(page)).toBe(max - 10);
  await undoEntry(page, "e2e ogre club");
  await expect.poll(() => currentHp(page)).toBe(max);
  await snap(page, "popped-in-order");
});

test("undoing an entry restores current HP and marks it undone", async ({
  page,
}) => {
  await openDashboard(page);
  const before = await currentHp(page);
  await markNoReload(page);

  await recordHp(page, "damage", 5, "e2e wrong target");
  await expect.poll(() => currentHp(page)).toBe(before - 5);

  const entry = logEntry(page, "e2e wrong target");
  await page.getByTestId("undo").click();

  await expect.poll(() => currentHp(page)).toBe(before);
  await expect(entry).toHaveAttribute("data-status", "voided");
  await expect(entry).toContainText("Undone");
  await expect(
    entry.getByRole("button", { name: "Undo", exact: true }),
  ).toHaveCount(0);
  await expect(
    combatLog(page).getByRole("button", { name: "Note" }),
  ).toHaveCount(0);

  await expectNoReload(page);
  await snap(page, "undone");
});

test("a server error is shown to the user", async ({ page, stack }) => {
  await openDashboard(page);
  await recordHp(page, "damage", 2, "e2e stale entry");
  const entry = logEntry(page, "e2e stale entry");
  await expect(entry).toBeVisible();

  // Void it behind the UI's back, then try again from the (stale) UI.
  const { entries } = (await (
    await fetch(`${stack.apiUrl}/api/log`)
  ).json()) as {
    entries: { id: number; summary: string }[];
  };
  const id = entries.find((e) => e.summary === "e2e stale entry")!.id;
  expect(
    (
      await fetch(`${stack.apiUrl}/api/log/undo`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ expectedTopId: id }),
      })
    ).ok,
  ).toBe(true);

  await page.getByTestId("undo").click();

  const alert = page.getByRole("alert");
  await expect(alert).toContainText(/nothing to undo|changed in the meantime/);
  await snap(page, "server-error");
  await alert.getByRole("button", { name: "Dismiss" }).click();
  await expect(alert).toHaveCount(0);
  // The refresh after the failed call shows the server's truth.
  await expect(entry).toHaveAttribute("data-status", "voided");
});

// Global undo (ADR 0010 step 5). Only a browser proves the button's wiring, the tooltip worded
// from the payload, the window-level Ctrl/Cmd+Z and its focus guard; the stack rules
// (expected top, every kind) are Go tests.
test("Annuler and Ctrl/Cmd+Z pop the newest action", async ({
  page,
  stack,
}) => {
  const arrows = async () => {
    const res = await fetch(`${stack.apiUrl}/api/items`);
    const items = (await res.json()) as { id: string; count: number }[];
    return items.find((i) => i.id === "arrows_plus3")?.count;
  };
  const before = await arrows();
  const shot = await fetch(`${stack.apiUrl}/api/roll/attack`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      weapon_id: "shortbow",
      ammo_item_id: "arrows_plus3",
      target_ac: 5,
    }),
  });
  expect(shot.ok).toBe(true);
  expect(await arrows()).toBe(before! - 1);

  await openDashboard(page);
  const undo = page.getByTestId("undo");
  await expect(undo).toHaveText("Undo");
  await expect(undo).toHaveAttribute("title", /^Undo: .*Short Bow \+2/);
  for (const vp of [
    { width: 1280, height: 800 },
    { width: 1440, height: 900 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    await snap(page, `undo-${vp.width}x${vp.height}`);
  }
  await page.setViewportSize({ width: 1440, height: 900 });

  // Annuler: the shot is popped and its arrow is back.
  await undo.click();
  await expect(logEntry(page, "Short Bow +2")).toContainText("Undone");
  await expect.poll(arrows).toBe(before);

  // Ctrl/Cmd+Z while typing in a field is the field's own undo: nothing is popped.
  const max = await currentHp(page);
  await recordHp(page, "damage", 5, "e2e undo goblin");
  await expect.poll(() => currentHp(page)).toBe(max - 5);
  await expect(undo).toHaveAttribute("title", /e2e undo goblin/);
  const source = page
    .getByRole("group", { name: "HP change" })
    .getByLabel("Source");
  await source.focus();
  await page.keyboard.press("ControlOrMeta+z");
  await page.keyboard.press("Meta+z");
  // A refresh would have landed by now; the damage is still on top.
  await page.waitForTimeout(300);
  await expect(logEntry(page, "e2e undo goblin")).not.toContainText("Undone");
  expect(await currentHp(page)).toBe(max - 5);

  // Outside a field the shortcut pops the damage.
  await source.blur();
  await page.keyboard.press("ControlOrMeta+z");
  await expect(logEntry(page, "e2e undo goblin")).toContainText("Undone");
  await expect.poll(() => currentHp(page)).toBe(max);
  await snap(page, "undo-popped");
});
