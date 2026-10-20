import { describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import i18next from "i18next";
import { I18nextProvider } from "react-i18next";
import { resources } from "@/i18n";
import type { RogueSkill, SkillBudget, Skills } from "@/types/skills";
import { ThiefPointsEditor } from "./ThiefPointsEditor";
import { ApiError } from "@/api/http";
import { previewDraft, typedPoints } from "./skillsPreview";

vi.mock("@/stores/skills", () => ({
  useSkillsStore: (select: (s: { savePoints: () => void }) => unknown) =>
    select({ savePoints: () => {} }),
}));
vi.mock("@/stores/table", () => ({
  useTableStore: (select: (s: { busy: boolean }) => unknown) =>
    select({ busy: false }),
}));

const skill = (id: string, points: number): RogueSkill => ({
  id,
  base: 10,
  points,
  race: 5,
  dexterity: 0,
  armor: 0,
  percent: 15 + points,
  rawPercent: 15 + points,
  maxPercent: 95,
  maxPoints: 95,
  usable: true,
  last: null,
});

const budget: SkillBudget = {
  level: 1,
  first: 60,
  perLevel: 30,
  available: 60,
  spent: 30,
  left: 30,
  perSkillFirst: 30,
  perSkillPerLevel: 15,
  perSkillCap: 30,
};

const skills = [skill("open_locks", 30), skill("pick_pockets", 0)];

describe("ThiefPointsEditor", () => {
  it("shows the stored points' terms and budget before the first preview", async () => {
    const inst = i18next.createInstance();
    await inst.init({
      lng: "en",
      fallbackLng: "en",
      defaultNS: "common",
      resources,
      interpolation: { escapeValue: false },
    });
    const html = renderToStaticMarkup(
      <I18nextProvider i18n={inst}>
        <ThiefPointsEditor skills={skills} budget={budget} onDone={() => {}} />
      </I18nextProvider>,
    );
    expect(html).toContain("Available: 60 + 30 × 0 = 60 (level 1)");
    expect(html).toContain("Spent 30, left 30");
    expect(html).toContain(
      "Base 10, points 30, race +5, Dex 0, armour 0 = 45%",
    );
    expect(html).toContain("0 of 30 points");
  });
});

describe("previewDraft", () => {
  it("asks the server for the typed points, zeros left out, and returns its answer", async () => {
    const answer = { budget, thiefSkills: skills } as unknown as Skills;
    const fetchPreview = vi.fn().mockResolvedValue(answer);
    const got = await previewDraft(
      { open_locks: "40", pick_pockets: " 0 " },
      skills,
      fetchPreview,
    );
    expect(fetchPreview).toHaveBeenCalledExactlyOnceWith({ open_locks: 40 });
    expect(got).toEqual({ ok: true, skills: answer });
  });

  it("asks nothing while a field is not a whole number", async () => {
    const fetchPreview = vi.fn();
    expect(typedPoints({ open_locks: "4x" }, skills)).toBeNull();
    expect(
      await previewDraft({ open_locks: "4x" }, skills, fetchPreview),
    ).toEqual({ ok: false });
    expect(fetchPreview).not.toHaveBeenCalled();
  });

  it("names the field the server refuses, so the editor can show it", async () => {
    const fetchPreview = vi
      .fn()
      .mockRejectedValue(
        new ApiError("out_of_range", 400, "skillPoints.open_locks"),
      );
    expect(
      await previewDraft({ open_locks: "96" }, skills, fetchPreview),
    ).toEqual({
      ok: false,
      field: "skillPoints.open_locks",
      message: "out_of_range",
    });
  });

  it("keeps an over-budget refusal's code and worded message, so the editor shows it", async () => {
    const fetchPreview = vi
      .fn()
      .mockRejectedValue(
        new ApiError(
          "Over budget: 31 for 30 available.",
          409,
          "skillPoints.open_locks",
          "over_budget",
        ),
      );
    expect(
      await previewDraft({ open_locks: "31" }, skills, fetchPreview),
    ).toEqual({
      ok: false,
      field: "skillPoints.open_locks",
      message: "Over budget: 31 for 30 available.",
      code: "over_budget",
    });
  });

  it("is not ok, with no field, when the call fails", async () => {
    const fetchPreview = vi.fn().mockRejectedValue(new TypeError("offline"));
    expect(
      await previewDraft({ open_locks: "40" }, skills, fetchPreview),
    ).toEqual({ ok: false });
  });
});
