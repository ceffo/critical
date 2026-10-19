import { test, expect, snap, catalogueSpell } from "./fixtures";
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
  undoEntry,
  expectUndoOn,
  nextRound,
  roundMenu,
  undoNextRound,
  expectSlotDots,
  readsAfter,
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

test("casting Stoneskin shows the result card, adds the buff; undoing gives the slot back", async ({
  page,
  stack,
}) => {
  await prepare(stack.apiUrl, ["stoneskin"]);
  await openSpells(page);
  const level4 = page.getByRole("region", { name: "Level 4" });
  await expectSlotDots(level4, 4, 1, 1);

  const card = resultCard(page).getByTestId("cast-result");
  const reads = await readsAfter(page, async () => {
    await level4.getByRole("button", { name: "Cast Stoneskin" }).click();
    await expect(card).toContainText(
      "Stoneskin (level 4), cast at caster level 15",
    );
  });
  test.info().annotations.push({ type: "reads", description: reads.join(" ") });
  // The answer carries the slots and names "buffs,log,spell_slots": the journal, the effects and
  // what follows them (STR, AC) are read again, nothing else.
  expect(reads).toEqual([
    "/api/ac",
    "/api/buffs",
    "/api/log",
    "/api/stats/str",
  ]);
  await expect(card.getByTestId("cast-buff")).toContainText("On you");
  await expect(card.getByTestId("cast-charges")).toContainText(/1d4\+\d+/);
  await expect(card.getByTestId("cast-text")).toBeHidden(); // folded by default
  await card.getByText("Spell text").click();
  await expect(card.getByTestId("cast-text")).toBeVisible();
  await expect(buffBadge(page, "Stoneskin")).toBeVisible();
  await expectSlotDots(level4, 4, 0, 1);
  await expect(logEntry(page, "Cast Stoneskin")).toBeVisible();
  await snap(page, "stoneskin-cast");

  await undoEntry(page, "Cast Stoneskin");
  await expect(buffBadge(page, "Stoneskin")).toHaveCount(0);
  await expectSlotDots(level4, 4, 1, 1);
  await snap(page, "stoneskin-undone");
});

