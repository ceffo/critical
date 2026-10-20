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
