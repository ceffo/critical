import { test, expect, snap } from "./fixtures";
import { openDashboard, chooseLanguage } from "./helpers";
import { expectNoClippedText } from "./noClip";

// The psionicist's creation (ADR 0043 steps 4.4 and 9), end to end in French: what only a browser
// proves is the psionics step appearing once the draft names the class, the powers picked in the
// shared editor reaching the stored character, and the layout of the step at the big viewports
// (the editor scrolls its own lists: one scrollbar, not two). The allowance, the PSP formula and
// the refusals are Go tests (psionics, charactersvc, the draft).

const scores = [
  ["FOR", "12"],
  ["DEX", "16"],
  ["CON", "14"],
  ["INT", "15"],
  ["SAG", "17"],
  ["CHA", "10"],
] as const;

test("create a thief/psionicist with Telepathy and Contact, the step at 1920 and 2560", async ({
  page,
}) => {
  test.setTimeout(120_000);
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  await page.setViewportSize({ width: 1920, height: 1080 });
  await page.goto("/create");
  const step = page.getByTestId("creation-step");
  const next = () =>
    step.getByRole("button", { name: "Passer à l'étape suivante" }).click();
  const steps = page.getByRole("navigation", { name: "Étapes" });

  // The single-class picker lists the psionicist like any class.
  await snap(page, "class-cards");
  await step
    .getByRole("radiogroup", { name: "Type de personnage" })
    .getByRole("radio", { name: "Multiclassé", exact: true })
    .click();
  const rows = step.getByTestId("class-rows");
  await rows
    .getByTestId("class-row-0")
    .getByLabel("Classe 1")
    .selectOption("thief");
  await rows.getByTestId("class-row-0").getByLabel("XP").fill("1250");
  await rows
    .getByTestId("class-row-1")
    .getByLabel("Classe 2")
    .selectOption("psionicist");
  await rows.getByTestId("class-row-1").getByLabel("XP").fill("1250");
  await expect(steps).toContainText("Psionique");
  await snap(page, "class-step");

  await next();
  await step.getByRole("radio", { name: "Humain", exact: true }).click();
  await next();
  await step.getByRole("radio", { name: "Saisir" }).click();
  for (const [ability, score] of scores)
    await step.getByLabel(ability, { exact: true }).fill(score);
  await next();
  await step.getByLabel("PV max").fill("20");
  await next();
  await expect(step).toHaveAttribute("data-step", "proficiencies");
  await next();
  await expect(step).toHaveAttribute("data-step", "psionics");

  const psionics = step.getByTestId("psionics-step");
  await expect(step.getByTestId("psp-computed")).toContainText("PFP");
  // A maximum of 0 is refused at its field, not taken as "compute it".
  const max = step.getByLabel("PFP maximum");
  await max.fill("0");
  await expect(step.getByTestId("field-error")).toContainText("refusé");
  await max.fill("");
  await expect(step.getByTestId("field-error")).toHaveCount(0);
  const panel = step.getByTestId("known-powers");
  await panel.getByRole("button", { name: /^Ajouter Télépathie/ }).click();
  await panel.getByRole("searchbox").fill("contact");
  await panel
    .getByTestId("pick-row")
    .first()
    .getByRole("button", { name: /^Ajouter/ })
    .click();
  await expect(panel.getByTestId("known-power")).toHaveCount(1);

  for (const vp of [
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    await expectNoClippedText(psionics, `psionics step ${vp.width}`);
    // One scrollbar: the step's scroller does not scroll when the editor has its own lists.
    // Polled: right after a pick the editor's list re-renders, and a slow runner can catch it mid-layout.
    await expect
      .poll(
        () =>
          step.evaluate((el) => {
            const scrollers = [el, ...el.querySelectorAll("*")].filter(
              (n) =>
                n instanceof HTMLElement &&
                n.scrollHeight > n.clientHeight + 1 &&
                ["auto", "scroll"].includes(getComputedStyle(n).overflowY),
            );
            return scrollers.length;
          }),
        { message: `scrolling boxes in the step at ${vp.width}` },
      )
      .toBe(0);
    await snap(page, `psionics-step-${vp.width}x${vp.height}`);
  }

  await next();
  await step.getByLabel("Nom", { exact: true }).fill("Aria");
  await step
    .getByTestId("alignment-grid")
    .getByRole("radio", { name: /^[^:]+$/ })
    .first()
    .click();
  await step
    .getByRole("radiogroup", { name: "Sexe" })
    .getByRole("radio")
    .first()
    .click();
  await next();
  await snap(page, "review");
  await step
    .getByRole("button", { name: "Créer ce personnage et le jouer" })
    .click();
  const sheet = page.getByTestId("character");
  await expect(sheet).toContainText("Aria");
  await expect(sheet).toContainText("Psioniste");
  await expect(sheet).toContainText("Voleur");
  await snap(page, "sheet");
  await page
    .getByTestId("rail")
    .getByRole("button", { name: "Pouvoirs" })
    .click();
  await expect(
    page.getByTestId("known-power").filter({ hasText: "Contact" }),
  ).toBeVisible();
});
