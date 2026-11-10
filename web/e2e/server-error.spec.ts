import { test, expect, snap } from "./fixtures";
import { chooseLanguage, openDashboard } from "./helpers";
import { expectNoClippedText } from "./noClip";

// An unknown server failure (HTTP 500, no known code) shows a worded toast in the player's language
// and never the raw body. Only a browser proves the toast's placement over the settings cog and the
// dice bar, the keyboard dismissal and the screenshots, fr first; the wording of each failure
// (`{}`, text, network) is unit-tested (src/api/http.fallback.test.ts). The server's 500 is faked
// by a route intercept on the HP write.
const VIEWPORTS = [
  { width: 1920, height: 1080 },
  { width: 2560, height: 1440 },
];

const LANGS = [
  {
    lang: "fr" as const,
    group: "Modifier les PV",
    amount: "Montant",
    damage: "Dégâts",
    wording: "Une erreur est survenue sur le serveur",
    title: "Erreur",
  },
  {
    lang: "en" as const,
    group: "HP change",
    amount: "Amount",
    damage: "Damage",
    wording: "Something went wrong on the server",
    title: "Error",
  },
];

test("a 500 with an empty body shows a worded toast, dismissed by Escape, over nothing it blocks", async ({
  page,
}) => {
  await openDashboard(page);
  await page.route("**/api/hp/event", (route) =>
    route.fulfill({ status: 500, contentType: "application/json", body: "{}" }),
  );
  for (const l of LANGS) {
    await chooseLanguage(page, l.lang);
    for (const vp of VIEWPORTS) {
      await page.setViewportSize(vp);
      const box = page.getByRole("group", { name: l.group });
      await box.getByLabel(l.amount).fill("3");
      await box.getByRole("button", { name: l.damage, exact: true }).click();
      const toast = page.getByRole("alert");
      await expect(toast).toContainText(l.wording);
      await expect(toast).toContainText(l.title);
      await expect(toast).not.toContainText("{}");
      await expect(page.locator("body")).not.toContainText("{}");
      await expectNoClippedText(toast, `server error toast ${l.lang}`);
      await snap(page, `server-error-${l.lang}-${vp.width}x${vp.height}`);

      // Escape dismisses it, and the settings cog and the dice bar are reachable again.
      await page.keyboard.press("Escape");
      await expect(toast).toBeHidden();
      await page.getByTestId("settings-trigger").click({ trial: true });
    }
  }
});
