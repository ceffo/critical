import { test, expect, snap } from "./fixtures";
import {
  addBuff,
  buffBadge,
  currentHp,
  effectiveStr,
  logEntry,
  openAddEffect,
  openDashboard,
  receivePicker,
  recordHp,
  voidEntry,
  nextTurn,
  turnMenu,
  voidInitiative,
} from "./helpers";
import type { Page } from "@playwright/test";

// Dice are random on the real stack: assert what the cast did, not the rolled numbers
// (castsvc's Go tests pin the rolls).

async function prepare(apiUrl: string, spells: string[]) {
  const res = await fetch(`${apiUrl}/api/spells/prepared`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ spells }),
  });
  expect(res.ok).toBe(true);
}

async function openSpells(page: Page) {
  await openDashboard(page);
  await page.getByRole("tab", { name: "Spells" }).click();
  await expect(
    page.getByRole("button", { name: "Prepare spells" }),
  ).toBeVisible();
}

function resultCard(page: Page) {
  return page.getByRole("region", { name: "Detail", exact: true });
}

test("casting Stoneskin shows the result card, adds the buff; voiding gives the slot back", async ({
  page,
  stack,
}) => {
  await prepare(stack.apiUrl, ["stoneskin"]);
  await openSpells(page);
  const level4 = page.getByRole("region", { name: "Level 4" });
  await expect(level4.getByTestId("remaining-4")).toHaveText("1 / 1");

  await level4.getByRole("button", { name: "Cast Stoneskin" }).click();
  const card = resultCard(page).getByTestId("cast-result");
  await expect(card).toContainText(
    "Stoneskin (level 4), cast at caster level 15",
  );
  await expect(card.getByTestId("cast-buff")).toContainText("On you");
  await expect(card.getByTestId("cast-charges")).toContainText(/1d4\+\d+/);
  await expect(card.getByTestId("cast-text")).toBeHidden(); // folded by default
  await card.getByText("Spell text").click();
  await expect(card.getByTestId("cast-text")).toBeVisible();
  await expect(buffBadge(page, "Stoneskin")).toBeVisible();
  await expect(level4.getByTestId("remaining-4")).toHaveText("0 / 1");
  await expect(logEntry(page, "Cast Stoneskin")).toBeVisible();
  await snap(page, "stoneskin-cast");

  await voidEntry(page, "Cast Stoneskin");
  await expect(buffBadge(page, "Stoneskin")).toHaveCount(0);
  await expect(level4.getByTestId("remaining-4")).toHaveText("1 / 1");
  await snap(page, "stoneskin-voided");
});

test("casting Fireball rolls 10d6 and notes the save for the DM", async ({
  page,
  stack,
}) => {
  await prepare(stack.apiUrl, ["fireball"]);
  await openSpells(page);
  await page.getByRole("button", { name: "Cast Fireball" }).click();

  const damage = resultCard(page).getByTestId("cast-damage");
  await expect(damage).toContainText("1 × 10d6");
  const total = Number(
    await damage.getByTestId("cast-damage-total").textContent(),
  );
  expect(total).toBeGreaterThanOrEqual(10);
  expect(total).toBeLessThanOrEqual(60);
  await expect(resultCard(page).getByTestId("cast-save")).toContainText(
    "spell, half damage",
  );
  await expect(
    logEntry(page, `Cast Fireball: ${total} fire damage`),
  ).toBeVisible();
  await snap(page, "fireball");
});

test("a low-confidence spell shows its text only", async ({ page, stack }) => {
  await prepare(stack.apiUrl, ["cloudkill"]);
  await openSpells(page);
  await page.getByRole("button", { name: "Cast Cloudkill" }).click();
  await expect(resultCard(page).getByTestId("cast-text-only")).toContainText(
    "add a buff by hand",
  );
  await expect(page.locator('[data-testid="buff"]')).toHaveCount(0);
  await snap(page, "low-confidence");
});

