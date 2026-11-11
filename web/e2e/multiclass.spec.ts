import { test, expect, snap } from "./fixtures";
import {
  chooseLanguage,
  logEntry,
  openDashboard,
  openWorkspace,
} from "./helpers";
import { expectNoClippedText } from "./noClip";
import { asMultiClass, openClassDetail } from "./multiclass";

// What only a browser proves for a multi-class character: the card's class lines, the Experience
// activity's standings, an award's preview, ledger row and journal row each showing the split the
// server sent, and the level-up prompt of each class sending its class. The equal shares, the
// per-class levels and the hit point division are Go tests (xpsvc, charactersvc, levelupsvc); the API
// cannot create a multi-class character yet, so the real answers are overlaid (e2e/multiclass.ts).
test("multi-class: the class lines, an XP award split between the classes, a level-up per class", async ({
  page,
}) => {
  await asMultiClass(page, { pending: true });
  await openDashboard(page);

  // The card: one summary line, no single class line; the detail per class behind it.
  const character = page.getByRole("complementary", { name: "Character" });
  await expect(character.getByTestId("class")).toHaveCount(0);
  await expect(character.getByTestId("class-summary")).toHaveText(
    "Fighter 7 / Thief 9",
  );
  const detail = await openClassDetail(page);
  await expect(detail.getByTestId("class-level-fighter")).toHaveText("Level 7");
  await expect(detail.getByTestId("class-level-thief")).toHaveText("Level 9");
  await expect(detail.getByTestId("class-xp-fighter")).toHaveText(
    "64,000 / 125,000 XP",
  );
  await expect(detail.getByTestId("class-limit-thief")).toBeVisible();
  // A row opens the Experience activity and closes the detail.
  await detail.getByTestId("class-entry-fighter").click();
  await expect(detail).toHaveCount(0);
  await expect(
    page.getByRole("region", { name: "Experience", exact: true }),
  ).toBeVisible();
  await openWorkspace(page, "Act");

  // A level-up prompt per class, each naming its class and sending it.
  const prompts = character.getByTestId("level-up");
  await expect(prompts).toHaveCount(2);
  await expect(prompts.first()).toContainText("Fighter: level 8 reached");
  await expect(prompts.last()).toContainText("Thief: level 10 reached");
  const sent = page.waitForRequest(
    (r) =>
      r.url().endsWith("/api/character/levelup/roll") && r.method() === "POST",
  );
  await prompts.last().getByTestId("level-up-roll").click();
  expect((await sent).postDataJSON()).toEqual({ level: 10, class: "thief" });

  // The Experience activity: the standings, then the award with its split.
  await openWorkspace(page, "Experience");
  const ws = page.getByRole("region", { name: "Experience", exact: true });
  await expect(ws.getByTestId("xp-class-fighter")).toContainText(
    "Fighter · level 7",
  );
  await expect(ws.getByTestId("xp-class-thief")).toContainText(
    "Thief · level 9",
  );
  const add = ws.getByRole("region", { name: "Add XP" });
  await add.getByLabel("Amount").fill("1001");
  await add.getByRole("checkbox").uncheck();
  await add.getByLabel("Reason").fill("Goblin warren");
  await expect(add.getByTestId("xp-split")).toHaveText(
    "Split: Fighter 500 · Thief 500",
  );
  await add.getByLabel("Reason").press("Enter");
  const row = ws
    .getByRole("region", { name: "Ledger" })
    .getByRole("listitem", { name: "Goblin warren" });
  await expect(row.getByTestId("xp-row-split")).toHaveText(
    "Fighter 500 · Thief 500",
  );

  // The journal words the same split.
  await openWorkspace(page, "Act");
  await expect(
    logEntry(page, "XP +1,001 (Fighter 500 · Thief 500): Goblin warren"),
  ).toBeVisible();
});

test("dual-class: the old class is shown frozen beside the active one", async ({
  page,
}) => {
  await asMultiClass(page, { dual: true });
  await openDashboard(page);
  const detail = await openClassDetail(page);
  await expect(detail.getByTestId("class-entry-fighter")).toHaveAttribute(
    "data-state",
    "frozen",
  );
  await expect(detail.getByTestId("class-frozen-fighter")).toHaveText("Frozen");
  await expect(detail.getByTestId("class-entry-mage")).toHaveAttribute(
    "data-state",
    "active",
  );
  await expect(detail.getByTestId("class-xp-fighter")).toHaveCount(0);
});

test("multi-class: the XP bonus rule names every active class's prime requisites, in French then English", async ({
  page,
}) => {
  await asMultiClass(page);
  await openDashboard(page);
  await openWorkspace(page, "Experience");
  const top = page.getByTestId("xp-top");
  // The bonus rule is the union of the active classes' prime requisites, French first, at the
  // two large layout viewports (the sentence wraps, never cut).
  await chooseLanguage(page, "fr");
  for (const vp of [
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    await expect(top.getByTestId("xp-bonus-hint")).toHaveText(
      "+10 % sur les XP gagnés quand la Force et la Dextérité sont tous à 16 ou plus",
    );
    await expectNoClippedText(top, `add XP, fr ${vp.width}`);
    await snap(page, `multi-xp-fr-${vp.width}`);
  }
  await chooseLanguage(page, "en");
  for (const vp of [
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    await expect(top.getByTestId("xp-bonus-hint")).toHaveText(
      "+10% on XP earned when Strength and Dexterity are all 16 or more",
    );
    await expectNoClippedText(top, `add XP, en ${vp.width}`);
    await snap(page, `multi-xp-en-${vp.width}`);
  }
});
