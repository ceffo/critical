import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/api/http";
import { emptyPlacement, place } from "./flow";
import type { ScoresArrangement, ScoresRoll } from "@/types/character";
import { arrangeFor, pickRace } from "./arrange";
import { EMPTY_DRAFT, type FlowDraft } from "./flow";

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
const typed = {
  scores: { str: "11", dex: "", con: "", int: "", wis: "", cha: "" },
  arranged: null,
};

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
    const dwarf = await arrangeFor(roll, placement, "dwarf", {
      scores: human.scores!,
      arranged: human.arranged!,
    });

    const placed = { str: [], dex: [], con: [18], int: [], wis: [], cha: [3] };
    expect(arrangeScores).toHaveBeenNthCalledWith(1, "III", "human", placed);
    expect(arrangeScores).toHaveBeenNthCalledWith(2, "III", "dwarf", placed);
    expect(human.scores).toEqual({ ...typed.scores, con: "18", cha: "3" });
    // The server's 19 and 2 as they are; the creation refuses the 2 on the scores step.
    expect(dwarf.scores).toEqual({ ...typed.scores, con: "19", cha: "2" });
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

  it("a total moved to another ability leaves its old field empty: the answer is the whole truth", async () => {
    arrangeScores.mockResolvedValueOnce(
      answer({ con: 0, cha: 0 }, { con: 18, cha: 3 }),
    );
    const first = await arrangeFor(roll, placement, "human", typed);
    // The 18 moves from Constitution to Strength; the server's answer no longer names Constitution.
    const moved = place(roll, placement, 0, "str")!;
    arrangeScores.mockResolvedValueOnce({
      ...answer({ str: 0, cha: 0 }, { str: 18, cha: 3 }),
      rolled: { str: 18, cha: 3 },
    });
    const second = await arrangeFor(roll, moved, "human", {
      scores: first.scores!,
      arranged: first.arranged!,
    });
    expect(second.scores).toEqual({
      str: "18",
      dex: "",
      con: "",
      int: "",
      wis: "",
      cha: "3",
    });
  });

  it("a new roll empties the fields the old one filled, but keeps a value the player typed over it", async () => {
    const before = {
      // The old roll filled CON 18 and CHA 3; the player then typed 9 over the 3.
      scores: { str: "11", dex: "", con: "18", int: "", wis: "", cha: "9" },
      arranged: answer({ con: 0, cha: 0 }, { con: 18, cha: 3 }),
    };
    arrangeScores.mockResolvedValueOnce(answer({ dex: 0 }, { dex: 14 }));
    const next = await arrangeFor(roll, placement, "human", before);
    expect(next.scores).toEqual({
      str: "11",
      dex: "14",
      con: "",
      int: "",
      wis: "",
      cha: "9",
    });
  });
});

/** A deferred promise: the test decides when (and in which order) the server's replies land. */
function deferred<T>() {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

/** The flow's draft and its revisions, as `CreationFlow` keeps them. */
function flowState(start: FlowDraft) {
  let draft = start;
  let revision = 0;
  return {
    get draft() {
      return draft;
    },
    update: (patch: Partial<FlowDraft>) => {
      revision += 1;
      draft = { ...draft, ...patch };
    },
    watch: () => {
      const at = revision;
      return () => revision === at;
    },
  };
}

describe("pickRace (a race changed after the roll)", () => {
  beforeEach(() => arrangeScores.mockReset());

  const rolled: FlowDraft = {
    ...EMPTY_DRAFT,
    race: "dwarf",
    roll,
    placement,
    scores: { str: "", dex: "", con: "19", int: "", wis: "", cha: "2" },
    arranged: answer({ con: 1, cha: -1 }, { con: 19, cha: 2 }),
  };

  it("Elf then Human: the Elf reply landing last is dropped, the scores are Human's", async () => {
    const elf = deferred<ScoresArrangement>();
    const human = deferred<ScoresArrangement>();
    arrangeScores
      .mockReturnValueOnce(elf.promise)
      .mockReturnValueOnce(human.promise);
    const flow = flowState(rolled);

    const pickElf = pickRace(flow.draft, "elf", flow.update, flow.watch);
    const pickHuman = pickRace(flow.draft, "human", flow.update, flow.watch);
    human.resolve(answer({ con: 0, cha: 0 }, { con: 18, cha: 3 }));
    await pickHuman;
    elf.resolve(answer({ con: -1, cha: 0 }, { con: 17, cha: 3 }));
    await pickElf;

    expect(arrangeScores).toHaveBeenCalledTimes(2);
    expect(flow.draft.race).toBe("human");
    expect(flow.draft.scores.con).toBe("18");
    expect(flow.draft.arranged?.adjustments).toEqual({ con: 0, cha: 0 });
  });

  it("a stale refusal does not drop the roll the latest pick kept", async () => {
    const elf = deferred<ScoresArrangement>();
    const human = deferred<ScoresArrangement>();
    arrangeScores
      .mockReturnValueOnce(elf.promise)
      .mockReturnValueOnce(human.promise);
    const flow = flowState(rolled);

    const pickElf = pickRace(flow.draft, "elf", flow.update, flow.watch);
    const pickHuman = pickRace(flow.draft, "human", flow.update, flow.watch);
    human.resolve(answer({ con: 0, cha: 0 }, { con: 18, cha: 3 }));
    await pickHuman;
    elf.reject(new ApiError("boom", 500, undefined, "internal"));
    await pickElf;

    expect(flow.draft.roll).toBe(roll);
    expect(flow.draft.scores.con).toBe("18");
  });

  it("the latest pick's refusal drops the roll rather than leave it on the old race", async () => {
    arrangeScores.mockRejectedValueOnce(
      new ApiError("boom", 500, undefined, "internal"),
    );
    const flow = flowState(rolled);
    await pickRace(flow.draft, "human", flow.update, flow.watch);
    expect(flow.draft.race).toBe("human");
    expect(flow.draft.roll).toBeNull();
    expect(flow.draft.arranged).toBeNull();
  });
});
