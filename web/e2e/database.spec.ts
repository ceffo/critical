import { test, expect } from "./fixtures";
import { currentHp, logEntry, openDashboard, recordHp } from "./helpers";

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
// The first write is the uncertain case: the server saves it (its COMMIT landed after the request
// deadline) but the browser got 503, so after the reload the journal shows it exactly once.
// The retry and refusal rules themselves are unit-tested (src/api/http.unavailable.test.ts); the
// banner's no-cut audit and screenshots, fr and en, are in layout.spec.
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
  // The write reaches the server and is saved; the browser gets the deadline's 503.
  await page.route("**/api/hp/event", async (route) => {
    await route.fetch();
    await route.fulfill(unavailable);
  });
  await page.route("**/api/character", (route) => route.fulfill(unavailable));

  await recordHp(page, "damage", 4, "e2e late commit", { refused: true });
  const banner = page.getByTestId("database-banner");
  await expect(banner).toContainText("The database is not answering");
  await expect(banner).toContainText(
    "Your last action may not have been saved",
  );

  // A second click while the banner shows never reaches the server.
  await recordHp(page, "damage", 4, "e2e late commit", { refused: true });
  expect(writes).toHaveLength(1);

  // The database answers again: the sheet is reloaded, the banner goes, writes resume.
  await page.unroute("**/api/character");
  await page.unroute("**/api/hp/event");
  await expect(banner).toBeHidden({ timeout: 10_000 });
  // The reload shows the saved write once, and the refused click is nowhere.
  expect(await currentHp(page)).toBe(before - 4);
  await expect(logEntry(page, "e2e late commit")).toHaveCount(1);
  await recordHp(page, "damage", 4, "e2e landed blow");
  await expect.poll(() => currentHp(page)).toBe(before - 8);
  await expect(logEntry(page, "e2e late commit")).toHaveCount(1);
});
