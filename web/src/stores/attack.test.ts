import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AttackFormPatch, AttackFormView } from "@/types/combat";

const calls: { patch?: AttackFormPatch; resolve: (v: AttackFormView) => void }[] = [];

function view(targetAc: number): AttackFormView {
  return {
    form: {
      weaponId: "longsword", previousWeaponId: "", ammoItemId: "", targetAc, targetSize: "sm",
      situational: { value: 0, label: "" },
    },
    ammoFromDefault: false,
    ammoChoices: [],
    preview: null,
  };
}

vi.mock("@/api/combat", () => ({
  fetchAttackForm: () => new Promise((resolve) => calls.push({ resolve })),
  patchAttackForm: (patch: AttackFormPatch) => new Promise((resolve) => calls.push({ patch, resolve })),
  fetchWeapons: async () => [],
}));

import { useAttackStore } from "./attack";

const tick = () => new Promise((r) => setTimeout(r, 0));

describe("attack form store", () => {
  beforeEach(() => {
    calls.length = 0;
    useAttackStore.setState({ view: view(5) });
  });

  it("shows a target change at once and sends the changes one after the other", async () => {
    const store = useAttackStore.getState();
    const first = store.update({ targetAc: 6 });
    const second = store.update({ targetAc: 7 });
    expect(useAttackStore.getState().view?.form.targetAc).toBe(7);

    await tick();
    expect(calls).toHaveLength(1); // the second waits for the first
    calls[0].resolve(view(6));
    await tick();
    // An older answer does not undo the newer change shown.
    expect(useAttackStore.getState().view?.form.targetAc).toBe(7);
    expect(calls).toHaveLength(2);
    expect(calls[1].patch).toEqual({ targetAc: 7 });
    calls[1].resolve(view(7));
    await Promise.all([first, second]);
    expect(useAttackStore.getState().view?.form.targetAc).toBe(7);
  });
});
