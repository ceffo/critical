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

  // The one flow only a browser proves for a table-paid power: the controls come from the table's
  // modifier rows, the cost shown is the server's preview, and Use spends exactly that. The rules and
  // refusals are Go tests (the contact preview) and Vitest (the controls, the request).
  test("contact: pick the modifiers, see the server's cost, use it", async ({
    page,
  }) => {
    await openDashboard(page);
    expect(
      await apiCall(page, "PUT", "/api/psionics/picks", {
        picks: [
          { kind: "discipline", id: "telepathy" },
          { kind: "power", id: "contact" },
        ],
      }),
    ).toBe(true);
    await reloadSettled(page);
    await nextRound(page);
    await page.getByRole("tab", { name: "Powers" }).click();
    const tab = page.getByTestId("powers-tab");
    await tab.getByTestId("target-name").fill("Orc");
    const band = tab.getByTestId("target-band");
    await band.getByRole("radio", { name: "Open" }).click();
    await band.getByLabel("Level", { exact: true }).fill("3");
    await tab
      .locator("[data-testid=power-row][data-id=contact]")
      .getByRole("button")
      .last()
      .click();
    const form = page.getByTestId("action-bar").getByTestId("table-form");
    const preview = form.getByTestId("table-preview");
    await expect(preview).toContainText("Cost 3 PSPs · score 15");
    const toggle = form.locator("input[type=checkbox]").first();
    await toggle.check();
    // A row changes the score the server computes; the cost is the table's for the level.
    await expect(preview).toContainText("score 13");
    const shown = await preview.innerText();
    const cost = Number(/Cost (\d+) PSPs/.exec(shown)![1]);
    await snap(page, "contact-table-form");
    await form.getByRole("button", { name: "Use Contact" }).click();
    // The d20 is the server's: a failed check spends part of the cost the preview showed, a passed one
    // all of it. Either way the pool falls by what the result card says, never past the previewed cost.
    const result = form.getByTestId("use-result");
    await expect(result).toBeVisible();
    const spent = Number(/PSPs\s*\D?(\d+)/.exec(await result.innerText())![1]);
    expect(spent).toBeGreaterThan(0);
    expect(spent).toBeLessThanOrEqual(cost);
    await expect(page.getByTestId("psp-value")).toHaveText(
      `${40 - spent} / 40`,
    );
    await snap(page, "contact-table-used");
  });
});
