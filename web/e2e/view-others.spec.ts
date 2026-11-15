import { test, expect, snap } from "./fixtures";
import {
  chooseLanguage,
  newCharacter,
  apiSettled,
  openDashboard,
  openRoster,
  seedOtherUser,
} from "./helpers";

// Browsing another user's character, read-only (ADR 0029 step 11): the one flow only a browser proves.
// Two users: B owns a mage; A opens the roster, finds it under B, opens it, sees the banner and the
// locked controls, moves around the sheet, and returns to their own character. No write request ever
// leaves the page and A's current character is unchanged. The grouping, the wording of the owner
// and the lock itself are unit and component tests (lib/others.test.ts, lib/readOnly.test.ts,
// components/characters/Others.dom.test.tsx); the banner's layout is layout.spec.ts.
test("open another user's character read-only, move around, return to your own", async ({
  page,
  stack,
}) => {
  await seedOtherUser(stack.apiUrl, "bruno@example.com", [
    newCharacter("Brunhilde Voss", "mage"),
  ]);
  const writes: string[] = [];
  page.on("request", (r) => {
    if (!["GET", "HEAD"].includes(r.method()))
      writes.push(`${r.method()} ${r.url()}`);
  });

  await openDashboard(page);
  await chooseLanguage(page, "fr");
  const roster = await openRoster(page);
  const others = roster.getByTestId("roster-others");
  await expect(others).toContainText("Joueur sans nom");
  const card = others.getByTestId("other-card");
  await expect(card).toHaveCount(1);
  await expect(card).toContainText("Brunhilde Voss");
  await expect(card).toContainText("Magicien · niveau 1");
  await snap(page, "roster-others-fr");
  await card.getByRole("button", { name: "Consulter Brunhilde Voss" }).click();

  await expect(page).toHaveURL(/\/view\/\d+$/);
  const banner = page.getByTestId("view-banner");
  await expect(banner).toContainText("Brunhilde Voss");
  await expect(banner).toContainText("un autre joueur");
  await expect(banner).toContainText("Lecture seule");
  await expect(page.getByTestId("character")).toContainText("Brunhilde Voss");
  await apiSettled(page);
  await snap(page, "view-act-fr");

  // Every write control is locked (HP change, the round, the effects); moving around still works.
  const character = page.getByTestId("character");
  await expect(
    character.getByRole("button", { name: "Dégâts" }),
  ).toBeDisabled();
  await expect(character.getByRole("button", { name: "Soins" })).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Tour suivant" }),
  ).toBeDisabled();
  await expect(page.getByRole("button", { name: "Ajouter" })).toBeDisabled();
  await expect(page.getByTestId("settings-trigger")).toBeEnabled();
  await page.getByRole("tab", { name: "Compétences" }).click();
  await expect(page.getByRole("tab", { name: "Compétences" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await page
    .getByTestId("rail")
    .getByRole("button", { name: "Inventaire" })
    .click();
  await expect(page.locator("[data-activity=inventory]")).toBeVisible();
  await snap(page, "view-inventory-fr");
  await page.getByTestId("rail").getByRole("button", { name: "Agir" }).click();

  // Back: a full load on the user's own character, whose current character never changed.
  await banner.getByTestId("view-back").click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByTestId("character")).toContainText("Test Bard");
  expect(writes).toEqual([]);
});