test("Wall of Ice cast as the falling ice sheet rolls its 3d10 and names the option", async ({
  page,
  stack,
}) => {
  await prepare(stack.apiUrl, ["wall_of_ice"]);
  await openSpells(page);
  const level4 = page.getByRole("region", { name: "Level 4" });
  const option = level4.getByLabel("Cast-time option");
  await expect(option).toHaveValue("ice_plane"); // the first is the default
  await option.selectOption("ice_sheet");
  await expect(level4.getByTestId("slot-dice")).toHaveText("3d10 cold");
  await snap(page, "wall-of-ice-option");
  await level4.getByRole("button", { name: "Cast Wall of Ice" }).click();

  const card = resultCard(page);
  await expect(card.getByTestId("cast-mode")).toHaveText("Option: Ice sheet");
  const damage = card.getByTestId("cast-damage");
  await expect(damage).toContainText("1 × 3d10");
  const total = Number(
    await damage.getByTestId("cast-damage-total").textContent(),
  );
  expect(total).toBeGreaterThanOrEqual(3);
  expect(total).toBeLessThanOrEqual(30);
  await expect(
    logEntry(page, `Cast Wall of Ice: Ice sheet, ${total} cold damage`),
  ).toBeVisible();
  await snap(page, "wall-of-ice-sheet");

  // Received from a party wizard, Ice Storm as sleet: no damage, no text-only banner.
  const picker = await receivePicker(page);
  await picker.getByLabel("Search for a spell cast on me").fill("Ice Storm");
  await picker.getByLabel("Caster level").fill("9");
  await picker.getByLabel("Option of Ice Storm").selectOption("sleet");
  await picker.getByRole("button", { name: "Apply Ice Storm" }).click();
  await expect(page.getByTestId("add-effect-popover")).toBeHidden();
  await expect(card.getByTestId("cast-mode")).toHaveText("Option: Sleet");
  await expect(card.getByTestId("cast-damage")).toHaveCount(0);
  await expect(card.getByTestId("cast-text-only")).toHaveCount(0);
  await snap(page, "ice-storm-sleet-received");

  // Dice steps (ADR 0008 step 15): Incendiary Cloud from a level 12 caster rolls each turn on its
  // own, the two smoke turns without dice.
  const receive = async (name: string, level: string) => {
    const p = await receivePicker(page);
    await p.getByLabel("Search for a spell cast on me").fill(name);
    await p.getByLabel("Caster level").fill(level);
    await p.getByRole("button", { name: `Apply ${name}` }).click();
    await expect(page.getByTestId("add-effect-popover")).toBeHidden();
  };
  await receive("Incendiary Cloud", "12");
  const cloud = card.getByTestId("cast-damage");
  await expect(cloud).toContainText("5 turns, one roll each");
  const turns = cloud.getByTestId("cast-damage-roll");
  await expect(turns).toHaveCount(5);
  await expect(turns.nth(0)).toHaveText("Turn 1no damage");
  await expect(turns.nth(1)).toHaveText("Turn 2no damage");
  await expect(turns.nth(2)).toContainText("Turn 312d2");
  await expect(turns.nth(3)).toContainText("Turn 412d4");
  await expect(turns.nth(4)).toContainText("Turn 512d2");
  await snap(page, "incendiary-cloud-received");

  // Chain Lightning from a level 15 caster: twelve strikes, 12d6 down to 1d6, at 1280 wide.
  await page.setViewportSize({ width: 1280, height: 900 });
  await receive("Chain Lightning", "15");
  const strikes = card.getByTestId("cast-damage").getByTestId("cast-damage-roll");
  await expect(strikes).toHaveCount(12);
  await expect(strikes.nth(0)).toContainText("#112d6");
  await expect(strikes.nth(11)).toContainText("#121d6");
  await snap(page, "chain-lightning-received-1280");
  await page.setViewportSize({ width: 1440, height: 900 });

  // The journal card of the earlier Incendiary Cloud shows the same turns.
  await logEntry(page, "Incendiary Cloud").getByTestId("log-select").click();
  const entry = card.getByTestId("cast-entry");
  await expect(entry.getByTestId("cast-entry-instances").getByRole("listitem").filter({ hasText: /^Turn/ })).toHaveCount(5);
  await expect(entry).toContainText("Turn 1no damage");
  await expect(entry).toContainText("Turn 5");
  await snap(page, "incendiary-cloud-journal");
});

