import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AttackFormPatch, AttackFormView } from "@/types/combat";

const calls: { patch?: AttackFormPatch; resolve: (v: AttackFormView) => void }[] = [];

function view(targetAc: number | null): AttackFormView {
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
import { useRollStore } from "./roll";

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

  it("an attack with the offered first arrow makes it the choice before rolling with it", async () => {
    const order: string[] = [];
    const rollAttack = vi.fn(async () => {
      order.push("attack");
      return true;
    });
    useRollStore.setState({ rollAttack });
    const bow = (ammoFromDefault: boolean): AttackFormView => ({
      ...view(5),
      form: { ...view(5).form, weaponId: "shortbow", ammoItemId: "arrows_plus1" },
      ammoFromDefault,
      preview: { baseThac0: 13, modifiers: [], effectiveThac0: 9, toHit: 4, rollNeeded: 4, damageBonus: 2 },
    });
    useAttackStore.setState({ view: bow(true) });

    const done = useAttackStore.getState().attack();
    await tick();
    expect(calls).toHaveLength(1);
    expect(calls[0].patch).toEqual({ ammoItemId: "arrows_plus1" });
    expect(rollAttack).not.toHaveBeenCalled();
    order.push("patch");
    calls[0].resolve(bow(false));
    await tick();
    expect(rollAttack).toHaveBeenCalledWith({
      weapon_id: "shortbow",
      ammo_item_id: "arrows_plus1",
      target_ac: 5,
      target_size: "sm",
    });
    expect(order).toEqual(["patch", "attack"]);
    await tick();
    calls[1].resolve(bow(false)); // the reload after the attack
    expect(await done).toBe(true);
  });

  it("an unknown target AC is sent as no target_ac (docs/adr/0012 step 1)", async () => {
    const rollAttack = vi.fn(async () => true);
    useRollStore.setState({ rollAttack });
    useAttackStore.setState({
      view: {
        ...view(null),
        preview: { baseThac0: 13, modifiers: [], effectiveThac0: 11, toHit: 2, rollNeeded: null, damageBonus: 2 },
      },
    });
    const done = useAttackStore.getState().attack();
    await tick();
    expect(rollAttack).toHaveBeenCalledWith({ weapon_id: "longsword", target_size: "sm" });
    calls[0].resolve(view(null));
    expect(await done).toBe(true);
  });

  it("Unknown shows at once and sends targetAc null", async () => {
    const done = useAttackStore.getState().update({ targetAc: null });
    expect(useAttackStore.getState().view?.form.targetAc).toBeNull();
    await tick();
    expect(calls[0].patch).toEqual({ targetAc: null });
    calls[0].resolve(view(null));
    await done;
  });
});

