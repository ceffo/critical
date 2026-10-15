import { test, expect, snap } from "./fixtures";
import {
  buffBadge,
  currentHp,
  voidEntry,
  expectNoReload,
  logEntry,
  markNoReload,
  openDashboard,
  recordHp,
} from "./helpers";

// The log is a stack with note-only amends (critical-qki.38). User,
// 2026-10-14: "no amending values. just the note.. can only pop elements".
test("a note on a heal and on a buff entry changes no value and no HP", async ({
  page,
  stack,
}) => {
  const res = await fetch(`${stack.apiUrl}/api/buffs`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name: "e2e Haste", duration: 5, charges: 0, effects: {} }),
  });
  expect(res.ok).toBe(true);
  await openDashboard(page);
  const max = await currentHp(page);
  await markNoReload(page);
  // Wait for each change to land: the HP form resets after a success.
  await recordHp(page, "damage", 6, "e2e orc axe");
  await expect.poll(() => currentHp(page)).toBe(max - 6);
  await recordHp(page, "heal", 4, "e2e cure light");
  await expect.poll(() => currentHp(page)).toBe(max - 2);

  for (const [text, note] of [
    ["e2e cure light", "e2e DM: rolled 4 on the d8"],
    ["Activated e2e Haste", "e2e DM: from the scroll"],
  ]) {
    const entry = logEntry(page, text);
    await entry.getByRole("button", { name: "Note" }).click();
    const form = entry.getByRole("form", { name: "Entry note" });
    await expect(form.getByRole("spinbutton")).toHaveCount(0); // no value field
    await form.getByLabel("Note").fill(note);
    await form.getByRole("button", { name: "Save" }).click();
    await expect(entry.getByTestId("log-note")).toHaveText(note);
    await expect(entry).not.toContainText("Amended");
  }
  await expect(logEntry(page, "e2e cure light").getByTestId("log-delta")).toHaveText("+4");
  await expect.poll(() => currentHp(page)).toBe(max - 2);
  await expect(buffBadge(page, "e2e Haste")).toBeVisible();

  await expectNoReload(page);
  await snap(page, "notes");
});

test("only the newest entry offers Void: damage 10, heal 4, pop the heal, pop the damage", async ({
  page,
}) => {
  await openDashboard(page);
  const max = await currentHp(page);
  await recordHp(page, "damage", 10, "e2e ogre club");
  await expect.poll(() => currentHp(page)).toBe(max - 10);
  await recordHp(page, "heal", 4, "e2e potion");
  await expect.poll(() => currentHp(page)).toBe(max - 6);

  const damage = logEntry(page, "e2e ogre club");
  const heal = logEntry(page, "e2e potion");
  await expect(damage.getByRole("button", { name: "Void" })).toHaveCount(0);
  await expect(heal.getByRole("button", { name: "Void" })).toHaveCount(1);
  await snap(page, "void-only-on-top");

  await voidEntry(page, "e2e potion");
  await expect.poll(() => currentHp(page)).toBe(max - 10);
  await voidEntry(page, "e2e ogre club");
  await expect.poll(() => currentHp(page)).toBe(max);
  await snap(page, "popped-in-order");
});

test("voiding an entry restores current HP and marks it voided", async ({
  page,
}) => {
  await openDashboard(page);
  const before = await currentHp(page);
  await markNoReload(page);

  await recordHp(page, "damage", 5, "e2e wrong target");
  await expect.poll(() => currentHp(page)).toBe(before - 5);

  const entry = logEntry(page, "e2e wrong target");
  await entry.getByRole("button", { name: "Void" }).click();
  await expect(entry).toContainText("Void this entry?");
  await entry.getByRole("button", { name: "Confirm" }).click();

  await expect.poll(() => currentHp(page)).toBe(before);
  await expect(entry).toHaveAttribute("data-status", "voided");
  await expect(entry).toContainText("Voided");
  await expect(entry.getByRole("button", { name: "Void" })).toHaveCount(0);
  await expect(entry.getByRole("button", { name: "Note" })).toHaveCount(0);

  await expectNoReload(page);
  await snap(page, "voided");
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
    (await fetch(`${stack.apiUrl}/api/log/${id}/void`, { method: "POST" })).ok,
  ).toBe(true);

  await entry.getByRole("button", { name: "Void" }).click();
  await entry.getByRole("button", { name: "Confirm" }).click();

  const alert = page.getByRole("alert");
  await expect(alert).toContainText("already voided");
  await snap(page, "server-error");
  await alert.getByRole("button", { name: "Dismiss" }).click();
  await expect(alert).toHaveCount(0);
  // The refresh after the failed call shows the server's truth.
  await expect(entry).toHaveAttribute("data-status", "voided");
});
