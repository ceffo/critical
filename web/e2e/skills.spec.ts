import { test, expect, snap } from "./fixtures";
import { chooseLanguage, combatLog, openDashboard, openFold } from "./helpers";

const VIEWPORTS = [
  { width: 1280, height: 800 },
  { width: 1440, height: 900 },
  { width: 1920, height: 1080 },
  { width: 2560, height: 1440 },
];

// What only a browser proves: the Skills tab's rows are buttons wired to the server's check (no
// pre-roll adjustment in the tab, ADR 0027 step 12), the result card shows the check and adjusts the
// latest one after the roll, the journal words both, and Undo pops a check. The rules (d20 / d100,
// the natural 20, targets, ranges) are Go tests (skillsvc, composition), the wording unit tests
// (skillAdjust, the journal registry); the outcome of a roll is random, so nothing assumes success or failure.
test("skills: roll a proficiency, adjust it on the card, roll a bard skill, undo pops", async ({
  page,
}) => {
  await openDashboard(page);
  await page.getByRole("tab", { name: "Skills" }).click();
  const panel = page.getByRole("tabpanel", { name: "Skills" });
  const card = page.getByRole("region", { name: "Detail", exact: true });
  await expect(panel.getByRole("group")).toHaveCount(0);

  // Proficiency: Musical instrument, DEX 17 with its −1 as a pill; the target 16 is the card's.
  const instrument = panel.getByTestId("skill-musical_instrument");
  await expect(instrument).toContainText("DEX 17 −1");
  await expect(instrument.getByTestId("skill-modifier")).toHaveText("−1");
  await instrument.click();
  const result = card.getByTestId("skill-result");
  await expect(card.getByTestId("card-header")).toContainText(
    "Musical instrument",
  );
  await expect(result.getByTestId("skill-vs")).toHaveText(/^\d+ vs 16$/);
  const d20 = Number((await result.getByTestId("hex-die").textContent()) ?? "");
  expect(d20).toBeGreaterThanOrEqual(1);
  expect(d20).toBeLessThanOrEqual(20);
  await expect(result.getByTestId("verdict")).toHaveText(/^(Success|Failure)$/);
  await expect(combatLog(page)).toContainText(
    `Musical instrument: d20 ${d20} vs 16`,
  );

  // After the roll, on the latest check: +3 with a reason; the target moves, the roll stays.
  const form = await openFold(
    card,
    "Adjust after the roll",
    "Adjust this check",
  );
  for (let i = 0; i < 3; i++)
    await form.getByRole("button", { name: "Increase adjustment" }).click();
  await form.getByLabel("Reason (optional)").fill("lit by torches");
  await form.getByRole("button", { name: "Apply" }).click();
  await expect(result.getByTestId("skill-vs")).toHaveText(
    new RegExp(`^${d20} vs 19$`),
  );
  await expect(result.getByTestId("skill-adjusted-target")).toHaveText("19");
  await expect(result.getByTestId("skill-adjustment-line")).toContainText(
    "lit by torches",
  );
  await expect(combatLog(page)).toContainText(
    `Musical instrument: d20 ${d20} vs 19 (16 +3 lit by torches)`,
  );
  await snap(page, "skills-adjust-card");

  // Bard skill: Climb walls, 85%; a new check replaces the card.
  const climb = panel.getByTestId("skill-climb_walls");
  await expect(climb).toContainText("85%");
  await climb.click();
  await expect(card.getByTestId("card-header")).toContainText("Climb walls");
  await expect(result.getByTestId("skill-vs")).toHaveText(/^\d+ vs 85%$/);
  const d100 = Number(
    (await result.getByTestId("hex-die").textContent()) ?? "",
  );
  expect(d100).toBeGreaterThanOrEqual(1);
  expect(d100).toBeLessThanOrEqual(100);
  await expect(combatLog(page)).toContainText(
    `Climb walls: d100 ${d100} vs 85`,
  );

  for (const vp of VIEWPORTS) {
    await page.setViewportSize(vp);
    await expect(climb).toBeVisible();
    const scroll = await page.evaluate(() => {
      const el = document.scrollingElement ?? document.documentElement;
      return {
        x: el.scrollWidth - el.clientWidth,
        y: el.scrollHeight - el.clientHeight,
      };
    });
    expect(scroll, "no page scroll").toEqual({ x: 0, y: 0 });
    await snap(page, `skills-${vp.width}x${vp.height}`);
  }
  await page.setViewportSize({ width: 1440, height: 900 });

  // Undo pops the bard skill check: its journal row is cancelled, the proficiency's is not.
  const undo = page.getByTestId("undo");
  await expect(undo).toHaveAttribute("title", /Climb walls/);
  await undo.click();
  const voided = combatLog(page).locator('[data-status="voided"]');
  await expect(voided).toHaveCount(1);
  await expect(voided).toContainText(`Climb walls: d100 ${d100}`);

  // Another skill after the undo; the journal's Rolls filter keeps the skill checks.
  const noise = panel.getByTestId("skill-detect_noise");
  await noise.click();
  await expect(card.getByTestId("card-header")).toContainText("Detect noise");
  const heard = (await result.getByTestId("hex-die").textContent()) ?? "";
  const journal = combatLog(page);
  await journal.getByRole("radio", { name: "Rolls" }).click();
  await expect(journal).toContainText(`Detect noise: d100 ${heard} vs 90`);
  await expect(journal).toContainText(`Musical instrument: d20 ${d20} vs 19`);

  // French, dark: names, "DEX 17 −1", the card's "contre" and verdict, for the eye.
  await page.emulateMedia({ colorScheme: "dark" });
  await chooseLanguage(page, "fr");
  await page.getByRole("tab", { name: "Compétences" }).click();
  const instrumentFrench = page.getByTestId("skill-musical_instrument");
  await expect(
    page.getByTestId("proficiency-name-musical_instrument"),
  ).toContainText("Instrument de musique");
  await expect(instrumentFrench).toContainText("DEX 17 −1");
  const resultFrench = page.getByTestId("skill-result");
  await expect(resultFrench.getByTestId("skill-vs")).toHaveText(
    /^\d+ contre 90\u00a0%$/,
  );
  await expect(resultFrench.getByTestId("verdict")).toHaveText(
    /^(Réussi|Échec)$/,
  );
  await snap(page, "skills-fr-dark-1440x900");
});

