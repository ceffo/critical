import { describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import i18next from "i18next";
import { I18nextProvider } from "react-i18next";
import { resources } from "@/i18n";
import type { RogueSkill, Skills } from "@/types/skills";
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
  bardSkills: [],
  thiefSkills: [],
  skillArmor: "",
  budget: null,
};

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
    expect(html).not.toContain("skill-armor");
  });

  it("shows the bard skills for a bard", async () => {
    const html = await render({ ...base, bardSkills: [rogueSkill] });
    expect(html).toContain("Bard skills");
    expect(html).not.toContain("Thief skills");
    expect(html).toContain("thief-points-edit");
  });

  it("shows the thief skills for a thief", async () => {
    const html = await render({
      ...base,
      thiefSkills: [rogueSkill],
      skillArmor: "leather",
    });
    expect(html).toContain("Thief skills");
    expect(html).not.toContain("Bard skills");
    expect(html).toContain("skill-armor");
  });
});

const thiefBudget = {
  level: 2,
  first: 60,
  perLevel: 30,
  available: 90,
  spent: 5,
  left: 85,
  perSkillFirst: 30,
  perSkillPerLevel: 15,
  perSkillCap: 45,
};

// ADR 0030 steps 17 and 18: the budget with its formula and each skill's terms are always text.
describe("SkillsTab budget and terms", () => {
  it("shows the available points with their formula, spent, left and the thief's cap", async () => {
    const html = await render({
      ...base,
      thiefSkills: [rogueSkill],
      budget: thiefBudget,
    });
    expect(html).toContain("Available: 60 + 30 × 1 = 90 (level 2)");
    expect(html).toContain("Spent 5, left 85");
    expect(html).toContain("At most 45 per skill (30 + 15 × 1 = 45)");
    expect(html).toContain("5 of 45 points");
  });

  it("shows a bard's budget without a per-skill cap", async () => {
    const html = await render({
      ...base,
      bardSkills: [rogueSkill],
      budget: {
        ...thiefBudget,
        first: 20,
        perLevel: 15,
        available: 35,
        left: 30,
        perSkillFirst: 0,
        perSkillPerLevel: 0,
        perSkillCap: 0,
      },
    });
    expect(html).toContain("Available: 20 + 15 × 1 = 35 (level 2)");
    expect(html).not.toContain("per skill");
  });

  it("writes each skill's terms, and says when the cap or the floor applied", async () => {
    const html = await render({
      ...base,
      thiefSkills: [
        { ...rogueSkill, race: 5, dexterity: -5, armor: 0 },
        { ...rogueSkill, id: "climb_walls", rawPercent: 112, percent: 95 },
        { ...rogueSkill, id: "read_languages", rawPercent: -5, percent: 0 },
      ],
    });
    expect(html).toContain(
      "Base 10, points 5, race +5, Dex −5, armour 0 = 15%",
    );
    expect(html).toMatch(/= 112%.{0,80}capped at 95%/);
    expect(html).toMatch(/= −5%.{0,80}floored at 0%/);
  });
});

// A proficiency row is two sibling targets, never one inside the other: the name opens the
// description, the ability part rolls the check (class skills stay one roll button).
describe("SkillsTab proficiency row", () => {
  it("has a name button and a roll button side by side", async () => {
    const html = await render({
      ...base,
      bardSkills: [rogueSkill],
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
    // The class skill stays one roll button.
    expect(html).toContain('data-testid="skill-hide_in_shadows"');
    expect(html).not.toContain("proficiency-name-hide_in_shadows");
  });
});
