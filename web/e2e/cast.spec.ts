import { test, expect, snap } from "./fixtures";
import {
  buffBadge,
  currentHp,
  effectiveStr,
  logEntry,
  openDashboard,
  recordHp,
  voidEntry,
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
  return page.getByRole("region", { name: "Last Result" });
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
  await prepare(stack.apiUrl, ["wall_of_ice"]);
  await openSpells(page);
  await page.getByRole("button", { name: "Cast Wall of Ice" }).click();
  await expect(resultCard(page).getByTestId("cast-text-only")).toContainText(
    "add a buff by hand",
  );
  await expect(page.locator('[data-testid="buff"]')).toHaveCount(0);
  await snap(page, "low-confidence");
});

test("Strength cast on me by a party wizard, found by its French name", async ({
  page,
}) => {
  await openSpells(page);
  const before = await effectiveStr(page);
  const picker = page.getByRole("region", { name: "Spell cast on me" });
  await picker.getByLabel("Search for a spell cast on me").fill("Force");
  await picker.getByLabel("Caster level").fill("9");
  await picker.getByRole("button", { name: "Apply Strength" }).click();

  const card = resultCard(page).getByTestId("cast-result");
  await expect(card).toContainText("Strength cast on you, caster level 9");
  await expect(card.getByTestId("cast-rounds")).toContainText("= 540");
  await expect(card.getByTestId("cast-str-gain")).toContainText("1d6");
  await expect(buffBadge(page, "Strength")).toBeVisible();
  await expect.poll(() => effectiveStr(page)).toBeGreaterThan(before);
  await expect(
    logEntry(page, "Strength cast on me (caster level 9)"),
  ).toBeVisible();
  await snap(page, "strength-received");
});

// Effective STR rule 1 (docs/backend.md "Effective STR"): every Strength bonus stacks, total
// capped at 18; a second receive is a second buff, not a renewal.
test.describe("Strength received twice on the sheet's STR 12", () => {
  test.use({ character: { str: 12 } });

  async function receiveStrength(page: Page): Promise<number> {
    const picker = page.getByRole("region", { name: "Spell cast on me" });
    await picker.getByLabel("Search for a spell cast on me").fill("Strength");
    await picker.getByLabel("Caster level").fill("9");
    await picker.getByRole("button", { name: "Apply Strength" }).click();
    const gain = resultCard(page)
      .getByTestId("cast-result")
      .getByTestId("cast-str-gain");
    await expect(gain).toContainText("1d6");
    return Number(((await gain.textContent()) ?? "").split("= ").pop());
  }

  test("two Strength badges that add up; void the second, dismiss one by its badge", async ({
    page,
  }) => {
    await openSpells(page);
    await expect.poll(() => effectiveStr(page)).toBe(12);
    const strength = buffBadge(page, "Strength");

    const g1 = await receiveStrength(page);
    await expect(strength).toHaveCount(1);
    await expect.poll(() => effectiveStr(page)).toBe(Math.min(18, 12 + g1));
    const g2 = await receiveStrength(page);
    await expect(strength).toHaveCount(2);
    await expect
      .poll(() => effectiveStr(page))
      .toBe(Math.min(18, 12 + g1 + g2));
    await expect(strength.nth(0)).toHaveAttribute("data-applies", "true");
    await expect(strength.nth(1)).toHaveAttribute("data-applies", "true");
    await snap(page, "strength-received-twice");

    // Void the second receive: one badge left, STR back to the first cast alone.
    // Two entries share the text; the log lists the newest first.
    const second = logEntry(
      page,
      "Strength cast on me (caster level 9)",
    ).first();
    await second.getByRole("button", { name: "Void" }).click();
    await second.getByRole("button", { name: "Confirm" }).click();
    await expect(second).toHaveAttribute("data-status", "voided");
    await expect(strength).toHaveCount(1);
    await expect.poll(() => effectiveStr(page)).toBe(Math.min(18, 12 + g1));

    // Receive again, then dismiss the first badge: only that one goes, and it is logged.
    const g3 = await receiveStrength(page);
    await expect(strength).toHaveCount(2);
    await strength
      .nth(0)
      .getByRole("button", { name: "Dismiss Strength" })
      .click();
    await expect(strength).toHaveCount(1);
    await expect.poll(() => effectiveStr(page)).toBe(Math.min(18, 12 + g3));
    await expect(logEntry(page, "Dismissed Strength")).toBeVisible();
    await snap(page, "strength-one-dismissed");
  });
});

test("after Next Round the cast offers no void until Next Round is popped", async ({
  page,
  stack,
}) => {
  await prepare(stack.apiUrl, ["stoneskin"]);
  await openSpells(page);
  await page.getByRole("button", { name: "Cast Stoneskin" }).click();
  await expect(buffBadge(page, "Stoneskin")).toBeVisible();
  await page.getByRole("button", { name: "Next Round" }).click();
  await expect(logEntry(page, "Round 0 → 1")).toBeVisible();

  const entry = logEntry(page, "Cast Stoneskin");
  await expect(entry.getByRole("button", { name: "Void" })).toHaveCount(0);
  await snap(page, "void-not-offered-after-next-round");

  await voidEntry(page, "Round 0 → 1");
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

  await expect(logEntry(page, "Cast Haste").getByRole("button", { name: "Void" })).toHaveCount(0);
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
    await openSpells(page);
    const picker = page.getByRole("region", { name: "Spell cast on me" });
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

  test("a Priest spell named like a Wizard spell gets no Wizard effect", async ({
    page,
  }) => {
    const body = await receive(page, "Detect Magic", "Priest");
    expect(body.spellId).toBe("detect_magic_priest");
    const card = resultCard(page).getByTestId("cast-result");
    await expect(card.getByTestId("cast-spell-ident")).toContainText("Priest");
    await expect(card.getByTestId("cast-text-only")).toBeVisible();
    await expect(page.locator('[data-testid="buff"]')).toHaveCount(0);
    await snap(page, "priest-detect-magic-received");
  });

  test("receive search shows how many matches are not listed", async ({
    page,
  }) => {
    await openSpells(page);
    const picker = page.getByRole("region", { name: "Spell cast on me" });
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

  const form = page.getByRole("form", { name: "⏩ Time passes" });
  await page.getByRole("button", { name: "⏩ Time passes" }).click();
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

// The log is a stack (critical-qki.38): cast, HP change, cast. Only the last
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
    await expect(logEntry(page, text).getByRole("button", { name: "Void" })).toHaveCount(0);
  }
  await expect(logEntry(page, "Cast Stoneskin").getByRole("button", { name: "Void" })).toHaveCount(1);
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
  await expect(logEntry(page, "Cast Haste").getByRole("button", { name: "Void" })).toHaveCount(1);
  await expect(undo).toBeDisabled();
  await snap(page, "popped-cast-then-heal");
});
