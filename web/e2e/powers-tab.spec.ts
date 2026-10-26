import { test, expect, snap } from "./fixtures";
import { psionicist } from "./psionicist";
import {
  apiCall,
  keepATangent,
  nextRound,
  openDashboard,
  reloadSettled,
} from "./helpers";

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

  test("attack from the bar, see its result", async ({ page }) => {
    await openDashboard(page);
    expect(
      await apiCall(page, "PUT", "/api/psionics/picks", {
        picks: [
          { kind: "discipline", id: "telepathy" },
          { kind: "power", id: "contact" },
          { kind: "power", id: "mindlink" },
          { kind: "power", id: "mind_thrust" },
        ],
      }),
    ).toBe(true);
    await reloadSettled(page);
    await nextRound(page);
    await page.getByRole("tab", { name: "Powers" }).click();
    const tab = page.getByTestId("powers-tab");
    await tab.getByTestId("target-name").fill("Orc");
    await tab
      .getByRole("button", { name: "Attack with Mind Thrust" })
      .first()
      .click();
    const bar = page.getByTestId("action-bar");
    await bar.getByRole("button", { name: "Attack with Mind Thrust" }).click();
    await expect(bar.getByTestId("attack-result")).toBeVisible();
    await expect(bar.getByTestId("attack-tangents")).toContainText("Orc");
    await snap(page, "powers-bar-attack-result");
  });

  test("keep a tangent up, see the balance, Next turn charges it", async ({
    page,
  }) => {
    await openDashboard(page);
    expect(
      await apiCall(page, "PUT", "/api/psionics/picks", {
        picks: [
          { kind: "discipline", id: "telepathy" },
          { kind: "power", id: "contact" },
          { kind: "power", id: "mindlink" },
          { kind: "power", id: "mind_thrust" },
        ],
      }),
    ).toBe(true);
    await reloadSettled(page);
    await nextRound(page);
    await page.getByRole("tab", { name: "Powers" }).click();
    const tab = page.getByTestId("powers-tab");
    await expect(tab.getByTestId("maintained-card")).toHaveCount(0);
    await keepATangent(page, tab, "Orc");
    const card = tab.getByTestId("maintained-card");
    const pool = page.getByTestId("psp-value");
    await expect(card.getByTestId("maintained-row")).toContainText("Orc");
    const now = Number((await pool.innerText()).split("/")[0]);
    // The row's first Next Round charges nothing; the balance is the server's.
    const balance = card.getByTestId("maintained-balance");
    await expect(balance).toContainText(`${now} → ${now} PSPs`);
    await nextRound(page);
    await expect(balance).toContainText(`${now} → ${now - 1} PSPs`);
    await snap(page, "maintained-card");
    await nextRound(page);
    await expect(pool).toHaveText(`${now - 1} / 40`);
    await expect(balance).toContainText(`${now - 1} → ${now - 2} PSPs`);
    await card.getByRole("button", { name: /^Stop/ }).click();
    await expect(card).toHaveCount(0);
  });
});
