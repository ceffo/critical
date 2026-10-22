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
    await panel.getByRole("button", { name: "Recover" }).last().click();
    await expect(value).toHaveText("22 / 40");
  });
});
