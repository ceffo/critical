import { test, expect, snap } from "./fixtures";
import {
  chooseLanguage,
  currentHp,
  logEntry,
  openDashboard,
  recordHp,
} from "./helpers";

const unavailable = {
  status: 503,
  contentType: "application/json",
  body: JSON.stringify({
    code: "database_unavailable",
    error: "database unavailable",
  }),
};

// ADR 0025 step 10.4, the one flow only a browser proves: the banner over the real sheet, a
// second click refused without reaching the server, and the sheet reloaded before writes resume.
// The retry and refusal rules themselves are unit-tested (src/api/http.unavailable.test.ts).
test("an unavailable database shows the banner, refuses writes, then reloads the sheet", async ({
  page,
}) => {
  await openDashboard(page);
  const before = await currentHp(page);

  // The database stops answering: the write and the probe get 503 database_unavailable.
  const writes: string[] = [];
  page.on("request", (r) => {
    if (r.method() === "POST" && r.url().endsWith("/api/hp/event"))
      writes.push(r.url());
  });
  await page.route("**/api/hp/event", (route) => route.fulfill(unavailable));
  await page.route("**/api/character", (route) => route.fulfill(unavailable));

  await recordHp(page, "damage", 4, "e2e lost blow");
  const banner = page.getByTestId("database-banner");
  await expect(banner).toContainText("The database is not answering");
  await expect(banner).toContainText(
    "Your last action may not have been saved",
  );

  // A second click while the banner shows never reaches the server.
  await recordHp(page, "damage", 4, "e2e lost blow");
  expect(writes).toHaveLength(1);

  await chooseLanguage(page, "fr");
  await expect(banner).toContainText("La base de données ne répond pas");
  await snap(page, "database-unavailable");
  await chooseLanguage(page, "en");

  // The database answers again: the sheet is reloaded, the banner goes, writes resume.
  await page.unroute("**/api/character");
  await page.unroute("**/api/hp/event");
  await expect(banner).toBeHidden({ timeout: 10_000 });
  expect(await currentHp(page)).toBe(before);
  await expect(logEntry(page, "e2e lost blow")).toHaveCount(0);
  await recordHp(page, "damage", 4, "e2e landed blow");
  await expect.poll(() => currentHp(page)).toBe(before - 4);
});
