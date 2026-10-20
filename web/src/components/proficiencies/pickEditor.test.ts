import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18next from "i18next";
import { ApiError } from "@/api/http";
import { resources } from "@/i18n";
import type {
  ProficienciesAnswer,
  ProficienciesInput,
  ProficiencySlots,
} from "@/types/character";
import { PREVIEW_DELAY, schedulePreview } from "./pickEditor";
import { wordRefusal } from "./refusal";

const slots = { weaponPicks: [] } as unknown as ProficiencySlots;
const answer = (s: ProficiencySlots | null) =>
  ({ proficiencySlots: s, warnings: [] }) as unknown as ProficienciesAnswer;

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe("the pick editor's preview (ADR 0037 steps 12 and 13)", () => {
  it("debounces: only the picks left after the rest are asked, once", async () => {
    const ask = vi.fn(async (i: ProficienciesInput) => {
      void i;
      return answer(slots);
    });
    const figures = vi.fn();
    const on = { figures, refused: vi.fn() };
    // The editor cancels the wait of the previous picks when they change.
    const cancel = schedulePreview(
      { weapon: [{ type: "a", slots: 1 }] },
      "weapon",
      on,
      ask,
    );
    vi.advanceTimersByTime(PREVIEW_DELAY - 1);
    cancel();
    schedulePreview({ weapon: [{ type: "b", slots: 1 }] }, "weapon", on, ask);
    vi.advanceTimersByTime(PREVIEW_DELAY - 1);
    expect(ask).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    await vi.runAllTimersAsync();
    expect(ask).toHaveBeenCalledTimes(1);
    expect(ask).toHaveBeenCalledWith({ weapon: [{ type: "b", slots: 1 }] });
    expect(figures).toHaveBeenCalledTimes(1);
  });

  it("flags the figures out of date on a refusal, and drops the answer of a cancelled call", async () => {
    const on = { figures: vi.fn(), refused: vi.fn() };
    const refuse = vi.fn(async () => {
      throw new ApiError("out_of_range", 422, "nonweapon.swimming");
    });
    schedulePreview({ nonweapon: [] }, "nonWeapon", on, refuse);
    await vi.runAllTimersAsync();
    expect(on.refused).toHaveBeenCalledWith({
      field: "nonweapon.swimming",
      code: undefined,
      message: "out_of_range",
    });
    expect(on.figures).not.toHaveBeenCalled();

    const cancel = schedulePreview({ nonweapon: [] }, "nonWeapon", on, refuse);
    vi.advanceTimersByTime(PREVIEW_DELAY);
    cancel();
    await vi.runAllTimersAsync();
    expect(on.refused).toHaveBeenCalledTimes(1);
  });

  it("names a failure that is not a refusal on the pool's own field", async () => {
    const on = { figures: vi.fn(), refused: vi.fn() };
    const down = vi.fn(async () => {
      throw new Error("offline");
    });
    schedulePreview({ weapon: [] }, "weapon", on, down);
    await vi.runAllTimersAsync();
    expect(on.refused).toHaveBeenCalledWith({
      field: "weapon",
      message: "offline",
    });
  });
});

describe("the pick editor's refusal wording (ADR 0037 steps 10 and 12)", () => {
  async function t(lng: string) {
    const inst = i18next.createInstance();
    await inst.init({
      lng,
      fallbackLng: "en",
      defaultNS: "common",
      resources,
      interpolation: { escapeValue: false },
    });
    return inst.t;
  }

  it("words an over-budget refusal with the server's message", async () => {
    expect(
      wordRefusal(await t("fr"), {
        field: "nonweapon",
        code: "over_budget",
        message: "3 slots, 2 left",
      }),
    ).toBe("3 slots, 2 left");
  });

  it("words a pick's own refusal by its pool's wording, and falls back to the generic one", async () => {
    const fr = await t("fr");
    expect(
      wordRefusal(fr, { field: "nonweapon.x", message: "unheard_of" }),
    ).toBe(fr("proficiencies.errors.refused"));
    expect(
      wordRefusal(fr, { field: "weapon.sword", message: "unheard_of" }),
    ).toBe(fr("weaponProficiencies.errors.refused"));
    expect(wordRefusal(fr, { field: "other" })).toBe(
      fr("proficiencies.errors.refused"),
    );
  });
});
