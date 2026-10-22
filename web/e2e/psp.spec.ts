import { test, expect } from "./fixtures";
import { psionicist } from "./psionicist";
import { logEntry, openDashboard, undoEntry } from "./helpers";

// What only a browser proves for the PSP pool (ADR 0043 steps 4 to 6): the panel on the character card
// shows current / maximum, a spend from the panel changes the pool through the real API, and the
// journal's Undo puts it back. The ledger's arithmetic, the cap and the undo are Go tests (actionlog,
// actionlogsvc, composition psionics_test).
test.describe("a psionicist", () => {
  test.use({ character: psionicist });

  test("the PSP panel shows the pool, a spend changes it, Undo restores it", async ({
    page,
  }) => {
    await openDashboard(page);
    const panel = page.getByTestId("psp-panel");
    const value = panel.getByTestId("psp-value");
    await expect(value).toHaveText("40 / 40");

    await panel.getByLabel("PSP amount").fill("12");
    await panel.getByRole("button", { name: "Spend" }).click();
    await expect(value).toHaveText("28 / 40");
    await expect(panel.getByRole("meter")).toHaveAttribute(
      "aria-valuenow",
      "28",
    );
    await expect(logEntry(page, "PSPs spent 12")).toBeVisible();

    await undoEntry(page, "PSPs spent 12");
    await expect(value).toHaveText("40 / 40");
  });

  test("recovering over 2 hours of rest adds 12 PSPs (Table 6)", async ({
    page,
  }) => {
    await openDashboard(page);
    const panel = page.getByTestId("psp-panel");
    const value = panel.getByTestId("psp-value");
    await panel.getByLabel("PSP amount").fill("30");
    await panel.getByRole("button", { name: "Spend" }).click();
    await expect(value).toHaveText("10 / 40");

    await panel.getByRole("spinbutton", { name: "Hours" }).fill("2");
    await panel
      .getByRole("combobox", { name: "Activity" })
      .selectOption("resting");
    await panel.getByRole("button", { name: "Recover", exact: true }).click();
    await expect(value).toHaveText("22 / 40");
  });
});

// What only a browser proves for the known powers (ADR 0043 step 9): the rail lists the Powers
// activity for a psionicist only, a discipline and a power picked from the catalogue search are saved
// whole and counted against Table 4's allowance, a pick over it is refused with the rule named and the
// list kept, the list survives a reload, and the journal's Undo restores the previous list. The rules,
// the refusals and the undo codec are Go tests (psionics, psionicsvc, composition psionics_test).
test.describe("a psionicist's known powers", () => {
  test.use({ character: psionicist });

  test("pick a discipline and a power, a pick over the allowance is refused, Undo restores", async ({
    page,
  }) => {
    await openDashboard(page);
    const rail = page.getByTestId("rail");
    await rail.getByRole("button", { name: "Powers" }).click();
    const panel = page.getByTestId("known-powers");
    const count = (k: string) => panel.getByTestId(`powers-count-${k}`);
    await expect(count("disciplines")).toContainText("0 / 2");

    await panel.getByRole("button", { name: "Add Telepathy" }).click();
    await expect(count("disciplines")).toContainText("1 / 2");
    await panel
      .getByRole("searchbox", { name: "Search powers..." })
      .fill("contact");
    const row = panel.getByTestId("pick-row").filter({ hasText: "Contact" });
    await expect(row).toHaveCount(1);
    await row.getByRole("button", { name: "Add Contact" }).click();
    await expect(count("devotions")).toContainText("1 / 10");
    await expect(
      panel.getByTestId("known-power").filter({ hasText: "Contact" }),
    ).toBeVisible();

    await panel.getByRole("button", { name: "Add Clairsentience" }).click();
    await expect(count("disciplines")).toContainText("2 / 2");
    await panel.getByRole("button", { name: "Add Psychokinesis" }).click();
    const refusal = panel.getByTestId("powers-refusal");
    await expect(refusal).toContainText("Over the allowance for this level");
    await expect(count("disciplines")).toContainText("2 / 2");
    await expect(panel.getByTestId("known-discipline")).toHaveCount(2);

    await page.reload();
    await rail.getByRole("button", { name: "Powers" }).click();
    await expect(count("disciplines")).toContainText("2 / 2");

    await rail.getByRole("button", { name: "Act" }).click();
    await undoEntry(page, "Known powers: 2 → 3 picks");
    await rail.getByRole("button", { name: "Powers" }).click();
    await expect(count("disciplines")).toContainText("1 / 2");
    await expect(panel.getByTestId("known-discipline")).toHaveCount(1);
  });

  test("removing a power takes its relearns, removing a relearn takes one", async ({
    page,
  }) => {
    await openDashboard(page);
    await page
      .getByTestId("rail")
      .getByRole("button", { name: "Powers" })
      .click();
    const panel = page.getByTestId("known-powers");
    await panel.getByRole("button", { name: "Add Telepathy" }).click();
    await panel
      .getByRole("searchbox", { name: "Search powers..." })
      .fill("contact");
    await panel
      .getByTestId("pick-row")
      .filter({ hasText: "Contact" })
      .getByRole("button", { name: "Add Contact" })
      .click();
    const devotions = panel.getByTestId("powers-count-devotions");
    const rows = panel.getByTestId("known-power");
    await panel.getByRole("button", { name: "Relearn" }).click();
    await panel.getByRole("button", { name: "Relearn" }).click();
    await expect(devotions).toContainText("3 / 10");
    await expect(rows).toHaveCount(3);

    // One relearn goes, the other and the power stay.
    await rows
      .nth(1)
      .getByRole("button", { name: /^Remove/ })
      .click();
    await expect(devotions).toContainText("2 / 10");
    await expect(rows).toHaveCount(2);

    // The power goes with its relearn, no refusal.
    await rows
      .first()
      .getByRole("button", { name: /^Remove/ })
      .click();
    await expect(devotions).toContainText("0 / 10");
    await expect(rows).toHaveCount(0);
    await expect(panel.getByTestId("powers-refusal")).toHaveCount(0);
  });
});
