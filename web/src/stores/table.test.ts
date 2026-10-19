import { beforeEach, describe, expect, it, vi } from "vitest";

const roundCalls: { resolve: (round: number) => void }[] = [];

vi.mock("../api/round", () => ({
  fetchRound: () =>
    new Promise((resolve) => {
      roundCalls.push({ resolve: (round) => resolve({ round }) });
    }),
}));
vi.mock("@/api/hp", () => ({
  getHpState: async () => ({
    currentHp: 1,
    maxHp: 1,
    floor: -10,
    condition: "ok" as const,
    events: [],
  }),
}));
vi.mock("@/api/log", () => ({
  fetchLog: async () => ({ entries: [], topId: null, redo: null }),
}));
vi.mock("@/api/items", () => ({ fetchItems: async () => [] }));
vi.mock("@/api/combat", () => ({ fetchWeapons: async () => [] }));
vi.mock("@/api/ac", () => ({ fetchAc: async () => null }));
vi.mock("../api/buffs", () => ({
  fetchActiveBuffs: async () => [],
  fetchSavedBuffs: async () => [],
  fetchStrength: async () => null,
}));

import { useBuffsStore } from "./buffs";
import { useTableStore } from "./table";

const tick = () => new Promise((r) => setTimeout(r, 0));

describe("table refresh ordering", () => {
  beforeEach(() => {
    roundCalls.length = 0;
    useBuffsStore.setState({ round: 0 });
  });

  it("the newest refresh wins when responses would arrive out of order", async () => {
    const first = useTableStore.getState().refresh();
    const second = useTableStore.getState().refresh();

    // Answer the most recently issued request first, then the older one; the n-th request
    // issued sees server state n (so the newest state is 2).
    const answered = new Set<number>();
    while (answered.size < 2) {
      await tick();
      const pending = roundCalls
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