test("Strength cast on me by a party wizard, found by its French name; a saved effect joins it", async ({
  page,
}) => {
  await openDashboard(page);
  const before = await effectiveStr(page);
  const picker = await receivePicker(page);
  await picker.getByLabel("Search for a spell cast on me").fill("Force");
  await picker.getByLabel("Caster level").fill("9");
  await picker.getByRole("button", { name: "Apply Strength" }).click();

  const card = resultCard(page).getByTestId("cast-result");
  await expect(card).toContainText("Strength cast on you, caster level 9");
  await expect(card.getByTestId("cast-rounds")).toContainText("= 540");
  await expect(card.getByTestId("cast-str-gain")).toContainText("1d6");
  // One entry point: the popover closes, the buff is on the timeline from the current turn.
  await expect(page.getByTestId("add-effect-popover")).toBeHidden();
  await expect(buffBadge(page, "Strength")).toBeVisible();
  await expect(
    buffBadge(page, "Strength").getByTestId("buff-rounds"),
  ).toContainText("540 turns");
  await expect.poll(() => effectiveStr(page)).toBeGreaterThan(before);
  await expect(
    logEntry(page, "Strength cast on me (caster level 9)"),
  ).toBeVisible();

  // A saved effect from the same popover.
  await addBuff(page, { name: "e2e Ward", rounds: 5, save: true });
  const saved = await openAddEffect(page, "Saved effect");
  await saved
    .getByRole("button", { name: "Add saved effect e2e Ward" })
    .click();
  await expect(buffBadge(page, "e2e Ward")).toHaveCount(2);

  for (const vp of [
    { width: 1280, height: 800 },
    { width: 1440, height: 900 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    await openAddEffect(page, "Spell cast on me");
    await snap(page, `add-effect-${vp.width}x${vp.height}`);
    await page.keyboard.press("Escape");
  }
  await page.setViewportSize({ width: 1440, height: 900 });
  await snap(page, "strength-received");
});

test("after Next turn the cast offers no void until Next turn is popped", async ({
  page,
  stack,
}) => {
  await prepare(stack.apiUrl, ["stoneskin"]);
  await openSpells(page);
  await page.getByRole("button", { name: "Cast Stoneskin" }).click();
  await expect(buffBadge(page, "Stoneskin")).toBeVisible();
  await nextTurn(page);
  await expect(logEntry(page, "Turn 0 → 1")).toBeVisible();

  const entry = logEntry(page, "Cast Stoneskin");
  await expect(entry.getByRole("button", { name: "Void" })).toHaveCount(0);
  await snap(page, "void-not-offered-after-next-round");

  await voidInitiative(page);
  await voidEntry(page, "Turn 0 → 1");
  await expect(buffBadge(page, "Stoneskin")).toBeVisible();
  await voidEntry(page, "Cast Stoneskin");
  await expect(buffBadge(page, "Stoneskin")).toHaveCount(0);
});

test("a long rest is logged; popping it, then the cast, walks the slots back", async ({
  page,
  stack,
}) => {
  await prepare(stack.apiUrl, ["haste"]);
  await openSpells(page);
  const level3 = page.getByRole("region", { name: "Level 3" });
  await page.getByRole("button", { name: "Cast Haste" }).click();
  await expect(level3.getByTestId("remaining-3")).toHaveText("0 / 1");
  await page.getByRole("button", { name: "Long Rest (Reset)" }).click();
  await expect(level3.getByTestId("remaining-3")).toHaveText("1 / 1");
  await expect(logEntry(page, "Long rest: 1 spell slots")).toBeVisible();

  await expect(
    logEntry(page, "Cast Haste").getByRole("button", { name: "Void" }),
  ).toHaveCount(0);
  await snap(page, "void-not-offered-after-long-rest");
  await voidEntry(page, "Long rest: 1 spell slots");
  await expect(level3.getByTestId("remaining-3")).toHaveText("0 / 1");
  await voidEntry(page, "Cast Haste");
  await expect(level3.getByTestId("remaining-3")).toHaveText("1 / 1");
});

// Spells that share a name: the receive request carries each spell's own key.
const priest = (name: string, level: number, sphere: string) => ({
  name,
  class: "Priest",
  level,
  school: "Alteration",
  sphere,
  casting_time: "6",
  range: "0",
  duration: "Special",
  area_of_effect: "Creature touched",
  components: "V, S, M",
  saving_throw: "None",
  description: `${name} fixture text.`,
});

test.describe("same-named spells", () => {
  test.use({
    extraSpells: {
      rows: [
        priest("Fortify", 3, "War"),
        priest("Fortify", 4, "Healing"),
        ...Array.from({ length: 60 }, (_, i) =>
          priest(`Cure Test ${String(i).padStart(2, "0")}`, 1, "Healing"),
        ),
      ],
    },
  });

  async function receive(page: Page, query: string, row: string) {
    await openDashboard(page);
    const picker = await receivePicker(page);
    await picker.getByLabel("Search for a spell cast on me").fill(query);
    await picker.getByLabel("Caster level").fill("9");
    const item = picker.getByRole("listitem").filter({ hasText: row });
    const req = page.waitForRequest(
      (r) => r.url().endsWith("/api/spells/receive") && r.method() === "POST",
    );
    await item.getByRole("button", { name: /^Apply/ }).click();
    return (await req).postDataJSON() as { spellId: string };
  }

  test("receiving Fortify level 4 sends fortify_l4 and the result names the level 4 spell", async ({
    page,
  }) => {
    const body = await receive(page, "Fortify", "Priest 4");
    expect(body.spellId).toBe("fortify_l4");
    const card = resultCard(page).getByTestId("cast-result");
    await expect(card.getByTestId("cast-spell-ident")).toHaveText(
      "Priest · Lvl 4",
    );
    await expect(card.getByTestId("cast-text")).toContainText(
      "Fortify fixture text.",
    );
    await snap(page, "fortify-l4-received");
  });

  test("receive search shows how many matches are not listed", async ({
    page,
  }) => {
    await openDashboard(page);
    const picker = await receivePicker(page);
    await picker.getByLabel("Search for a spell cast on me").fill("Cure Test");
    await expect(picker.getByRole("listitem")).toHaveCount(50);
    await expect(picker.getByTestId("receive-more")).toHaveText(
      "10 more, refine your search",
    );
    await snap(page, "receive-more");
  });
});

test("a cast buff expired by Time passes: pop the fast-forward, then the cast", async ({
  page,
  stack,
}) => {
  await prepare(stack.apiUrl, ["haste"]);
  await openSpells(page);
  await page.getByRole("button", { name: "Cast Haste" }).click();
  await expect(buffBadge(page, "Haste")).toBeVisible();

  const form = page.getByRole("form", { name: "Time passes…" });
  await turnMenu(page, "Time passes…");
  await form.getByLabel("Hours that pass").fill("1");
  await form.getByRole("button", { name: "Confirm" }).click();
  await expect(page.getByRole("status")).toContainText("Expired buffs: Haste");
  await expect(buffBadge(page, "Haste")).toHaveCount(0);

  const entry = logEntry(page, "Cast Haste");
  await expect(entry.getByRole("button", { name: "Void" })).toHaveCount(0);
  await snap(page, "void-cast-not-offered-under-pass-time");
  await voidEntry(page, "Time passed: 1 h (60 rd)");
  await expect(buffBadge(page, "Haste")).toBeVisible();
  await voidEntry(page, "Cast Haste");
  await expect(buffBadge(page, "Haste")).toHaveCount(0);
});

// The log is a stack: cast, HP change, cast. Only the last
// cast offers Void and the HP panel's undo is off (with a hint) while it is on
// top; popping it, then the heal, walks HP and slots back one step each.
test("cast, heal, cast: pop the last cast, then the heal; HP undo is off under a cast", async ({
  page,
  stack,
}) => {
  await prepare(stack.apiUrl, ["haste", "stoneskin"]);
  await openSpells(page);
  const max = await currentHp(page);
  const level3 = page.getByRole("region", { name: "Level 3" });
  const level4 = page.getByRole("region", { name: "Level 4" });
  const undo = page.getByRole("button", { name: "Undo last HP change" });
  const hint = page.getByTestId("hp-undo-hint");

  await recordHp(page, "damage", 9, "e2e troll");
  await expect.poll(() => currentHp(page)).toBe(max - 9);
  await page.getByRole("button", { name: "Cast Haste" }).click();
  await expect(level3.getByTestId("remaining-3")).toHaveText("0 / 1");
  await expect(undo).toBeDisabled();
  await recordHp(page, "heal", 4, "e2e cure light");
  await expect.poll(() => currentHp(page)).toBe(max - 5);
  await expect(undo).toBeEnabled();
  await expect(hint).toHaveCount(0);
  await page.getByRole("button", { name: "Cast Stoneskin" }).click();
  await expect(level4.getByTestId("remaining-4")).toHaveText("0 / 1");

  // Only the newest entry (the Stoneskin cast) offers Void.
  for (const text of ["Cast Haste", "e2e cure light", "e2e troll"]) {
    await expect(
      logEntry(page, text).getByRole("button", { name: "Void" }),
    ).toHaveCount(0);
  }
  await expect(
    logEntry(page, "Cast Stoneskin").getByRole("button", { name: "Void" }),
  ).toHaveCount(1);
  await expect(undo).toBeDisabled();
  await expect(hint).toContainText("Latest entry: “Cast Stoneskin");
  await expect(hint).toContainText("Undo it from the log first.");
  await snap(page, "hp-undo-off-under-a-cast");

  await voidEntry(page, "Cast Stoneskin");
  await expect(level4.getByTestId("remaining-4")).toHaveText("1 / 1");
  await expect(level3.getByTestId("remaining-3")).toHaveText("0 / 1");
  await expect.poll(() => currentHp(page)).toBe(max - 5);
  await expect(undo).toBeEnabled();

  await voidEntry(page, "e2e cure light");
  await expect.poll(() => currentHp(page)).toBe(max - 9);
  await expect(level3.getByTestId("remaining-3")).toHaveText("0 / 1");
  await expect(
    logEntry(page, "Cast Haste").getByRole("button", { name: "Void" }),
  ).toHaveCount(1);
  await expect(undo).toBeDisabled();
  await snap(page, "popped-cast-then-heal");
});
