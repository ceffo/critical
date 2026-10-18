import { mkdir, rm } from "node:fs/promises";
import path from "node:path";
import { test, expect } from "../e2e/fixtures";
import { chooseSetting, chooseTheme } from "../e2e/helpers";
import { THEMES, themeVars, type Theme } from "../src/lib/theme";
import { CANDIDATES, CANDIDATE_MODES } from "./candidate-themes";
import { annotate, audit, seedMockTable } from "./paint";
import { VIEWPORTS, focus, translator, webDir } from "./table";

/**
 * The new-themes round (critical-4jaw, docs/design/new-themes/README.md): the candidate themes of
 * candidate-themes.ts painted over the real app (the mock-up round's seeded table, French, Act ·
 * Weapons) at both layout viewports, through the app's own themeVars. Every shot goes through the
 * no-cut audit. Output: docs/design/new-themes/shots/ (PNG). Run with `just ui-themes`.
 */

const shots = path.join(webDir, "..", "docs", "design", "new-themes", "shots");

// themeVars looks a theme up in THEMES: add the candidates (this process only).
(THEMES as unknown as Theme[]).push(...CANDIDATES);

test("new themes: each candidate in each of its modes, in French, at both viewports", async ({
  page,
  stack,
}) => {
  test.setTimeout(10 * 60_000);
  await rm(shots, { recursive: true, force: true });
  await mkdir(shots, { recursive: true });
  await seedMockTable(page, stack.apiUrl, await translator("en"));
  await chooseSetting(page, "Français");
  const fr = await translator("fr");
  const problems: string[] = [];

  for (const { id, mode } of CANDIDATE_MODES) {
    // The app's Arcane in that mode sets the mode; the candidate's variables replace Arcane's.
    await chooseSetting(page, fr(`settings.mode.${mode}`));
    await chooseTheme(page, fr("settings.themes.arcane"));
    await focus(page, fr, "act");
    await page.getByRole("tab", { name: fr("actions.tabs.weapons") }).click();
    await page.evaluate(
      ([theme, vars]) => {
        const root = document.documentElement;
        root.dataset.theme = theme;
        for (const [k, v] of Object.entries(vars)) root.style.setProperty(k, v);
      },
      [id, themeVars(id, mode === "light")] as const,
    );
    await annotate(page);
    for (const { width, height } of VIEWPORTS) {
      await page.setViewportSize({ width, height });
      await page.waitForTimeout(200);
      const name = `${id}-${mode}-${width}x${height}.png`;
      await page.screenshot({
        path: path.join(shots, name),
        animations: "disabled",
      });
      for (const c of await audit(page)) problems.push(`${name}: ${c}`);
    }
  }
  expect(problems, "cut text or page scroll").toEqual([]);
});
