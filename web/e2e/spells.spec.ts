import { test, expect, snap } from "./fixtures";
import { combatLog, expectSlotDots, logEntry, openDashboard } from "./helpers";
import type { Page } from "@playwright/test";

async function openSpells(page: Page) {
  await openDashboard(page);
  await page.getByRole("tab", { name: "Spells" }).click();
  await expect(
    page.getByRole("button", { name: "Prepare spells" }),
  ).toBeVisible();
}

async function add(page: Page, ...names: string[]) {
  for (const name of names) {
    await page
      .getByRole("button", { name: `Add ${name}`, exact: true })
      .click();
  }
}

async function put(apiUrl: string, spells: string[]) {
  return fetch(`${apiUrl}/api/spells/prepared`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ spells }),
  });
}

test("4 level-1 spells are refused with the server's message, 3 are accepted", async ({
  page,
  stack,
}) => {
  await openSpells(page);
  await page.getByRole("button", { name: "Prepare spells" }).click();
  const editor = page.getByRole("form", { name: "Prepare today's spells" });

  await page.getByLabel("Search the spellbook").fill("a");
  await add(page, "Alarm", "Burning Hands", "Chromatic Orb", "Detect Magic");
  await expect(editor.getByTestId("draft-level-1")).toContainText(
    "4 / 3 prepared",
  );
  await snap(page, "four-level-1-draft");

  await editor.getByRole("button", { name: "Save preparation" }).click();
  await expect(page.getByRole("alert")).toContainText(
    "too many spells for this level",
  );
  await expect(page.getByRole("alert")).toContainText("limit is 3");
  await snap(page, "four-level-1-refused");
  await expect(editor).toBeVisible(); // the editor stays open for a fix
  const none = await (await fetch(`${stack.apiUrl}/api/spells/slots`)).json();
  expect(none).toEqual([]); // nothing was saved

  await editor.getByRole("button", { name: "Remove Detect Magic" }).click();
  await editor.getByRole("button", { name: "Save preparation" }).click();
  await expect(editor).toHaveCount(0);
  const level1 = page.getByRole("region", { name: "Level 1" });
  await expectSlotDots(level1, 1, 3, 3);
  await expect(level1.getByTestId("dots-1")).toHaveAttribute(
    "data-state",
    "aaa",
  ); // 3 of 3 prepared: no empty dot
  await expect(level1).toContainText("Chromatic Orb");
  await snap(page, "three-level-1-accepted");
});

test("duplicates are prepared and a long rest keeps the preparation but resets used", async ({
  page,
  stack,
}) => {
  expect(
    (
      await put(stack.apiUrl, [
        "mirror_image",
        "mirror_image",
        "haste",
        "fireball",
      ])
    ).ok,
  ).toBe(true);
  await openSpells(page);

  const level2 = page.getByRole("region", { name: "Level 2" });
  await expectSlotDots(level2, 2, 2, 2);
  await level2
    .getByRole("button", { name: "Cast Mirror Image" })
    .first()
    .click();
  await expectSlotDots(level2, 2, 1, 2);
  await snap(page, "after-cast");

  await page.getByRole("button", { name: "Long rest" }).click();
  await expectSlotDots(level2, 2, 2, 2);
  await expect(level2.getByRole("listitem")).toHaveCount(1); // grouped: one row, with a count
  await expect(level2).toContainText("Mirror Image ×2");
  await expect(page.getByRole("region", { name: "Level 3" })).toContainText(
    "Haste",
  );
  await snap(page, "after-long-rest");
});

