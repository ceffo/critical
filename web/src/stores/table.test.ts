import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Slice } from "@/lib/stateKeys";

// Every read of the action module, mocked at the API: `reads` counts each slice's request, and
// `write` stands for a write whose answer named `header` in X-State-Keys (null: no header).
const h = vi.hoisted(() => {
  const s = {
    reads: {} as Record<string, number>,
    roundCalls: [] as { resolve: (round: number) => void }[],
    holdRound: false,
    read:
      <T>(slice: string, value: T) =>
      async () => {
        s.reads[slice] = (s.reads[slice] ?? 0) + 1;
        return value;
      },
  };
  return s;
});

vi.mock("@/api/hp", () => ({
  getHpState: h.read("hp", {
    currentHp: 1,
    maxHp: 1,
    floor: -10,
    condition: "ok",
    events: [],
  }),
  appendHpEvent: async () => {
    const { noteWrite } = await import("@/lib/stateKeys");
    noteWrite("hp,log");
    return { currentHp: 0, maxHp: 1, floor: -10, condition: "ok", events: [] };
  },
}));
vi.mock("@/api/log", () => ({
  fetchLog: h.read("log", { entries: [], topId: null, redo: null }),
}));
vi.mock("../api/buffs", () => ({
  fetchActiveBuffs: h.read("buffs", { buffs: [], powers: [] }),
  fetchSavedBuffs: h.read("savedBuffs", []),
  fetchStrength: h.read("strength", null),
}));
vi.mock("../api/round", () => ({
  fetchRound: () => {
    h.reads.round = (h.reads.round ?? 0) + 1;
    if (!h.holdRound) return Promise.resolve({ round: 0 });
    return new Promise((resolve) => {
      h.roundCalls.push({ resolve: (round) => resolve({ round }) });
    });
  },
}));
vi.mock("@/api/items", () => ({ fetchItems: h.read("items", []) }));
vi.mock("@/api/combat", () => ({ fetchWeapons: h.read("weapons", []) }));
vi.mock("@/api/ac", () => ({ fetchAc: h.read("ac", null) }));
vi.mock("../api/spells", () => ({
  fetchSpellSlots: h.read("slots", []),
  fetchPreparationInfo: h.read("spellInfo", { spellbook: [], limits: [] }),
}));
vi.mock("@/api/skills", () => ({ fetchSkills: h.read("skills", {}) }));
vi.mock("@/api/character", () => ({ fetchCharacter: h.read("character", {}) }));
vi.mock("@/api/xp", () => ({
  fetchXp: h.read("xp", { total: 0, entries: [] }),
}));

import {
  noteAnswer,
  noteWrite,
  STATE_FAMILIES,
  takeChanges,
} from "@/lib/stateKeys";
import { ApiError } from "@/api/http";
import { useBuffsStore } from "./buffs";
import { useHpStore } from "./hp";
import { useSkillsStore } from "./skills";
import { useSpellStore } from "./spells";
import { useTableStore } from "./table";
import { useXpStore } from "./xp";

const tick = () => new Promise((r) => setTimeout(r, 0));

/** The slices read since the last reset, sorted. */
const readSlices = () => Object.keys(h.reads).sort();

/** A write whose answer named header (null: none). */
const write = (header: string | null) => async () => noteWrite(header);

/** A write the server refused with status and code (on field), noted as api/http.ts notes it. */
const refused = (status: number, code: string, field?: string) => async () => {
  noteAnswer(status, null);
  throw new ApiError(code, status, field, code);
};

/** Every slice, each once. */
const allSlices = () =>
  Object.keys(expected)
    .flatMap((f) => expected[f as keyof typeof expected])
    .filter((s, i, all) => all.indexOf(s) === i)
    .sort();

function reset() {
  h.reads = {};
  h.roundCalls = [];
  h.holdRound = false;
  takeChanges();
  useTableStore.setState({ error: null, busy: false, notice: null });
}

/** Every panel shown: the slices read only once loaded (the ledger, the skills, the spellbook) are
 * loaded. */
function everyPanelShown() {
  useXpStore.setState({ ledger: { total: 0, entries: [] } as never });
  useSkillsStore.setState({ skills: {} as never });
  useSpellStore.setState({ infoLoaded: true });
}

// ADR 0018 step 11: a write's answer names its state keys; the action module reads again the
// slices mapped to their families, exactly, and nothing else. One case per family, written out
// here rather than read from the map, so a changed mapping fails a test.
const expected: Record<(typeof STATE_FAMILIES)[number], Slice[]> = {
  log: ["log"],
  hp: ["hp"],
  ac_override: ["ac"],
  buffs: ["ac", "buffs", "skills", "strength"],
  initiative: ["round"],
  items: ["ac", "buffs", "items", "skills", "spellInfo", "strength", "weapons"],
  proficiencies: ["character", "items", "skills", "weapons"],
  class_entries: ["character", "hp", "skills", "slots", "spellInfo"],
  realized_level: ["character", "hp", "skills", "slots", "spellInfo"],
  round: ["round"],
  saved_buffs: ["savedBuffs"],
  skill_armor: ["character", "skills"],
  skill_points: ["character", "skills"],
  spell_slots: ["slots"],
  spellbook: ["character", "spellInfo"],
  weapons: [
    "ac",
    "buffs",
    "items",
    "skills",
    "spellInfo",
    "strength",
    "weapons",
  ],
  xp: ["character", "skills", "slots", "spellInfo", "xp"],
};

