import { test, expect, snap } from "./fixtures";
import { chooseLanguage, currentHp, openDashboard } from "./helpers";

test("app loads and shows the character's HP", async ({ page }) => {
  await openDashboard(page);
  await expect(page.getByRole("heading", { level: 1 })).not.toHaveText(
    "Loading...",
  );
  expect(await currentHp(page)).toBeGreaterThan(0);
  // The round screen's regions are all there.
  for (const name of ["Actions", "Detail", "Journal"]) {
    await expect(page.getByRole("region", { name, exact: true })).toBeVisible();
  }
  await expect(page.getByRole("group", { name: "HP change" })).toBeVisible();
  await snap(page, "dashboard");
});

test("fr smoke: French toggle shows no raw i18n keys", async ({ page }) => {
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  await expect(
    page.getByRole("button", { name: "Dégâts", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Tour suivant" }),
  ).toBeVisible();

  const body = (await page.locator("body").innerText()).split("\n");
  // A raw key looks like `ns.key` / `hp.applyDamage`: dotted lowercase identifier.
  const raw = body.filter((l) => /^[a-z]+(\.[a-zA-Z_]+)+$/.test(l.trim()));
  expect(raw).toEqual([]);
  // ...same for placeholders / titles / aria-labels.
  const attrs = await page.evaluate(() =>
    [
      ...document.querySelectorAll("[placeholder],[title],[aria-label]"),
    ].flatMap((el) =>
      ["placeholder", "title", "aria-label"].map(
        (a) => el.getAttribute(a) ?? "",
      ),
    ),
  );
  expect(attrs.filter((v) => /^[a-z]+(\.[a-zA-Z_]+)+$/.test(v))).toEqual([]);
  await snap(page, "dashboard-fr");
});
