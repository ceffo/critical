import { test, expect, snap } from "./fixtures";
import { combatLog, openDashboard } from "./helpers";

const VIEWPORTS = [
  { width: 1280, height: 800 },
  { width: 1440, height: 900 },
  { width: 1920, height: 1080 },
  { width: 2560, height: 1440 },
];

// What only a browser proves: the composer's buttons and field build a formula, Enter and Roll reach
// the server, the card and the journal render the dice entry, Reroll adds a new entry, and a favourite
// saved on the server comes back after a reload. The grammar and limits are Go tests (dicesvc);
// the dice are random, so only relations between values are asserted.
test("dice composer: build, roll, reroll, save a favourite, roll it", async ({
  page,
}) => {
  await openDashboard(page);
  const composer = page.getByRole("form", { name: "Dice" });
  const card = page.getByRole("region", { name: "Detail", exact: true });
  const journal = combatLog(page);
  const formula = composer.getByLabel("Formula");

  // Click two d6 and a d4, then +1: the badges count, a right click removes one die.
  await composer.getByTestId("die-6").click();
  await composer.getByTestId("die-6").click();
  await composer.getByTestId("die-4").click();
  await composer.getByRole("button", { name: "Add 1" }).click();
  await expect(formula).toHaveValue("2d6+d4+1");
  await expect(composer.getByTestId("die-badge-6")).toHaveText("2 −");
  await composer.getByTestId("die-4").click({ button: "right" });
  await expect(formula).toHaveValue("2d6+1");
  await expect(composer.getByTestId("die-badge-4")).toHaveCount(0);

  // Roll: the card shows the server's roll, the total adds up from what the card shows.
  await composer.getByRole("button", { name: "Roll", exact: true }).click();
  const result = card.getByTestId("dice-result");
  await expect(result).toBeVisible();
  await expect(result.getByTestId("dice-formula")).toHaveText("2d6+1");
  const total = async () =>
    Number(await result.getByTestId("dice-total").textContent());
  const first = await total();
  expect(first).toBeGreaterThanOrEqual(3);
  expect(first).toBeLessThanOrEqual(13);
  await expect(result.getByTestId("dice-group")).toHaveCount(1);
  await expect(journal.locator("[data-selected=true]")).toContainText(
    `Dice 2d6+1 = ${first}`,
  );

  // Reroll from the journal row: a new entry, still the same formula.
  const rows = journal.getByText(/^Dice 2d6\+1 = \d+$/);
  await expect(rows).toHaveCount(1);
  await journal
    .locator("[data-selected=true]")
    .getByRole("button", { name: "Reroll" })
    .click();
  await expect(rows).toHaveCount(2);

  // Reroll from the card too.
  await result.getByRole("button", { name: "Reroll" }).click();
  await expect(rows).toHaveCount(3);

  // A formula the server refuses turns the field red; editing clears it.
  await formula.fill("2d6*3");
  await composer.getByRole("button", { name: "Roll", exact: true }).click();
  await expect(composer.getByTestId("dice-error")).toBeVisible();
  await expect(formula).toHaveAttribute("aria-invalid", "true");
  await formula.fill("d%+d4");
  await expect(composer.getByTestId("dice-error")).toHaveCount(0);

  // Enter rolls; the star saves the formula on the server.
  await formula.press("Enter");
  await expect(result.getByTestId("dice-formula")).toHaveText("d%+d4");
  await composer.getByRole("button", { name: "Save as a favourite" }).click();
  const fav = composer.getByTestId("dice-favourite");
  await expect(fav).toHaveCount(1);
  await expect(fav).toContainText("d%+d4");

  // A second favourite goes last; moving it earlier is kept by the server.
  await formula.fill("d6");
  await composer.getByRole("button", { name: "Save as a favourite" }).click();
  await expect(fav).toHaveText([/d%\+d4/, /d6/]);
  await composer.getByRole("button", { name: "Move d6 earlier" }).click();
  await expect(fav).toHaveText([/d6/, /d%\+d4/]);

  // The favourites and their order survive a reload (kept by the server); a chip rolls.
  await page.reload();
  await expect(page.getByTestId("hp-value")).toHaveText(/\d+ \/ \d+/);
  const favReloaded = page
    .getByRole("form", { name: "Dice" })
    .getByTestId("dice-favourite");
  await expect(favReloaded).toHaveText([/d6/, /d%\+d4/]);
  await favReloaded.getByRole("button", { name: "Roll d%+d4" }).click();
  await expect(result.getByTestId("dice-formula")).toHaveText("d%+d4");
  await expect(result.getByTestId("dice-group")).toHaveCount(2);
  await expect(journal.getByText(/^Dice d%\+d4 = \d+$/)).toHaveCount(2);

  for (const vp of VIEWPORTS) {
    await page.setViewportSize(vp);
    await expect(composer).toBeVisible();
    const scroll = await page.evaluate(() => {
      const el = document.scrollingElement ?? document.documentElement;
      return {
        x: el.scrollWidth - el.clientWidth,
        y: el.scrollHeight - el.clientHeight,
      };
    });
    expect(scroll, "no page scroll").toEqual({ x: 0, y: 0 });
    await snap(page, `dice-${vp.width}x${vp.height}`);
  }

  // Undo pops the last dice roll.
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.getByTestId("undo").click();
  await expect(journal.locator('[data-status="voided"]')).toHaveCount(1);
  await expect(journal.locator('[data-status="voided"]')).toContainText(
    "Dice d%+d4",
  );
});
