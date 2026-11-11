import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { test, expect, snap } from "./fixtures";
import {
  combatLog,
  expectUndoOn,
  currentHp,
  undoEntry,
  expectNoReload,
  logEntry,
  markNoReload,
  apiCall,
  openDashboard,
  recordHp,
  reloadSettled,
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

test.describe("the default journal view", () => {
  test.use({ journalAll: false });

  // Setup rows are hidden by default (the kind views flag them); the filter brings them back and the
  // choice survives a reload. A mage whose spellbook is filled by hand, one add (one transaction) per
  // spell, opens on a journal without those rows; Undo still names the entry it pops. What the filter
  // keeps and hides per kind is a unit test (bookkeeping.test.ts); this is the wiring and the persistence.
  test("a mage with 17 hand-added spells opens on a journal without them; All brings them back and survives a reload", async ({
    page,
  }) => {
    test.setTimeout(90_000);
    await page.setViewportSize({ width: 1920, height: 1080 });
    await openDashboard(page);
    const catalogue = JSON.parse(
      readFileSync(
        fileURLToPath(
          new URL("../../api/testdata/catalogue.json", import.meta.url),
        ),
        "utf8",
      ),
    ) as { spells: { id: string; class: string }[] };
    const wizard = catalogue.spells
      .filter((s) => s.class === "Wizard" && s.id !== "detect_magic")
      .map((s) => s.id)
      .slice(0, 17);
    expect(wizard).toHaveLength(17);
    expect(
      await apiCall(page, "POST", "/api/characters", {
        name: "Zéphyrin le Collectionneur",
        class: "mage",
        raceCode: "human",
        details: { alignment: "neutral_good", sex: "male" },
        str: 9,
        dex: 12,
        con: 11,
        int: 19,
        wis: 12,
        cha: 10,
        hpMax: 4,
        xp: 0,
      }),
    ).toBe(true);
    expect(
      await apiCall(page, "POST", "/api/xp", { base: 40000, bonus: false }),
    ).toBe(true);
    for (const id of wizard) {
      expect(
        await apiCall(page, "POST", "/api/character/spellbook/add", {
          spellId: id,
        }),
      ).toBe(true);
    }
    await reloadSettled(page);
    await recordHp(page, "damage", 1, "e2e rat bite");

    const log = combatLog(page);
    await expect(logEntry(page, "e2e rat bite")).toBeVisible();
    await expect(log.locator('[data-kind="spell_add"]')).toHaveCount(0);
    await expect(log.getByTestId("journal-hidden")).toHaveText(
      "17 setup entries hidden",
    );
    // The newest entry is the damage; the XP award stays (character progress).
    await expect(log.locator('[data-kind="xp_add"]')).toHaveCount(1);
    await snap(page, "setup-hidden-1920x1080");

    await log.getByRole("radio", { name: "All" }).click();
    await expect(log.locator('[data-kind="spell_add"]')).toHaveCount(17);
    await expect(log.getByTestId("journal-hidden")).toHaveCount(0);
    await snap(page, "setup-shown-1920x1080");

    await reloadSettled(page);
    await expect(log.getByRole("radio", { name: "All" })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    await expect(log.locator('[data-kind="spell_add"]')).toHaveCount(17);

    // Back to the default; Undo pops the newest entry, which is visible here, so pop it and then the
    // hidden spell: its tooltip names the spell although the row is not listed.
    await log.getByRole("radio", { name: "Hide setup" }).click();
    await undoEntry(page, "e2e rat bite");
    await expect(log.locator('[data-kind="xp_add"]')).toHaveCount(1);
    await page.getByTestId("undo").click();
    await expect(page.getByTestId("undo")).toHaveAttribute(
      "aria-label",
      /Added to the spellbook/,
    );
    await snap(page, "undo-names-hidden-spell-1920x1080");
  });
});
