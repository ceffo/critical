import type { Locator } from "@playwright/test";
import { test, expect } from "./fixtures";
import { chooseLanguage, openDashboard, openSpend } from "./helpers";

// ADR 0037 step 22.4, what only a browser proves: the picks the class grants free move in and out
// of the offered group as the player edits them. The costs, the reminder and the pools are Go
// tests; the picker's wording a unit test (ProficiencyPicker.test.tsx).

const free = [
  "reading_writing",
  "local_history",
  "singing",
  "musical_instrument",
];

/** The check target the pick's terms read ("target CHA 16"). */
async function target(pick: Locator): Promise<number> {
  const terms = await pick.getByTestId("proficiency-pick-terms").textContent();
  const m = /target \D*(\d+)/.exec(terms ?? "");
  expect(m, terms ?? "").not.toBeNull();
  return Number(m![1]);
}

async function chooseClass(step: Locator, name: string) {
  await step.getByRole("radio", { name, exact: true }).click();
}

test("creation: the bard's four start at 0 once; a removed one is offered, not put back; a class change seeds again", async ({
  page,
}) => {
  page.on("dialog", (d) => void d.accept());
  await openDashboard(page);
  await chooseLanguage(page, "en");
  await page.goto("/create");
  const step = page.getByTestId("creation-step");
  const next = step.getByRole("button", { name: "Go to the next step" });
  const back = page.getByRole("button", {
    name: "Back to the previous step, the values stay",
  });
  await chooseClass(step, "Bard");
  await next.click();
  await step.getByRole("radio", { name: "Half-elf", exact: true }).click();
  await next.click();
  await step.getByRole("radio", { name: "Type" }).click();
  for (const [label, score] of [
    ["STR", "10"],
    ["DEX", "15"],
    ["CON", "12"],
    ["INT", "14"],
    ["WIS", "9"],
    ["CHA", "16"],
  ])
    await step.getByLabel(label, { exact: true }).fill(score);
  await next.click();
  await step.getByLabel("Max HP").fill("6");
  await next.click();
  await expect(step).toHaveAttribute("data-step", "proficiencies");

  const picks = step.getByTestId("proficiency-picks");
  const offered = step.getByTestId("proficiency-offered");
  for (const id of free) {
    const pick = picks.getByTestId(`proficiency-pick-${id}`);
    await expect(pick.getByTestId("proficiency-slots")).toHaveText("0");
    await expect(pick.getByTestId("proficiency-pick-terms")).toContainText(
      "free (class)",
    );
  }
  await expect(picks.getByTestId("proficiency-pick-terms")).toHaveCount(4);

  // Removed, Singing is offered at no cost and not put back, also after leaving the step.
  await picks
    .getByTestId("proficiency-pick-singing")
    .getByTestId("proficiency-remove")
    .click();
  await expect(picks.getByTestId("proficiency-pick-singing")).toHaveCount(0);
  await expect(offered.getByTestId("proficiency-add-singing")).toBeVisible();
  await back.click();
  await next.click();
  await expect(step).toHaveAttribute("data-step", "proficiencies");
  await expect(picks.getByTestId("proficiency-pick-singing")).toHaveCount(0);
  await expect(offered.getByTestId("proficiency-add-singing")).toBeVisible();

  // A thief is granted none: nothing offered. Back to bard: the four are seeded again.
  for (let i = 0; i < 4; i++) await back.click();
  await expect(step).toHaveAttribute("data-step", "class");
  await chooseClass(step, "Thief");
  for (let i = 0; i < 4; i++) await next.click();
  await expect(step).toHaveAttribute("data-step", "proficiencies");
  await expect(offered).toHaveCount(0);
  for (const id of free)
    await expect(picks.getByTestId(`proficiency-pick-${id}`)).toHaveCount(0);
  for (let i = 0; i < 4; i++) await back.click();
  await chooseClass(step, "Bard");
  for (let i = 0; i < 4; i++) await next.click();
  await expect(step).toHaveAttribute("data-step", "proficiencies");
  for (const id of free)
    await expect(
      picks
        .getByTestId(`proficiency-pick-${id}`)
        .getByTestId("proficiency-slots"),
    ).toHaveText("0");
});

test("editor: a free pick raised to 2 slots is +2 on target; removed, it is offered; re-added at 0", async ({
  page,
}) => {
  await openDashboard(page);
  const editor = await openSpend(page, "nonWeapon");
  const spent = editor.getByTestId("proficiency-pool-spent");
  await expect(spent).toHaveText("Spent 4, left 6");
  const singing = editor.getByTestId("proficiency-pick-singing");
  await expect(singing.getByTestId("proficiency-slots")).toHaveText("0");
  const base = await target(singing);

  await singing.getByTestId("proficiency-more").click();
  await singing.getByTestId("proficiency-more").click();
  await expect(singing.getByTestId("proficiency-slots")).toHaveText("2");
  await expect(spent).toHaveText("Spent 6, left 4");
  await expect
    .poll(() => target(singing), { message: "two extra slots" })
    .toBe(base + 2);

  await singing.getByTestId("proficiency-remove").click();
  await expect(singing).toHaveCount(0);
  await expect(spent).toHaveText("Spent 4, left 6");
  const offer = editor
    .getByTestId("proficiency-offered")
    .getByTestId("proficiency-add-singing");
  await expect(offer).toBeVisible();
  await offer.click();
  await expect(singing.getByTestId("proficiency-slots")).toHaveText("0");
  await expect(singing.getByTestId("proficiency-pick-terms")).toContainText(
    "free (class)",
  );
  await expect(spent).toHaveText("Spent 4, left 6");
  await expect(editor.getByTestId("proficiency-offered")).toHaveCount(0);
});