describe("the action module reads again what a write's keys name", () => {
  beforeEach(() => {
    reset();
    everyPanelShown();
  });

  it.each(STATE_FAMILIES)("family %s", async (family) => {
    const key = family === "items" ? "items:arrows" : family;
    expect(await useTableStore.getState().run(write(key))).toBe(true);
    expect(readSlices()).toEqual(expected[family].slice().sort());
    expect(Object.values(h.reads).every((n) => n === 1)).toBe(true);
  });

  it("several keys read the union, each slice once", async () => {
    await useTableStore
      .getState()
      .run(write("log,spell_slots,items:a,items:b"));
    expect(readSlices()).toEqual(
      [...expected.log, ...expected.spell_slots, ...expected.items].sort(),
    );
    expect(Object.values(h.reads).every((n) => n === 1)).toBe(true);
  });

  it("a write that logged nothing reads nothing", async () => {
    await useTableStore.getState().run(write(""));
    expect(readSlices()).toEqual([]);
  });

  it("a slice the action set from its answer is not read again", async () => {
    expect(await useTableStore.getState().recordHp(-1, "orc")).toBe(true);
    expect(readSlices()).toEqual(["log"]);
    expect(useHpStore.getState().currentHp).toBe(0);
  });

  it("a slice no panel has loaded stays unread", async () => {
    useXpStore.setState({ ledger: null });
    useSkillsStore.setState({ skills: null });
    useSpellStore.setState({ infoLoaded: false });
    await useTableStore.getState().run(write("log,xp:12"));
    expect(readSlices()).toEqual(["character", "log", "slots"]);
  });

  // Rolling deploy (ADR 0024 step 5.2): a server of the release before sends no header.
  it.each([
    ["an answer without the header", write(null)],
    ["a family this web does not know", write("log,familiars")],
    [
      "a lost answer",
      async () => {
        noteWrite(null);
        throw new TypeError("Failed to fetch");
      },
    ],
    // A stale undo or redo top, a level-up no longer pending: the player sees the new state.
    ["a conflict (stale undo top)", refused(409, "log_top_changed")],
    ["a conflict (stale level-up)", refused(409, "level_up_not_pending")],
    ["a weapon removed elsewhere", refused(404, "weapon_not_found")],
    ["a server error", refused(500, "internal")],
  ])("%s reads every slice again", async (_, action) => {
    await useTableStore.getState().submit(action);
    expect(readSlices()).toEqual(allSlices());
  });

  // A refusal rolled back: nothing changed, nothing is read again (a dice typo, a bad form field).
  it.each([
    ["a refused dice formula", refused(400, "invalid_formula"), false],
    ["a refused form field", refused(400, "invalid_input", "base"), true],
    ["a refused input", refused(422, "invalid_input"), false],
  ])("%s reads nothing", async (_, action, onField) => {
    const rejected = await useTableStore.getState().submit(action);
    expect(readSlices()).toEqual([]);
    expect(rejected !== null).toBe(onField);
  });

  it("a refusal after a write that landed reads that write's keys only", async () => {
    await useTableStore.getState().run(async () => {
      await write("spell_slots")();
      await refused(400, "invalid_input")();
    });
    expect(readSlices()).toEqual(expected.spell_slots.slice().sort());
  });

  it("a slice the action set from its answer is read again after a conflict", async () => {
    await useTableStore.getState().run(refused(409, "conflict"), {
      fresh: ["hp"],
    });
    expect(readSlices()).toEqual(allSlices());
  });

  it("the busy flag and the error belong to the action module", async () => {
    let busyDuring = false;
    const ok = await useTableStore.getState().run(async () => {
      busyDuring = useTableStore.getState().busy;
      throw new Error("refused");
    });
    expect(ok).toBe(false);
    expect(busyDuring).toBe(true);
    expect(useTableStore.getState().busy).toBe(false);
    expect(useTableStore.getState().error).toBe("refused");

    // A background action leaves both alone.
    await useTableStore.getState().run(write(""), { quiet: true });
    expect(useTableStore.getState().error).toBe("refused");
  });
});

describe("overlapping writes", () => {
  beforeEach(() => {
    reset();
    everyPanelShown();
  });

  /** A write held until answered with header. */
  function held() {
    let answer!: (header: string) => void;
    const done = new Promise<string>((r) => (answer = r));
    return {
      answer,
      action: async () => noteWrite(await done),
    };
  }

  it.each([
    ["the newer answers first", true],
    ["the older answers first", false],
  ])("%s: neither loses the other's keys", async (_, newerFirst) => {
    const a = held();
    const b = held();
    const first = useTableStore.getState().run(a.action);
    const second = useTableStore.getState().run(b.action);
    await tick();
    if (newerFirst) {
      b.answer("spell_slots");
      await tick();
      a.answer("xp:3");
    } else {
      a.answer("xp:3");
      await tick();
      b.answer("spell_slots");
    }
    await Promise.all([first, second]);
    expect(readSlices()).toEqual(
      [...new Set([...expected.xp, ...expected.spell_slots])].sort(),
    );
  });
});

describe("table refresh ordering", () => {
  beforeEach(() => {
    reset();
    h.holdRound = true;
    useBuffsStore.setState({ round: 0 });
  });

  it("the newest refresh wins when responses would arrive out of order", async () => {
    const first = useTableStore.getState().run(write("round"));
    await tick(); // the first action has read its answer's keys
    const second = useTableStore.getState().run(write("round"));

    // Answer the most recently issued request first, then the older one; the n-th request
    // issued sees server state n (so the newest state is 2).
    const answered = new Set<number>();
    while (answered.size < 2) {
      await tick();
      const pending = h.roundCalls
        .map((c, i) => ({ c, i }))
        .filter(({ i }) => !answered.has(i));
      if (pending.length === 0) continue;
      const { c, i } = pending[pending.length - 1];
      answered.add(i);
      c.resolve(i + 1);
    }
    await Promise.all([first, second]);

    expect(useBuffsStore.getState().round).toBe(2);
  });
});
