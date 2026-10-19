import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/api/http";
import { emptyPlacement, place } from "@/lib/creationFlow";
import type { ScoresArrangement, ScoresRoll } from "@/types/character";
import { arrangeFor } from "./arrange";

const arrangeScores = vi.fn();
vi.mock("@/api/characters", () => ({
  arrangeScores: (...args: unknown[]) => arrangeScores(...args),
}));

// Method III, six totals; the player puts 18 on Constitution and 3 on Charisma.
const roll: ScoresRoll = {
  method: {
    code: "III",
    rolls: 6,
    dice: 3,
    dropLowest: false,
    assign: "arrange",
    keep: 6,
    base: 0,
    max: 0,
  },
  rolls: [18, 3, 12, 10, 9, 14].map((total) => ({
    dice: [total],
    dropped: null,
    total,
    ability: "",
  })),
  adjustments: { str: 0, dex: 0, con: 0, int: 0, wis: 0, cha: 0 },
  scores: null,
};
const placement = place(
  roll,
  place(roll, emptyPlacement(), 0, "con")!,
  1,
  "cha",
)!;
const typed = { str: "11", dex: "", con: "", int: "", wis: "", cha: "" };

const answer = (
  adjustments: Record<string, number>,
  scores: Record<string, number>,
): ScoresArrangement => ({
  method: "III",
  adjustments,
  rolled: { con: 18, cha: 3 },
  scores,
});

describe("arrangeFor (the server's scores for a placement, ADR 0032 step 6)", () => {
  beforeEach(() => arrangeScores.mockReset());

  it("a race changed after the roll sends the same values: the scores and the Race column follow", async () => {
    arrangeScores.mockResolvedValueOnce(
      answer({ con: 0, cha: 0 }, { con: 18, cha: 3 }),
    );
    const human = await arrangeFor(roll, placement, "human", typed);
    arrangeScores.mockResolvedValueOnce(
      answer({ con: 1, cha: -1 }, { con: 19, cha: 2 }),
    );
    const dwarf = await arrangeFor(roll, placement, "dwarf", human.scores!);

    const placed = { str: [], dex: [], con: [18], int: [], wis: [], cha: [3] };
    expect(arrangeScores).toHaveBeenNthCalledWith(1, "III", "human", placed);
    expect(arrangeScores).toHaveBeenNthCalledWith(2, "III", "dwarf", placed);
    expect(human.scores).toEqual({ ...typed, con: "18", cha: "3" });
    // The server's 19 and 2 as they are; the creation refuses the 2 on the scores step.
    expect(dwarf.scores).toEqual({ ...typed, con: "19", cha: "2" });
    // The Race column reads the arrangement's adjustments, not the roll's.
    expect(dwarf.arranged?.adjustments).toEqual({ con: 1, cha: -1 });
    expect(dwarf.placement).toBe(placement);
  });

  it("a refusal (method VI above its max) is thrown and changes nothing", async () => {
    arrangeScores.mockRejectedValueOnce(
      new ApiError("above_max", 400, "placed.con", "invalid_input"),
    );
    await expect(arrangeFor(roll, placement, "", typed)).rejects.toThrow(
      "above_max",
    );
  });

  it("an older server without the arrangement keeps the placement and the typed scores", async () => {
    arrangeScores.mockRejectedValueOnce(
      new ApiError("not found", 404, undefined, "not_found"),
    );
    expect(await arrangeFor(roll, placement, "elf", typed)).toEqual({
      placement,
      arranged: null,
    });
  });
});
