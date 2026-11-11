import { test, expect, snap } from "./fixtures";
import { psionicist } from "./psionicist";
import {
  apiCall,
  chooseLanguage,
  LOOKUP_RAIL,
  openDashboard,
  reloadSettled,
} from "./helpers";

// ADR 0022 steps 2.4 and 2.5: the Act tabs and the rail's lookup name follow what the character has.
// The rule (slots, known powers, per-day items) is the unit test actFeatures; this proves the wiring
// on what the server serves, per class, in both languages, and looks at the layout sizes.
const SIZES = [
  { width: 1280, height: 800 },
  { width: 1920, height: 1080 },
  { width: 2560, height: 1440 },
] as const;

const NAMES = {
  mage: { en: "Spells", fr: "Sorts" },
  fighter: { en: "Reference", fr: "Référence" },
  psionicist: { en: "Powers", fr: "Pouvoirs" },
} as const;

async function check(
  page: import("@playwright/test").Page,
  cls: keyof typeof NAMES,
  spells: boolean,
) {
  for (const lang of ["en", "fr"] as const) {
    await chooseLanguage(page, lang);
    const rail = page.getByTestId("rail");
    await expect(rail.getByRole("button", { name: LOOKUP_RAIL })).toHaveText(
      NAMES[cls][lang],
    );
    await expect(page.getByTestId("tab-count-spells")).toHaveCount(
      spells ? 1 : 0,
    );
    for (const size of SIZES) {
      await page.setViewportSize(size);
      await snap(page, `${cls}-${lang}-${size.width}`);
    }
  }
}

for (const cls of ["mage", "fighter"] as const) {
  test(`${cls}: the Spells tab and the rail name follow the class`, async ({
    page,
  }) => {
    await openDashboard(page);
    expect(
      await apiCall(page, "POST", "/api/characters", {
        name: `Test ${cls}`,
        class: cls,
        raceCode: "human",
        details: { alignment: "neutral_good", sex: "male" },
        str: 12,
        dex: 12,
        con: 12,
        int: 16,
        wis: 12,
        cha: 10,
        hpMax: 8,
        xp: 0,
      }),
    ).toBe(true);
    await reloadSettled(page);
    await check(page, cls, cls === "mage");
  });
}

test.describe("a psionicist", () => {
  test.use({ character: psionicist });
  test("has Powers; the Spells tab stays only for the per-day item Long rest refills", async ({
    page,
  }) => {
    await openDashboard(page);
    expect(
      await apiCall(page, "PUT", "/api/psionics/picks", {
        picks: [
          { kind: "discipline", id: "telepathy" },
          { kind: "power", id: "thought_shield" },
        ],
      }),
    ).toBe(true);
    await reloadSettled(page);
    await expect(page.getByRole("tab", { name: "Powers" })).toBeVisible();
    // The fixture character holds a per-day item (the Power Gem): Long rest lives in the Spells
    // tab, so the tab stays.
    await expect(page.getByTestId("tab-count-spells")).toHaveCount(1);
    expect(
      await apiCall(page, "DELETE", "/api/items/power_gem_dispel", undefined),
    ).toBe(true);
    await reloadSettled(page);
    await check(page, "psionicist", false);
  });
});