test("preparing is logged and voiding it restores the previous preparation", async ({
  page,
  stack,
}) => {
  expect((await put(stack.apiUrl, ["alarm"])).ok).toBe(true);
  await openSpells(page);
  const level1 = page.getByRole("region", { name: "Level 1" });
  await expect(level1).toContainText("Alarm");

  await page.getByRole("button", { name: "Prepare spells" }).click();
  await page
    .getByRole("form", { name: "Prepare today's spells" })
    .getByRole("button", { name: "Remove Alarm" })
    .click();
  await add(page, "Magic Missile", "Identify");
  await page.getByRole("button", { name: "Save preparation" }).click();
  await expect(level1).toContainText("Magic Missile");
  await expect(level1).not.toContainText("Alarm");

  const entry = logEntry(page, "Prepared: Magic Missile, Identify");
  await expect(entry).toBeVisible();
  await snap(page, "prepared-logged");
  await page.getByTestId("undo").click();

  await expect(entry).toHaveAttribute("data-status", "voided");
  await expect(level1).toContainText("Alarm");
  await expect(level1).not.toContainText("Magic Missile");
  await snap(page, "prepare-voided");
});

test("the spell names follow the language: the catalogue's French name, English without one", async ({
  page,
  stack,
}) => {
  expect(
    (await put(stack.apiUrl, ["mirror_image", "melfs_acid_arrow", "fireball"]))
      .ok,
  ).toBe(true);
  await page.addInitScript(() =>
    localStorage.setItem(
      "critical.settings.v1",
      JSON.stringify({ lang: "fr", mode: "system", amb: "arcane" }),
    ),
  );
  await openDashboard(page);
  await page.getByRole("tab", { name: "Sorts", exact: true }).click();
  await expect(page.getByText("Image miroir", { exact: true })).toBeVisible();
  await expect(page.getByText("Boule de feu", { exact: true })).toBeVisible();
  // The longest name in a narrow two-column row: shown whole, no overflow, Cast still visible.
  const acid = page
    .getByRole("region", { name: "Niveau 2" })
    .getByRole("listitem")
    .filter({ hasText: "Melf's Acid Arrow" });
  await expect(acid.getByRole("button", { name: "Lancer" })).toBeInViewport({
    ratio: 1,
  });
  for (const el of [acid, acid.getByText("Melf's Acid Arrow")]) {
    expect(await el.evaluate((e) => e.scrollWidth <= e.clientWidth)).toBe(true);
  }
  await snap(page, "french");

  // A buff from a spell is named from the catalogue too, in the timeline and in the journal.
  await page
    .getByRole("button", { name: "Lancer Image miroir" })
    .first()
    .click();
  const buff = page.locator('[data-testid="buff"][data-buff="Mirror Image"]');
  await expect(buff).toContainText("Image miroir");
  await expect(page.getByTestId("cast-buff")).toContainText("Image miroir");
  await snap(page, "french-buff");
  await buff.getByRole("button", { name: "Retirer Image miroir" }).click();
  await expect(buff).toHaveCount(0);
  const dismissed = combatLog(page)
    .getByRole("listitem")
    .filter({ hasText: "Effet retiré : Image miroir" });
  await expect(dismissed).toBeVisible();

  // Undo and redo restore the buff with its spell: still named from the catalogue.
  await page.getByTestId("undo").click();
  await expect(dismissed).toHaveAttribute("data-status", "voided");
  await expect(buff).toContainText("Image miroir");
  await page.getByTestId("redo").click();
  await expect(buff).toHaveCount(0);
  await expect(dismissed).toHaveAttribute("data-status", "active");
  await expect(dismissed).toContainText("Effet retiré : Image miroir");
  await page.getByTestId("undo").click();
  await expect(buff).toContainText("Image miroir");

  // Next turn expires the spell's buff and a hand-made one together: each named its own way.
  const { buffs } = (await (
    await fetch(`${stack.apiUrl}/api/buffs`)
  ).json()) as {
    buffs: { id: string; spellId?: string }[];
  };
  const mirror = buffs.find((b) => b.spellId === "mirror_image");
  expect(mirror).toBeDefined();
  for (const [url, method, body] of [
    [`/api/buffs/${mirror!.id}`, "PATCH", { duration: 1 }],
    ["/api/buffs", "POST", { name: "Rage", duration: 1, effects: {} }],
  ] as const) {
    const res = await fetch(`${stack.apiUrl}${url}`, {
      method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    expect(res.ok).toBe(true);
  }
  await page.reload();
  await page.getByRole("button", { name: "Tour suivant" }).click();
  await expect(page.getByRole("status")).toContainText(
    "Bonus expirés : Image miroir, Rage",
  );
  await expect(
    combatLog(page).getByText("— expirés : Image miroir, Rage"),
  ).toBeVisible();

  // Names are never truncated: a long name wraps in the timeline and on its charge card.
  const long = "Protection contre le mal sur trois mètres de rayon";
  expect(
    (
      await fetch(`${stack.apiUrl}/api/buffs`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: long, charges: 3, effects: {} }),
      })
    ).ok,
  ).toBe(true);
  await page.reload();
  const row = page.locator(`[data-testid="buff"][data-buff="${long}"]`);
  const card = page.locator(`[data-testid="charge-card"][data-buff="${long}"]`);
  for (const width of [1440, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    for (const el of [row, card]) {
      await expect(el.getByText(long, { exact: true })).toBeVisible();
      expect(
        await el.evaluate(
          (root) =>
            [root, ...root.querySelectorAll("*")].filter(
              (e) =>
                getComputedStyle(e).textOverflow === "ellipsis" ||
                (e.textContent?.includes("Protection") &&
                  e.scrollWidth > e.clientWidth),
            ).length,
        ),
      ).toBe(0);
    }
    await snap(page, `french-long-buff-${width}`);
  }
});

