import { describe, expect, it } from "vitest";
import { preparedDots, slotDots } from "./slotDots";
import type { SpellSlot } from "@/types/spells";

const slot = (over: Partial<SpellSlot> = {}): SpellSlot => ({
  level: 1,
  prepared_spell_id: "magic_missile",
  prepared_spell_name: "Magic Missile",
  used: false,
  ...over,
});

describe("slotDots", () => {
  it("shows the available slots, then the used ones, then the unprepared ones", () => {
    expect(slotDots([slot({ used: true }), slot(), slot()], 4)).toEqual({
      dots: ["available", "available", "used", "empty"],
      available: 2,
      used: 1,
      empty: 1,
    });
  });

  it("turns an available dot into a used one after a cast", () => {
    const before = slotDots([slot(), slot()], 2);
    const after = slotDots([slot({ used: true }), slot()], 2);
    expect(before.dots).toEqual(["available", "available"]);
    expect(after.dots).toEqual(["available", "used"]);
  });

  it("adds the extra slots a spell granted without counting them against the limit", () => {
    // Two level 1 slots prepared out of 3, plus two blank extra slots, one already cast.
    const slots = [
      slot(),
      slot({ used: true }),
      slot({ extra: true, prepared_spell_id: "", prepared_spell_name: "" }),
      slot({
        extra: true,
        used: true,
        prepared_spell_id: "",
        prepared_spell_name: "",
      }),
    ];
    expect(slotDots(slots, 3)).toEqual({
      dots: ["available", "available", "used", "used", "empty"],
      available: 2,
      used: 2,
      empty: 1,
    });
  });

  it("has no empty dot without a known limit or when the level is over it", () => {
    expect(slotDots([slot()], undefined).dots).toEqual(["available"]);
    expect(slotDots([slot(), slot()], 1).dots).toEqual([
      "available",
      "available",
    ]);
  });
});

describe("preparedDots", () => {
  it("fills one dot per prepared spell and outlines the slots left", () => {
    expect(preparedDots(2, 3)).toEqual(["available", "available", "empty"]);
  });
  it("is all outlines with nothing prepared and all filled at the limit", () => {
    expect(preparedDots(0, 2)).toEqual(["empty", "empty"]);
    expect(preparedDots(2, 2)).toEqual(["available", "available"]);
  });
  it("marks each spell beyond the limit as over", () => {
    expect(preparedDots(4, 3)).toEqual([
      "available",
      "available",
      "available",
      "over",
    ]);
    expect(preparedDots(1, 0)).toEqual(["over"]);
  });
});
