import { describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import i18next from "i18next";
import { I18nextProvider } from "react-i18next";
import { resources } from "@/i18n";
import type { RogueSkill, SkillSet, Skills } from "@/types/skills";
import { SkillsTab } from "./SkillsTab";

// Server rendering reads a zustand store's initial state, so the skills store is replaced by one
// whose answer the test sets.
let current: Skills | null = null;
vi.mock("@/stores/skills", () => ({
  useSkillsStore: (select: (s: { skills: Skills | null }) => unknown) =>
    select({ skills: current }),
}));

const rogueSkill: RogueSkill = {
  id: "hide_in_shadows",
  base: 10,
  points: 5,
  race: 0,
  dexterity: 0,
  armor: 0,
  percent: 15,
  rawPercent: 15,
  maxPercent: 95,
  maxPoints: 40,
  usable: true,
  last: null,
};

const base: Skills = {
  proficiencies: [
    {
      id: "swimming",
      ability: "str",
      abilityScore: 12,
      modifier: 0,
      target: 12,
      last: null,
    },
  ],
  skillSets: [],
};

const NAMES: Record<string, SkillSet["name"]> = {
  thief: { en: "Thief skills", fr: "Talents de voleur" },
  bard: { en: "Bard skills", fr: "Talents de barde" },
  // A set this client has never heard of: only the served name and code can show it.
  scout: { en: "Scout talents" },
};

/** A skill set of the answer, as the server serves it: its code, its name by language. */
function skillSet(
  set: string,
  skills: RogueSkill[],
  over: Partial<SkillSet> = {},
): SkillSet {
  return {
    set,
    name: NAMES[set] ?? {},
    skillArmor: "",
    budget: null,
    skills,
    ...over,
  };
}

async function render(skills: Skills) {
  current = skills;
  const inst = i18next.createInstance();
  await inst.init({
    lng: "en",
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  return renderToStaticMarkup(
    <I18nextProvider i18n={inst}>
      <SkillsTab />
    </I18nextProvider>,
  );
}

// The class skills section follows what the server sends (ADR 0030 steps 17 and 18).
describe("SkillsTab class skills section", () => {
  it("shows only the proficiencies when the server sends no class skills", async () => {
    const html = await render(base);
    expect(html).toContain("Proficiencies");
    expect(html).not.toContain("Bard skills");
    expect(html).not.toContain("Thief skills");
    expect(html).not.toContain("thief-points-edit");
    expect(html).not.toContain("proficiency-edit");
    expect(html).not.toContain("skill-armor");
  });

  it("shows the bard skills for a bard", async () => {
    const html = await render({
      ...base,
      skillSets: [skillSet("bard", [rogueSkill])],
    });
    expect(html).toContain("Bard skills");
    expect(html).not.toContain("Thief skills");
    expect(html).not.toContain("thief-points-edit");
  });

  it("shows the thief skills for a thief", async () => {
    const html = await render({
      ...base,
      skillSets: [skillSet("thief", [rogueSkill], { skillArmor: "leather" })],
    });
    expect(html).toContain("Thief skills");
    expect(html).not.toContain("Bard skills");
    expect(html).toContain("skill-armor");
  });
});

// ADR 0040 step 15: a section is named by what the server served and keyed by the set's code, so a
// set the client has never heard of renders like the two of the books.
describe("SkillsTab sections follow the served sets", () => {
  it("renders two sections named by the server, one of them an unknown set", async () => {
    const html = await render({
      ...base,
      skillSets: [
        skillSet("thief", [rogueSkill]),
        skillSet("scout", [{ ...rogueSkill, percent: 33 }]),
      ],
    });
    expect(html).toContain("Thief skills");
    expect(html).toContain("Scout talents");
    expect(html).toContain('data-testid="skill-set-scout"');
    expect(html).toContain('data-testid="scout-percent-hide_in_shadows"');
    expect(html).toContain('data-testid="thief-percent-hide_in_shadows"');
    expect(html).toContain('id="skills-scout"');
  });

  it("falls back to the set's code for a name in no language", async () => {
    const html = await render({
      ...base,
      skillSets: [skillSet("scout", [rogueSkill], { name: {} })],
    });
    expect(html).toContain(">scout</span>");
  });
});

// ADR 0040 step 3.5: a character whose classes grant two skill sets gets two sections, each with
// its own checks and no spending, rendered from the list (no class name in the client).
describe("SkillsTab with two skill sets", () => {
  it("shows each set with its own checks and no spending", async () => {
    const budget = (available: number, spent: number) => ({
      level: 3,
      first: 20,
      perLevel: 15,
      available,
      spent,
      left: available - spent,
      perSkillFirst: 0,
      perSkillPerLevel: 0,
      perSkillCap: 0,
    });
    const html = await render({
      ...base,
      skillSets: [
        skillSet("thief", [rogueSkill], { budget: budget(150, 10) }),
        skillSet("bard", [{ ...rogueSkill, percent: 7 }], {
          budget: budget(50, 20),
        }),
      ],
    });
    expect(html).toContain("Thief skills");
    expect(html).toContain("Bard skills");
    expect(html).toContain('data-testid="skill-set-thief"');
    expect(html).toContain('data-testid="skill-set-bard"');
    // Spending lives in the character card's dialog, never in the tab.
    expect(html).not.toContain("left 140");
    expect(html).not.toContain("thief-points-edit");
  });
});

// ADR 0030 steps 17 and 18: each skill's terms are always text.
describe("SkillsTab budget and terms", () => {
  it("writes each skill's terms, and says when the cap or the floor applied", async () => {
    const html = await render({
      ...base,
      skillSets: [
        skillSet("thief", [
          { ...rogueSkill, race: 5, dexterity: -5, armor: 0 },
          { ...rogueSkill, id: "climb_walls", rawPercent: 112, percent: 95 },
          { ...rogueSkill, id: "read_languages", rawPercent: -5, percent: 0 },
        ]),
      ],
    });
    // One column per term, as a label over its figure; the total is the roll button's.
    const terms = /data-testid="skill-terms-hide_in_shadows">(.*?)<\/dl>/.exec(
      html,
    )?.[1];
    expect(terms).toBeDefined();
    expect(
      terms
        ?.match(/<dt[^>]*>([^<]*)<\/dt>/g)
        ?.map((h) => h.replace(/<[^>]*>/g, "")),
    ).toEqual(["Base", "Points", "Race", "Dexterity", "Armour"]);
    expect(
      terms
        ?.match(/<dd[^>]*>([^<]*)<\/dd>/g)
        ?.map((h) => h.replace(/<[^>]*>/g, "")),
    ).toEqual(["10", "5", "+5", "−5", "0"]);
    expect(html).toMatch(/skill-limit-climb_walls">capped at 95%/);
    expect(html).toMatch(/skill-limit-read_languages">floored at 0%/);
    expect(html).not.toContain('data-testid="skill-limit-hide_in_shadows"');
  });
});

// A proficiency row is two sibling targets, never one inside the other: the name opens the
// description, the ability part rolls the check.
describe("SkillsTab proficiency row", () => {
  it("has a name button and a roll button side by side", async () => {
    const html = await render({
      ...base,
      skillSets: [skillSet("bard", [rogueSkill])],
    });
    const row = /data-testid="skill-row-swimming">(.*?)<\/li>/.exec(html)?.[1];
    expect(row).toBeDefined();
    const name =
      /<button[^>]*data-testid="proficiency-name-swimming"[^>]*>.*?<\/button>/.exec(
        row ?? "",
      )?.[0];
    const roll =
      /<button[^>]*data-testid="skill-swimming"[^>]*>.*?<\/button>/.exec(
        row ?? "",
      )?.[0];
    expect(name).toContain("Swimming");
    expect(name).toContain('aria-label="Description of Swimming"');
    expect(name).not.toContain("STR");
    expect(roll).toContain("STR 12");
    expect(roll).toContain('aria-label="Roll Swimming"');
    expect(roll).not.toContain("proficiency-name");
    // No button inside the other.
    expect(row?.match(/<button/g)).toHaveLength(2);
    // The class skill is a row of its own kind (step 19.6), never a proficiency's name button.
    expect(html).toContain('data-testid="skill-hide_in_shadows"');
    expect(html).not.toContain("proficiency-name-hide_in_shadows");
  });
});

// ADR 0037 step 19.6: a skill of a skill set is the same two sibling targets: the name opens the
// set's description, the percentage part still rolls the check.
describe("SkillRow_Step19_6_NameOpensThePopupPercentageRolls", () => {
  it("has a name button and a roll button side by side, in each set", async () => {
    const html = await render({
      ...base,
      skillSets: [
        skillSet("thief", [rogueSkill]),
        skillSet("bard", [rogueSkill]),
      ],
    });
    for (const set of ["thief", "bard"]) {
      const row = new RegExp(
        `data-testid="skill-row-${set}-hide_in_shadows">(.*?)</li>`,
      ).exec(html)?.[1];
      expect(row).toBeDefined();
      const name = new RegExp(
        `<button[^>]*data-testid="skill-name-${set}-hide_in_shadows"[^>]*>.*?</button>`,
      ).exec(row ?? "")?.[0];
      const roll =
        /<button[^>]*data-testid="skill-hide_in_shadows"[^>]*>.*?<\/button>/.exec(
          row ?? "",
        )?.[0];
      expect(name).toContain("Hide in Shadows");
      expect(name).toContain('aria-label="Description of Hide in Shadows"');
      expect(name).not.toContain("15%");
      expect(roll).toContain("15%");
      expect(roll).toContain('aria-label="Roll Hide in Shadows"');
      expect(roll).not.toContain("skill-name");
      // No button inside the other, and the terms stay on the row, outside both.
      expect(row?.match(/<button/g)).toHaveLength(2);
      expect(row).toContain('data-testid="skill-terms-hide_in_shadows"');
    }
  });
});

describe("SkillsTab with nothing to roll", () => {
  it("says so and names where to spend", async () => {
    const html = await render({ ...base, proficiencies: [], skillSets: [] });
    expect(html).toContain('data-testid="skills-empty"');
    expect(html).toContain("in the Skills activity");
    expect(html).not.toContain("skills-blocks");
  });
});

// ADR 0014 step 1 as amended: the row shows the server's score in effect, the sheet's beside it
// only when the server sends it.
describe("SkillsTab proficiency score", () => {
  it("shows the sheet score beside the score in effect when the server sends it", async () => {
    const html = await render({
      ...base,
      proficiencies: [
        {
          ...base.proficiencies[0],
          abilityScore: 14,
          sheetScore: 12,
          target: 14,
        },
      ],
    });
    expect(html).toContain("14");
    expect(html).toContain("(sheet 12)");
  });
  it("shows no sheet score when there is none", async () => {
    const html = await render(base);
    expect(html).not.toContain("skill-sheet-score");
    expect(html).not.toContain("(sheet");
  });
});