test("a prepared spell outside the spellbook is listed as removed and not sent", async ({
  page,
  stack,
}) => {
  expect((await put(stack.apiUrl, ["alarm"])).ok).toBe(true);
  // The stored preparation also holds a slot the spellbook no longer has (seed changed since).
  await page.route("**/api/spells/slots", async (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    const slots = (await (await route.fetch()).json()) as object[];
    await route.fulfill({
      json: [
        ...slots,
        {
          level: 3,
          prepared_spell_id: "shield",
          prepared_spell_name: "Shield",
          used: false,
        },
      ],
    });
  });
  let sent: string[] | undefined;
  page.on("request", (r) => {
    if (r.method() === "PUT" && r.url().endsWith("/api/spells/prepared")) {
      sent = (JSON.parse(r.postData() ?? "{}") as { spells: string[] }).spells;
    }
  });
  await openSpells(page);
  await page.getByRole("button", { name: "Prepare spells" }).click();
  const editor = page.getByRole("form", { name: "Prepare today's spells" });
  await expect(editor.getByTestId("stale-prepared")).toContainText(
    "will be removed",
  );
  await expect(
    editor.getByTestId("stale-prepared").getByRole("listitem"),
  ).toHaveCount(1);
  await expect(editor.getByTestId("draft-level-1")).toContainText("Alarm");
  await snap(page, "stale-listed");

  await editor.getByRole("button", { name: "Save preparation" }).click();
  await expect(editor).toHaveCount(0);
  expect(sent).toEqual(["alarm"]);
  await expect(page.getByRole("alert")).toHaveCount(0);
  await snap(page, "stale-saved");
});

// a spell prepared several times is one row with a count; rows are sorted by name.
test("a repeated spell shows once with a count that drops as copies are cast; rows are alphabetical", async ({
  page,
  stack,
}) => {
  expect(
    (await put(stack.apiUrl, ["magic_missile", "alarm", "magic_missile"])).ok,
  ).toBe(true);
  await openSpells(page);
  const level1 = page.getByRole("region", { name: "Level 1" });
  await expect(level1.getByRole("listitem")).toHaveCount(2);
  await expect(level1.getByRole("listitem").nth(0)).toContainText("Alarm");
  await expect(level1.getByRole("listitem").nth(1)).toContainText(
    "Magic Missile ×2",
  );
  await level1.getByRole("button", { name: "Cast Magic Missile" }).click();
  await expect(level1.getByRole("listitem").nth(1)).not.toContainText(
    "Magic Missile ×",
  );
  await expectSlotDots(level1, 1, 2, 3);
  await level1.getByRole("button", { name: "Cast Magic Missile" }).click();
  await expectSlotDots(level1, 1, 1, 3);
  await snap(page, "grouped-spells");
});