// What only a browser proves (ADR 0028): the character card's DEX tile is the check's roll button
// (click, Enter and Space), the result card shows the check, its adjustment after the roll re-judges
// the same roll, the journal words it and Undo pops the adjustment. The d20 is random: the tile is
// rolled until the die lands where −2 decides against DEX 17 (16 or 17). The rules (target in
// effect, natural 20, ranges) are Go tests (skillsvc), the wording unit tests (web/src/journal).
test("ability check: click DEX, adjust −2 on the card, the verdict changes, undo restores it", async ({
  page,
}) => {
  await openDashboard(page);
  const dex = page.getByRole("button", { name: "Dexterity check (17)" });
  const card = page.getByRole("region", { name: "Detail", exact: true });
  const result = card.getByTestId("ability-result");
  // One roll per input: each must send its own POST (the keyboard is not left to the dice).
  const roll = async (input: () => Promise<void>) => {
    const rolled = page.waitForResponse(
      (r) =>
        r.url().endsWith("/api/roll/ability/dex") &&
        r.request().method() === "POST",
    );
    await input();
    const res = await rolled;
    expect(res.ok()).toBe(true);
    const n = ((await res.json()) as { roll: number }).roll;
    await expect(result.getByTestId("hex-die")).toHaveText(String(n));
    return n;
  };
  for (const key of ["Enter", "Space"])
    await roll(async () => {
      await dex.focus();
      await page.keyboard.press(key);
    });
  let d20 = 0;
  for (let i = 0; i < 200 && d20 !== 16 && d20 !== 17; i++)
    d20 = await roll(() => dex.click());
  expect([16, 17]).toContain(d20);
  await expect(card.getByTestId("card-header")).toContainText("Dexterity");
  await expect(result.getByTestId("skill-vs")).toHaveText(`${d20} vs 17`);
  await expect(result.getByTestId("verdict")).toHaveText("Success");

  const form = await openFold(
    card,
    "Adjust after the roll",
    "Adjust this check",
  );
  const harder = form.getByRole("button", { name: "Decrease adjustment" });
  await harder.click();
  await harder.click();
  await form.getByRole("button", { name: "Apply" }).click();
  await expect(result.getByTestId("verdict")).toHaveText("Failure");
  await expect(result.getByTestId("skill-vs")).toHaveText(`${d20} vs 15`);
  await expect(result.getByTestId("hex-die")).toHaveText(String(d20));
  await expect(combatLog(page)).toContainText(
    `DEX check: d20 ${d20} vs 15 (17 −2), failure`,
  );
  await snap(page, "ability-check-adjusted");

  // Undo pops the adjustment, its own step: the check reads as rolled.
  await page.getByTestId("undo").click();
  await expect(result.getByTestId("verdict")).toHaveText("Success");
  await expect(result.getByTestId("skill-vs")).toHaveText(`${d20} vs 17`);
});
