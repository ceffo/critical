import { test, expect, snap } from "./fixtures";
import { openDashboard } from "./helpers";

const isDark = (page: import("@playwright/test").Page) =>
  page.evaluate(() => document.documentElement.classList.contains("dark"));

test("theme: defaults to the OS scheme and follows it live", async ({
  page,
}) => {
  await page.emulateMedia({ colorScheme: "light" });
  await openDashboard(page);
  expect(await isDark(page)).toBe(false);
  await page.emulateMedia({ colorScheme: "dark" });
  await expect.poll(() => isDark(page)).toBe(true);
});

test("theme: toggle cycles light/dark/system and persists across reload", async ({
  page,
}) => {
  await page.emulateMedia({ colorScheme: "light" });
  await openDashboard(page);
  const toggle = page.getByTestId("theme-toggle");
  await expect(toggle).toHaveAttribute("data-theme", "system");

  await toggle.click(); // system -> light
  await expect(toggle).toHaveAttribute("data-theme", "light");
  await toggle.click(); // -> dark
  await expect(toggle).toHaveAttribute("data-theme", "dark");
  expect(await isDark(page)).toBe(true);

  // Persisted, and applied before React mounts (no light flash).
  await page.reload();
  expect(await isDark(page)).toBe(true);
  await expect(page.getByTestId("theme-toggle")).toHaveAttribute(
    "data-theme",
    "dark",
  );

  await page.getByTestId("theme-toggle").click(); // -> system (OS is light)
  expect(await isDark(page)).toBe(false);
  await page.getByTestId("theme-toggle").click(); // -> light
  await page.emulateMedia({ colorScheme: "dark" });
  expect(await isDark(page)).toBe(false); // explicit light ignores the OS
});

test("theme: .dark is on <html> before the app mounts", async ({ page }) => {
  await page.addInitScript(() =>
    localStorage.setItem("critical.theme", "dark"),
  );
  await page.route("**/src/main.tsx*", (r) => r.abort());
  await page.goto("/");
  expect(await isDark(page)).toBe(true);
});

test("theme: labels are translated", async ({ page }) => {
  await openDashboard(page);
  await expect(page.getByTestId("theme-toggle")).toHaveText("System");
  await page.getByRole("button", { name: "FR" }).click();
  await expect(page.getByTestId("theme-toggle")).toHaveText("Système");
});

test("theme: dashboard screens in light and dark", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "light" });
  await openDashboard(page);
  const toggle = page.getByTestId("theme-toggle");
  const tabs = await page.getByRole("tab").all();
  for (const mode of ["light", "dark"]) {
    while ((await toggle.getAttribute("data-theme")) !== mode)
      await toggle.click();
    for (const tab of tabs) {
      await tab.click();
      const name = ((await tab.textContent()) ?? "tab")
        .trim()
        .toLowerCase()
        .replace(/\W+/g, "-");
      await snap(page, `${mode}-${name}`);
    }
  }
});
