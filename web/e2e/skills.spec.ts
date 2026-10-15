import { test, expect, snap } from "./fixtures";
import { combatLog, openDashboard } from "./helpers";

const VIEWPORTS = [
  { width: 1280, height: 800 },
  { width: 1440, height: 900 },
  { width: 1920, height: 1080 },
  { width: 2560, height: 1440 },
];

// What only a browser proves: the Skills tab's rows are buttons wired to the server's check, the
// detail card and the journal render the skill_check entry, the row's last check follows the
// log, and Undo pops a check. The rules (d20 / d100, the natural 20, targets) are Go tests
// (skillsvc, composition); the outcome of a roll is random, so nothing assumes success or failure.
test("skills: roll a proficiency and a bard skill, the card and journal show them, undo pops", async ({ page }) => {
  await openDashboard(page);
  await page.getByRole("tab", { name: "Skills" }).click();
  const panel = page.getByRole("tabpanel", { name: "Skills" });
  const card = page.getByRole("region", { name: "Detail", exact: true });

  // Proficiency: Musical instrument, DEX 17 −1, target 16 (computed by the server).
  const instrument = panel.getByTestId("skill-musical_instrument");
  await expect(instrument).toContainText("DEX 17 −1");
  await expect(instrument).toContainText("16");
  await expect(instrument.getByTestId("skill-last")).toHaveText("–");
  await instrument.click();
  const result = card.getByTestId("skill-result");
  await expect(result).toBeVisible();
  await expect(card.getByTestId("card-header")).toContainText("Musical instrument");
  await expect(result.getByTestId("skill-vs")).toHaveText(/^\d+ vs 16$/);
  const d20 = Number((await result.getByTestId("hex-die").textContent()) ?? "");
  expect(d20).toBeGreaterThanOrEqual(1);
  expect(d20).toBeLessThanOrEqual(20);
  await expect(result.getByTestId("verdict")).toHaveText(/^(Success|Failure)$/);
  await expect(instrument.getByTestId("skill-last")).toHaveText(String(d20));
  await expect(combatLog(page)).toContainText(`Musical instrument: d20 ${d20} vs 16`);

  // Bard skill: Climb walls, 85%.
  const climb = panel.getByTestId("skill-climb_walls");
  await expect(climb).toContainText("85%");
  await climb.click();
  await expect(card.getByTestId("card-header")).toContainText("Climb walls");
  await expect(result.getByTestId("skill-vs")).toHaveText(/^\d+ vs 85%$/);
  const d100 = Number((await result.getByTestId("hex-die").textContent()) ?? "");
  expect(d100).toBeGreaterThanOrEqual(1);
  expect(d100).toBeLessThanOrEqual(100);
  const verdict = (await result.getByTestId("verdict").textContent()) ?? "";
  await expect(climb.getByTestId("skill-last")).toHaveText(String(d100));
  await expect(climb.getByTestId("skill-last")).toHaveAttribute("data-success", String(verdict === "Success"));
  await expect(combatLog(page)).toContainText(`Climb walls: d100 ${d100} vs 85`);

  for (const vp of VIEWPORTS) {
    await page.setViewportSize(vp);
    await expect(climb).toBeVisible();
    const scroll = await page.evaluate(() => {
      const el = document.scrollingElement ?? document.documentElement;
      return { x: el.scrollWidth - el.clientWidth, y: el.scrollHeight - el.clientHeight };
    });
    expect(scroll, "no page scroll").toEqual({ x: 0, y: 0 });
    await snap(page, `skills-${vp.width}x${vp.height}`);
  }
  await page.setViewportSize({ width: 1440, height: 900 });

  // Undo pops the bard skill check: its row has no last check again, the proficiency keeps its own.
  const undo = page.getByTestId("undo");
  await expect(undo).toHaveAttribute("title", /Climb walls/);
  await undo.click();
  await expect(climb.getByTestId("skill-last")).toHaveText("–");
  await expect(instrument.getByTestId("skill-last")).toHaveText(String(d20));
});
