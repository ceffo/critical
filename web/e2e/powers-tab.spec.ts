import { test, expect, snap } from "./fixtures";
import { psionicist } from "./psionicist";
import { apiCall, nextRound, openDashboard, reloadSettled } from "./helpers";

// Act > Pouvoirs (ADR 0047), the one flow only a browser proves: the tab appears for a psionicist,
// a defence chip raises its mode in one tap, and the PSPs are paid. What each row tags and groups,
// and the adjustment, are Vitest; the rules are Go tests.
test.describe("the Powers tab of a psionicist", () => {
  test.use({ character: psionicist });

  test("open the tab, raise a defence", async ({ page }) => {
    await openDashboard(page);
    expect(
      await apiCall(page, "PUT", "/api/psionics/picks", {
        picks: [
          { kind: "discipline", id: "telepathy" },
          { kind: "power", id: "thought_shield" },
        ],
      }),
    ).toBe(true);
    await reloadSettled(page);
    await nextRound(page);
    await page.getByRole("tab", { name: "Powers" }).click();
    const tab = page.getByTestId("powers-tab");
    await expect(tab.getByTestId("power-of-the-round")).toContainText("free");
    const chip = tab.getByRole("switch", { name: "Raise Thought Shield" });
    await expect(chip).toHaveAttribute("aria-checked", "false");
    await chip.click();
    await expect(chip).toHaveAttribute("aria-checked", "true");
    await expect(chip).toContainText("raised this round");
    await expect(page.getByTestId("psp-value")).toHaveText("39 / 40");
    await snap(page, "defence-raised");
  });
});
