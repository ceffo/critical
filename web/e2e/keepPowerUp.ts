import { expect, type Page } from "@playwright/test";
import { reloadSettled } from "./helpers";

/**
 * Gives the open dashboard's psionicist a paying power kept up: picks Metapsionics and Intensify, then
 * uses it for 3 PSPs with a high score so the check succeeds (the d20 is the server's, a natural 20
 * always fails: the use is repeated until the power is kept). The page is reloaded on the result.
 */
export async function keepPowerUp(page: Page) {
  const origin = new URL(page.url()).origin;
  const call = (method: string, url: string, data: unknown) =>
    page.request.fetch(url, { method, data, headers: { Origin: origin } });
  const picks = await call("PUT", "/api/psionics/picks", {
    picks: [
      { kind: "discipline", id: "metapsionics" },
      { kind: "power", id: "intensify" },
    ],
  });
  expect(picks.ok(), await picks.text()).toBe(true);
  for (let attempt = 0; attempt < 8; attempt++) {
    const res = await call("POST", "/api/psionics/powers/intensify/use", {
      modifiers: [],
      cost: 3,
      score: 25,
    });
    expect(res.ok(), await res.text()).toBe(true);
    const { check } = await res.json();
    if (check.success) {
      await reloadSettled(page);
      return;
    }
  }
  throw new Error("Intensify never succeeded");
}
