import { test, expect, snap } from "./fixtures";
import {
  currentHp,
  expectNoReload,
  logEntry,
  markNoReload,
  openDashboard,
  recordHp,
} from "./helpers";

test("amending a damage entry corrects current HP and shows the original", async ({
  page,
}) => {
  await openDashboard(page);
  const before = await currentHp(page);
  await markNoReload(page);

  await recordHp(page, "damage", 3, "e2e orc axe");
  await expect.poll(() => currentHp(page)).toBe(before - 3);

  const entry = logEntry(page, "e2e orc axe");
  await entry.getByRole("button", { name: "Amend" }).click();
  const form = entry.getByRole("form", { name: "Amend entry" });
  await form.getByLabel("HP change (− damage, + heal)").fill("-7");
  await form.getByLabel("Source").fill("e2e orc axe (crit)");
  await form.getByRole("button", { name: "Save" }).click();

  await expect.poll(() => currentHp(page)).toBe(before - 7);
  const amended = logEntry(page, "e2e orc axe (crit)");
  await expect(amended).toContainText("Amended");
  await expect(amended).toContainText("was −3");
  await expect(amended.getByTestId("log-delta")).toHaveText("−7");

  await expectNoReload(page);
  await snap(page, "amended");
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
  await expect(entry.getByRole("button", { name: "Amend" })).toHaveCount(0);

  await expectNoReload(page);
  await snap(page, "voided");
});

test("a server error is shown to the user", async ({ page, stack }) => {
  await openDashboard(page);
  await recordHp(page, "damage", 2, "e2e stale entry");
  const entry = logEntry(page, "e2e stale entry");
  await expect(entry).toBeVisible();

  // Void it behind the UI's back, then try again from the (stale) UI.
  const { entries } = (await (await fetch(`${stack.apiUrl}/api/log`)).json()) as {
    entries: { id: number; summary: string }[];
  };
  const id = entries.find((e) => e.summary === "e2e stale entry")!.id;
  expect((await fetch(`${stack.apiUrl}/api/log/${id}/void`, { method: "POST" })).ok).toBe(true);

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