// ADR 0027 step 10: only the levels with a slot; Rary's Mnemonic Enhancer adds dots; the split
// button's arrow, only on a spell whose target the server lets the player pick, offers the one
// other target and casts on an ally; a spell's name opens its details, Escape closes them.
test("Spells tab: Rary's adds slot dots, the arrow casts on an ally, a name opens the details", async ({
  page,
  stack,
}) => {
  await prepare(stack.apiUrl, ["rarys_mnemonic_enhancer", "strength"]);
  await openSpells(page);
  const level1 = page.getByRole("region", { name: "Level 1" });
  const level2 = page.getByRole("region", { name: "Level 2" });
  const level4 = page.getByRole("region", { name: "Level 4" });
  await expect(level1).toHaveCount(0); // nothing prepared at level 1
  await expectSlotDots(level4, 4, 1, 1);

  // Rary's adds no buff: no target to pick, no arrow.
  await expect(
    level4.getByRole("button", {
      name: "Choose the target of Rary's Mnemonic Enhancer",
    }),
  ).toHaveCount(0);
  // The main part: the server's default (all level 1 slots).
  await level4
    .getByRole("button", { name: "Cast Rary's Mnemonic Enhancer" })
    .click();
  await expectSlotDots(level4, 4, 0, 1);
  await expectSlotDots(level1, 1, 3, 3);

  // The arrow: Strength on an ally, nothing applied to the Bard.
  await level2
    .getByRole("button", { name: "Choose the target of Strength" })
    .click();
  const onAlly = page.getByRole("menuitem", { name: "On an ally" });
  await expect(onAlly).toBeVisible();
  // Strength lands on the Bard by default: the arrow offers only the other target.
  await expect(page.getByRole("menuitem")).toHaveCount(1);
  await snap(page, "cast-target-menu");
  await onAlly.click();
  await expect(resultCard(page).getByTestId("cast-text-only")).toContainText(
    "Cast on someone else",
  );
  await expect(buffBadge(page, "Strength")).toHaveCount(0);
  await expectSlotDots(level2, 2, 0, 1);
  await snap(page, "spells-tab-after-casts");

  await level2.getByRole("button", { name: "Strength", exact: true }).click();
  const details = page.getByRole("dialog", { name: "Details of Strength" });
  await expect(details.getByTestId("spell-name")).toHaveText("Strength");
  await expect(details.getByTestId("spell-cast-box")).toContainText(
    "At level 15",
  );
  await snap(page, "spell-details");
  await page.keyboard.press("Escape");
  await expect(details).toHaveCount(0);
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

test("a low-confidence spell shows its text only; Cloudkill rolls an exposure on me from its card", async ({
  page,
  stack,
}) => {
  await prepare(stack.apiUrl, [
    "tashas_uncontrollable_hideous_laughter",
    "cloudkill",
  ]);
  await openSpells(page);
  await page
    .getByRole("button", {
      name: "Cast Tasha's Uncontrollable Hideous Laughter",
    })
    .click();
  await expect(resultCard(page).getByTestId("cast-text-only")).toContainText(
    "add an effect by hand",
  );
  await expect(page.locator('[data-testid="buff"]')).toHaveCount(0);
  await snap(page, "low-confidence");

  // ADR 0008 step 16: the cast card serves the exposure dice; one exposure on me is one journal
  // entry and comes off the HP.
  const hpBefore = await currentHp(page);
  await page.getByRole("button", { name: "Cast Cloudkill" }).click();
  const card = resultCard(page);
  const block = card.getByTestId("cast-exposure");
  await expect(block.getByTestId("exposure-dice")).toHaveText(
    "1d10 poison per round of exposure",
  );
  await expect(card.getByTestId("cast-text-only")).toHaveCount(0);
  await block.getByLabel("I am exposed").check();
  await snap(page, "cloudkill-exposure-controls");
  await block.getByRole("button", { name: "Roll exposure" }).click();

  const result = card.getByTestId("exposure-result");
  await expect(result).toContainText(
    "Cloudkill exposure, round 1 (caster level 15)",
  );
  await expect(result.getByTestId("exposure-target")).toHaveText(
    /^Taken off your HP: −\d+$/,
  );
  const total = Number(
    await result.getByTestId("exposure-total").textContent(),
  );
  expect(total).toBeGreaterThanOrEqual(1);
  expect(total).toBeLessThanOrEqual(10);
  await expect.poll(() => currentHp(page)).toBe(hpBefore - total);
  await expect(
    logEntry(page, `Cloudkill exposure, round 1: ${total} poison damage to me`),
  ).toBeVisible();
  // The next round rolls from the exposure card.
  await expect(result.getByLabel("Round of exposure")).toHaveValue("2");
  await snap(page, "cloudkill-exposure");
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
  await picker.getByLabel("Search for a received spell").fill("Ice Storm");
  await picker.getByLabel("Caster level").fill("9");
  await picker.getByLabel("Option of Ice Storm").selectOption("sleet");
  await picker.getByRole("button", { name: "Apply Ice Storm" }).click();
  await expect(page.getByTestId("add-effect-popover")).toBeHidden();
  await expect(card.getByTestId("cast-mode")).toHaveText("Option: Sleet");
  await expect(card.getByTestId("cast-damage")).toHaveCount(0);
  await expect(card.getByTestId("cast-text-only")).toHaveCount(0);
  await snap(page, "ice-storm-sleet-received");

  // Dice steps (ADR 0008 step 15): Incendiary Cloud from a level 12 caster rolls each turn on its
  // own, the two smoke rounds without dice.
  const receive = async (name: string, level: string) => {
    const p = await receivePicker(page);
    await p.getByLabel("Search for a received spell").fill(name);
    await p.getByLabel("Caster level").fill(level);
    await p.getByRole("button", { name: `Apply ${name}` }).click();
    await expect(page.getByTestId("add-effect-popover")).toBeHidden();
  };
  await receive("Incendiary Cloud", "12");
  const cloud = card.getByTestId("cast-damage");
  await expect(cloud).toContainText("5 rounds, one roll each");
  const rounds = cloud.getByTestId("cast-damage-roll");
  await expect(rounds).toHaveCount(5);
  await expect(rounds.nth(0)).toHaveText("Round 1no damage");
  await expect(rounds.nth(1)).toHaveText("Round 2no damage");
  await expect(rounds.nth(2)).toContainText("Round 312d2");
  await expect(rounds.nth(3)).toContainText("Round 412d4");
  await expect(rounds.nth(4)).toContainText("Round 512d2");
  await snap(page, "incendiary-cloud-received");

  // Chain Lightning from a level 15 caster: twelve strikes, 12d6 down to 1d6, at 1280 wide.
  await page.setViewportSize({ width: 1280, height: 900 });
  await receive("Chain Lightning", "15");
  const strikes = card
    .getByTestId("cast-damage")
    .getByTestId("cast-damage-roll");
  await expect(strikes).toHaveCount(12);
  await expect(strikes.nth(0)).toContainText("#112d6");
  await expect(strikes.nth(11)).toContainText("#121d6");
  await snap(page, "chain-lightning-received-1280");
  await page.setViewportSize({ width: 1440, height: 900 });

  // The journal card of the earlier Incendiary Cloud shows the same rounds.
  await logEntry(page, "Incendiary Cloud").getByTestId("log-select").click();
  const entry = card.getByTestId("cast-entry");
  await expect(
    entry
      .getByTestId("cast-entry-instances")
      .getByRole("listitem")
      .filter({ hasText: /^Round/ }),
  ).toHaveCount(5);
  await expect(entry).toContainText("Round 1no damage");
  await expect(entry).toContainText("Round 5");
  await snap(page, "incendiary-cloud-journal");

  // Exposure modes (ADR 0008 step 16): Wall of Fire from a level 11 caster, passing through rolls
  // 2d6+11; the card and the journal row name the option.
  await receive("Wall of Fire", "11");
  const block = card.getByTestId("cast-exposure");
  await block.getByLabel("Where").selectOption("passing_through");
  await snap(page, "wall-of-fire-exposure-option");
  await block.getByRole("button", { name: "Roll exposure" }).click();
  const fire = card.getByTestId("exposure-result");
  await expect(fire.getByTestId("exposure-mode")).toHaveText(
    "Option: Passing through",
  );
  await expect(fire.getByTestId("exposure-roll")).toContainText("2d6+11");
  await expect(fire.getByTestId("exposure-target")).toHaveText(
    "For the DM: your HP is unchanged",
  );
  const fireTotal = Number(
    await fire.getByTestId("exposure-total").textContent(),
  );
  await expect(
    logEntry(
      page,
      `Wall of Fire exposure, round 1: ${fireTotal} fire damage · Passing through`,
    ),
  ).toBeVisible();
  await snap(page, "wall-of-fire-exposure");
});

test("Strength cast on me by a party wizard, found by its French name; a saved effect joins it", async ({
  page,
}) => {
  await openDashboard(page);
  const before = await effectiveStr(page);
  const picker = await receivePicker(page);
  await picker.getByLabel("Search for a received spell").fill("Force");
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
  ).toContainText("540 rounds");
  await expect.poll(() => effectiveStr(page)).toBeGreaterThan(before);
  await expect(
    logEntry(page, "Strength received (caster level 9)"),
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
    await openAddEffect(page, "Received spell");
    await snap(page, `add-effect-${vp.width}x${vp.height}`);
    await page.keyboard.press("Escape");
  }
  await page.setViewportSize({ width: 1440, height: 900 });
  await snap(page, "strength-received");

  // ADR 0028 step 2: the STR tile rolls against the score in effect, the sheet's beside it.
  const str = await effectiveStr(page);
  await page.getByRole("button", { name: `Strength check (${str})` }).click();
  await expect(resultCard(page).getByTestId("ability-result")).toContainText(
    new RegExp(`under \\S+ ${str} \\(sheet ${before}\\)`),
  );
});

test("after Next round the journal Undo is not on the cast until Next round is popped", async ({
  page,
  stack,
}) => {
  await prepare(stack.apiUrl, ["stoneskin"]);
  await openSpells(page);
  await page.getByRole("button", { name: "Cast Stoneskin" }).click();
  await expect(buffBadge(page, "Stoneskin")).toBeVisible();
  await nextRound(page);
  await expect(logEntry(page, "Round 0 → 1")).toBeVisible();

  await expectUndoOn(page, "Cast Stoneskin", true);
  await snap(page, "undo-not-on-the-cast-after-next-round");

  await undoNextRound(page, logEntry(page, "Round 0 → 1"));
  await expect(buffBadge(page, "Stoneskin")).toBeVisible();
  await undoEntry(page, "Cast Stoneskin");
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
  await expectSlotDots(level3, 3, 0, 1);
  await page.getByRole("button", { name: "Long rest" }).click();
  await expectSlotDots(level3, 3, 1, 1);
  await expect(logEntry(page, "Long rest: 1 spell slots")).toBeVisible();

  await expectUndoOn(page, "Cast Haste", true);
  await snap(page, "undo-not-on-the-cast-after-long-rest");
  await undoEntry(page, "Long rest: 1 spell slots");
  await expectSlotDots(level3, 3, 0, 1);
  await undoEntry(page, "Cast Haste");
  await expectSlotDots(level3, 3, 1, 1);
});

// Spells that share a name: the receive request carries each spell's own key.
const priest = (id: string, name: string, level: number, sphere: string) =>
  catalogueSpell({
    id,
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
  });

test.describe("same-named spells", () => {
  test.use({
    extraSpells: {
      rows: [
        priest("fortify", "Fortify", 3, "War"),
        priest("fortify_l4", "Fortify", 4, "Healing"),
        ...Array.from({ length: 60 }, (_, i) =>
          priest(
            `cure_test_${String(i).padStart(2, "0")}`,
            `Cure Test ${String(i).padStart(2, "0")}`,
            1,
            "Healing",
          ),
        ),
      ],
    },
  });

  async function receive(page: Page, query: string, row: string) {
    await openDashboard(page);
    const picker = await receivePicker(page);
    await picker.getByLabel("Search for a received spell").fill(query);
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
    await picker.getByLabel("Search for a received spell").fill("Cure Test");
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
  await roundMenu(page, "Time passes…");
  await form.getByLabel("Hours that pass").fill("1");
  await form.getByRole("button", { name: "Confirm" }).click();
  await expect(page.getByRole("status")).toContainText(
    "Expired effects: Haste",
  );
  await expect(buffBadge(page, "Haste")).toHaveCount(0);

  await expectUndoOn(page, "Cast Haste", true);
  await snap(page, "undo-cast-not-on-top-under-pass-time");
  await undoEntry(page, "Time passed: 1 h (60 rd)");
  await expect(buffBadge(page, "Haste")).toBeVisible();
  await undoEntry(page, "Cast Haste");
  await expect(buffBadge(page, "Haste")).toHaveCount(0);
});

// The log is a stack: cast, HP change, cast. The journal's one Undo (ADR 0018 step 3) names and
// pops the cast on top, then the heal: HP and slots walk back one step each.
test("cast, heal, cast: the journal Undo pops the cast on top, then the heal", async ({
  page,
  stack,
}) => {
  await prepare(stack.apiUrl, ["haste", "stoneskin"]);
  await openSpells(page);
  const max = await currentHp(page);
  const level3 = page.getByRole("region", { name: "Level 3" });
  const level4 = page.getByRole("region", { name: "Level 4" });
  const undo = page.getByTestId("undo");

  await recordHp(page, "damage", 9, "e2e troll");
  await expect.poll(() => currentHp(page)).toBe(max - 9);
  await page.getByRole("button", { name: "Cast Haste" }).click();
  await expectSlotDots(level3, 3, 0, 1);
  await recordHp(page, "heal", 4, "e2e cure light");
  await expect.poll(() => currentHp(page)).toBe(max - 5);
  await page.getByRole("button", { name: "Cast Stoneskin" }).click();
  await expectSlotDots(level4, 4, 0, 1);

  // The journal's one Undo button names the newest entry (the Stoneskin cast) only.
  for (const text of ["Cast Haste", "e2e cure light", "e2e troll"]) {
    await expectUndoOn(page, text, true);
  }
  await expectUndoOn(page, "Cast Stoneskin");
  await snap(page, "undo-names-the-cast");

  await undo.click();
  await expectSlotDots(level4, 4, 1, 1);
  await expectSlotDots(level3, 3, 0, 1);
  await expect.poll(() => currentHp(page)).toBe(max - 5);

  await undoEntry(page, "e2e cure light");
  await expect.poll(() => currentHp(page)).toBe(max - 9);
  await expectSlotDots(level3, 3, 0, 1);
  await expectUndoOn(page, "Cast Haste");
  await snap(page, "popped-cast-then-heal");

  // ADR 0019: Redo replays the popped transactions, the last popped first: its tooltip names the
  // heal, the button replays it, then Ctrl+Shift+Z replays the cast; the state is back.
  const redo = page.getByTestId("redo");
  await expect(redo).toHaveAttribute("title", /^Redo: .*e2e cure light/);
  await expect(redo).toBeEnabled();
  await snap(page, "redo-names-the-heal");
  await redo.click();
  await expect(logEntry(page, "e2e cure light")).toHaveAttribute(
    "data-status",
    "active",
  );
  await expect.poll(() => currentHp(page)).toBe(max - 5);
  await expect(redo).toHaveAttribute("title", /^Redo: .*Cast Stoneskin/);
  await page.keyboard.press("Control+Shift+Z");
  await expectSlotDots(level4, 4, 0, 1);
  await expectSlotDots(level3, 3, 0, 1);
  await expect.poll(() => currentHp(page)).toBe(max - 5);
  await expect(redo).toBeDisabled();
  await expectUndoOn(page, "Cast Stoneskin");
  await snap(page, "redone-heal-then-cast");
});
