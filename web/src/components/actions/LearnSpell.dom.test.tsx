import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { renderApp } from "@/test/render";
import { useCharacterStore } from "@/stores/character";
import { useLookupStore } from "@/stores/lookup";
import { useSpellStore } from "@/stores/spells";
import type { Character, SpellbookRules } from "@/types/character";

vi.mock("@/api/spells", () => ({ addSpell: vi.fn(), learnSpell: vi.fn() }));

import { LearnSpell } from "./LearnSpell";

const rules = (over: Partial<SpellbookRules>): SpellbookRules => ({
  classMaxSpellLevel: 9,
  intMaxSpellLevel: 9,
  learnChance: 55,
  maxSpellsPerLevel: 99,
  noMaximum: true,
  schoolAdjustment: 0,
  otherAdjustment: 0,
  ...over,
});

function seed(
  r: SpellbookRules,
  freeSpells: { level: number; spellLevel: number; ofSchool?: true }[] = [],
) {
  useCharacterStore.setState({
    character: {
      derived: { spellbook: r, levelUp: { toSpend: { freeSpells } } },
    } as unknown as Character,
  });
}

beforeEach(() => {
  useSpellStore.setState({
    info: { spellbook: [], limits: [] },
    fetchInfo: () => Promise.resolve(),
  });
  useLookupStore.setState({ grant: null });
});

describe("LearnSpell (ADR 0050)", () => {
  it("words a specialist's two adjustments before the roll, from the server's figures", async () => {
    seed(rules({ schoolAdjustment: 15, otherAdjustment: -15 }));
    await renderApp(<LearnSpell spellId="sleep" spellLevel={1} wizard />);
    expect(screen.getByTestId("learn-chance")).toHaveTextContent(
      "Chance to learn: 55%, +15 for a spell of his school, −15 for any other",
    );
  });

  it("says nothing of an adjustment for a mage", async () => {
    seed(rules({}));
    await renderApp(<LearnSpell spellId="sleep" spellLevel={1} wizard />);
    expect(screen.queryByTestId("learn-chance")).toBeNull();
  });

  it("words the free spell 'of his school' when the server says ofSchool, and plain otherwise", async () => {
    seed(rules({}), [{ level: 3, spellLevel: 2, ofSchool: true }]);
    useLookupStore.setState({ grant: { level: 3, spellLevel: 2 } });
    await renderApp(<LearnSpell spellId="sleep" spellLevel={2} wizard />);
    expect(screen.getByTestId("free-spell-claim")).toHaveTextContent(
      "Level 3's free spell, of his school",
    );
  });

  it("marks a spell kept in the book as opposed", async () => {
    seed(rules({}));
    useSpellStore.setState({
      info: {
        spellbook: [{ id: "sleep", name: "Sleep", level: 1, opposed: true }],
        limits: [],
      },
    });
    await renderApp(<LearnSpell spellId="sleep" spellLevel={1} wizard />);
    expect(screen.getByTestId("learn-known")).toHaveTextContent(
      "In the spellbook, opposed school",
    );
  });
});
