import { describe, expect, it } from "vitest";
import { abilityDraft, draftsToAbilities } from "./abilityEditor";
import type { ItemAbility } from "@/types/items";

describe("abilityEditor spell effect (ADR 0049 step 10)", () => {
  const wand: ItemAbility = {
    id: "zap",
    limit: { kind: "charges", max: 5, left: 4, cost: 1 },
    effects: [
      { kind: "spell", spellId: "magic_missile", level: 5 },
      { kind: "dice", dice: "1d6" },
    ],
  };

  it("keeps a stored spell effect through a save, as stored", () => {
    const sent = draftsToAbilities([abilityDraft(wand, "en")], "en");
    expect(sent?.[0].effects).toEqual(wand.effects);
  });
});
