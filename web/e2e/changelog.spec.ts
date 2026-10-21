import type { Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import { chooseLanguage, openDashboard } from "./helpers";
import { expectNoClippedText } from "./noClip";

// The changelog panel (ADR 0039 step 7), opened from the version badge. Only a browser proves the
// overlay geometry (two thirds of the viewport, no screen height change), the keyboard and
// click-outside closing, and the scroll that loads the next page; the pagination rules are unit tests
// (src/lib/changelogPages.test.ts). The API is mocked: 25 versions, 10 per page.
const TOTAL = 25;
const versionName = (i: number) => `1.${TOTAL - i}.0`;

async function mockChangelog(page: Page, opts: { failOnce?: boolean } = {}) {
  const asked: (string | null)[] = [];
  let failed = false;
  await page.route("**/api/changelog**", async (route) => {
    const url = new URL(route.request().url());
    const cursor = url.searchParams.get("cursor");
    asked.push(cursor);
    if (opts.failOnce && cursor && !failed) {
      failed = true;
      return route.fulfill({
        status: 500,
        json: { error: "boom", code: "internal" },
      });
    }
    const from = cursor ? Number(cursor) : 0;
    const to = Math.min(from + 10, TOTAL);
    const versions = Array.from({ length: to - from }, (_, k) => {
      const i = from + k;
      return {
        version: versionName(i),
        // Every fourth version has no user-facing change; others have both groups, one a long line.
        features: i % 4 === 3 ? [] : [`Added feature number ${i} to the app`],
        fixes:
          i % 4 === 3
            ? []
            : [
                i === 0
                  ? "Fixed a very long line that has to wrap inside the panel and never be cut off at any of the viewports the layout is checked at, in French or in English"
                  : `Fixed bug ${i}`,
              ],
      };
    });
    await route.fulfill({
      json: to < TOTAL ? { versions, nextCursor: String(to) } : { versions },
    });
  });
  return asked;
}

test("badge opens the panel, infinite scroll loads each page once, retry, Esc / close / outside close", async ({
  page,
}) => {
  const asked = await mockChangelog(page, { failOnce: true });
  await openDashboard(page);
  const footer = page.getByTestId("dice-footer");
  const heightBefore = (await footer.boundingBox())!.height;
  const panel = page.getByTestId("changelog-panel");
  const list = page.getByTestId("changelog-list");
  const open = page.getByTestId("version-open");

  // Enter on the focused badge opens it; Escape closes it and gives the focus back.
  await open.focus();
  await page.keyboard.press("Enter");
  await expect(panel).toBeVisible();
  await expect(page.getByTestId("changelog-version")).toHaveCount(10);
  await expect(
    page.getByTestId("changelog-version").first().getByRole("heading").first(),
  ).toHaveText(versionName(0));
  const vp = page.viewportSize()!;
  await expect
    .poll(async () => {
      const b = (await panel.boundingBox())!;
      return [b.width / vp.width, b.height / vp.height].map((r) =>
        Math.abs(r - 2 / 3) < 0.01 ? "ok" : String(r),
      );
    })
    .toEqual(["ok", "ok"]);
  expect(
    (await footer.boundingBox())!.height,
    "the overlay changes no screen height",
  ).toBe(heightBefore);
  await page.keyboard.press("Escape");
  await expect(panel).toBeHidden();
  await expect(open).toBeFocused();

  // Scrolling to the end asks the second page, which fails once: an error with a Retry; the list
  // kept what it had; Retry loads it, and the third page ends the list.
  asked.length = 0;
  await open.click();
  await expect(page.getByTestId("changelog-version")).toHaveCount(10);
  await list.evaluate((el) => (el.scrollTop = el.scrollHeight));
  await expect(page.getByTestId("changelog-error")).toBeVisible();
  await expect(page.getByTestId("changelog-version")).toHaveCount(10);
  await page.getByRole("button", { name: "Retry" }).click();
  await expect(page.getByTestId("changelog-version")).toHaveCount(20);
  await list.evaluate((el) => (el.scrollTop = el.scrollHeight));
  await expect(page.getByTestId("changelog-version")).toHaveCount(TOTAL);
  await page.waitForTimeout(300);
  expect(asked, "one request per page, the failed one asked again").toEqual([
    null,
    "10",
    "10",
    "20",
  ]);
  await expectNoClippedText(panel, "changelog panel");
  await expect(page.getByText("No user-facing changes").first()).toBeVisible();

  // The close control, then a click outside.
  await page.getByRole("button", { name: "Close" }).click();
  await expect(panel).toBeHidden();
  await open.click();
  await expect(panel).toBeVisible();
  await page.mouse.click(5, 5);
  await expect(panel).toBeHidden();
});

// Screens and sizes: the panel opens from the badge on the dashboard, roster, creation and admin, and
// the login screen's badge is plain text (the endpoint needs a session). French chrome, English bullets.
test("the panel on every screen at 1920x1080 and 2560x1440; the login badge is not a button", async ({
  page,
  context,
}) => {
  test.setTimeout(120_000);
  await mockChangelog(page);
  const sizes = [
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ];
  const panel = page.getByTestId("changelog-panel");
  const check = async (name: string) => {
    for (const vp of sizes) {
      await page.setViewportSize(vp);
      await page.getByTestId("version-open").click();
      await expect(panel).toBeVisible();
      await expect(page.getByTestId("changelog-version")).toHaveCount(10);
      // The open animation scales it from 95 %: measure once it is still.
      await expect
        .poll(async () => {
          const b = (await panel.boundingBox())!;
          return [b.width / vp.width, b.height / vp.height].map((r) =>
            Math.abs(r - 2 / 3) < 0.01 ? "ok" : String(r),
          );
        })
        .toEqual(["ok", "ok"]);
      await expectNoClippedText(panel, `${name} ${vp.width}`);
      await snap(page, `${name}-${vp.width}x${vp.height}`);
      await page.keyboard.press("Escape");
      await expect(panel).toBeHidden();
    }
  };

  await openDashboard(page);
  await chooseLanguage(page, "fr");
  await check("dashboard");

  await page.setViewportSize(sizes[0]!);
  await page.getByTestId("roster-open").click();
  await expect(page.getByTestId("roster")).toBeVisible();
  await check("roster");

  await page.goto("/create");
  await expect(page.getByTestId("creation-step")).toBeVisible();
  await check("creation");

  await page.goto("/admin");
  await expect(page.getByTestId("admin-users")).toBeVisible();
  await check("admin");

  // No session: the badge shows the version as plain text.
  await context.clearCookies();
  await page.goto("/login");
  await expect(page.getByTestId("version-text")).toBeVisible();
  await expect(page.getByTestId("version-open")).toHaveCount(0);
  await snap(page, "login-plain");
});
