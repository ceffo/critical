import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Ended } from "@/types/buffs";

// ADR 0004 step 21: stopping a row sends the cause the player picked, and what its end effects did
// becomes the notice; a stop with none leaves no notice.
const h = vi.hoisted(() => ({
  calls: [] as { id: string; cause?: string }[],
  ended: undefined as Ended | undefined,
}));

vi.mock("../api/buffs", () => ({
  dismissBuff: async (id: string, cause?: string) => {
    h.calls.push({ id, cause });
    return { buffs: [], powers: [], ended: h.ended };
  },
}));

const { useTableStore } = await import("./table");

const ended: Ended = {
  effects: [
    {
      spellId: "polymorph_self",
      buff: "Polymorph Self",
      cause: "voluntary",
      kind: "heal",
      dice: "1d12",
      rolls: [{ sides: 12, value: 4 }],
      constant: 0,
      total: 4,
    },
  ],
  hpBefore: 3,
  hpAfter: 7,
};

describe("dismissBuff", () => {
  beforeEach(() => {
    h.calls = [];
    useTableStore.setState({ notice: null, error: null, busy: false });
  });

  it("sends the cause and shows what the end effects did", async () => {
    h.ended = ended;
    expect(await useTableStore.getState().dismissBuff("p", "voluntary")).toBe(
      true,
    );
    expect(h.calls).toEqual([{ id: "p", cause: "voluntary" }]);
    expect(useTableStore.getState().notice).toEqual({
      kind: "ended",
      buffs: [],
      ended,
    });
  });

  it("a stop with no end effect sends no cause and leaves no notice", async () => {
    h.ended = undefined;
    await useTableStore.getState().dismissBuff("b");
    expect(h.calls).toEqual([{ id: "b", cause: undefined }]);
    expect(useTableStore.getState().notice).toBeNull();
  });
});
