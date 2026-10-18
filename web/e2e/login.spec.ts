import { test, expect, snap } from "./fixtures";

// The login flow (ADR 0029 steps 6 and 8.1), the one path only a browser proves: no session lands
// on the login page, the dev login opens the dashboard, Log out in the cog menu ends the session and
// shows the login page again. The refusal wording is unit-tested (src/lib/session.test.ts).
test("dev login, dashboard, log out, login page", async ({ page, context }) => {
  await context.clearCookies();
  await page.goto("/");
  await expect(page).toHaveURL(/\/login$/);
  await page.getByRole("radio", { name: "Français" }).click();
  const card = page.getByTestId("login-card");
  await expect(card.getByRole("heading", { name: "Connexion" })).toBeVisible();
  const dev = card.getByRole("button", { name: /^Se connecter comme/ });
  await expect(dev).toBeVisible();
  await snap(page, "login-fr");

  await dev.click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByTestId("hp-value")).toHaveText(/\d+ \/ \d+/);

  await page.getByTestId("settings-trigger").click();
  await expect(page.getByTestId("settings-account")).toContainText("@");
  await snap(page, "settings-account-fr");
  await page
    .getByTestId("settings-account")
    .getByRole("button", { name: /^Déconnecter/ })
    .click();
  await expect(page).toHaveURL(/\/login$/);
  await expect(dev).toBeVisible();

  // The session is gone: the dashboard sends back to the login page.
  await page.goto("/");
  await expect(page).toHaveURL(/\/login$/);
});
