import { create } from "zustand";
import { fetchAttackForm, patchAttackForm } from "@/api/combat";
import type {
  AttackFormPatch,
  AttackFormView,
  AttackRequest,
} from "@/types/combat";
import { useRollStore } from "./roll";
import { useTableStore } from "./table";
import { useItemsStore } from "./items";

interface AttackStore {
  /** The server's attack form with its preview; null until loaded. */
  view: AttackFormView | null;
  /** Reload the form (after anything that can change the preview: buffs, items, weapons). */
  load: () => Promise<void>;
  /**
   * Save a change of the form. Target and situational changes show at once; the server's answer
   * (weapon, ammunition, preview) replaces the view once no newer change is on its way.
   */
  update: (patch: AttackFormPatch) => Promise<void>;
  /**
   * The point-blank option of a specialized launcher (ADR 0030 step 12): per attack, never saved;
   * it applies only while the preview offers it.
   */
  pointBlank: boolean;
  setPointBlank: (on: boolean) => void;
  /**
   * A thief's backstab option (ADR 0030 step 19): per attack, never saved; it applies only while
   * the preview offers it and is cleared once an attack is rolled with it.
   */
  backstab: boolean;
  setBackstab: (on: boolean) => void;
  /** Roll an attack with the saved choices. Resolves true when it was rolled. */
  attack: () => Promise<boolean>;
}

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

// Requests to the form run one after the other, in the order they were made; only the newest
// one's answer is shown, so an older answer never overwrites a newer change.
let queue: Promise<unknown> = Promise.resolve();
let issued = 0;

/** The attack form's choices are server state (ADR 0009 step 6); this store only mirrors them. */
export const useAttackStore = create<AttackStore>((set, get) => {
  function send(call: () => Promise<AttackFormView>): Promise<void> {
    const seq = ++issued;
    const next = queue.then(async () => {
      try {
        const view = await call();
        if (seq === issued) set({ view });
      } catch (e) {
        useTableStore.setState({ error: message(e) });
        // Show what the server kept.
        if (seq === issued)
          set({ view: await fetchAttackForm().catch(() => get().view) });
      }
    });
    queue = next;
    return next;
  }

  return {
    view: null,
    pointBlank: false,
    setPointBlank: (on) => set({ pointBlank: on }),
    backstab: false,
    setBackstab: (on) => set({ backstab: on }),
    load: () => send(fetchAttackForm),
    update: (patch) => {
      const view = get().view;
      if (view) {
        const { targetAc, targetSize, situational } = patch;
        set({
          view: {
            ...view,
            form: {
              ...view.form,
              ...(targetAc !== undefined && { targetAc }),
              ...(targetSize !== undefined && { targetSize }),
              ...(situational !== undefined && { situational }),
            },
          },
        });
      }
      // ADR 0033 step 8: picking a weapon that is not equipped equips it on the server, a logged
      // move: the journal and the inventory are read again once the form is saved.
      const equips =
        patch.weaponId !== undefined &&
        useItemsStore
          .getState()
          .weapons.some((w) => w.id === patch.weaponId && !w.wielded);
      const saved = send(() => patchAttackForm(patch));
      return equips
        ? saved.then(() => useTableStore.getState().refresh())
        : saved;
    },
    attack: async () => {
      await queue;
      let view = get().view;
      if (!view?.preview) return false;
      // The first arrow offered becomes the choice, so it stays chosen once spent.
      if (view.ammoFromDefault && view.form.ammoItemId) {
        await get().update({ ammoItemId: view.form.ammoItemId });
        view = get().view;
        if (!view?.preview) return false;
      }
      const { form } = view;
      const req: AttackRequest = {
        weapon_id: form.weaponId,
        target_size: form.targetSize,
      };
      // No AC: the attack is against an unknown AC (docs/adr/0012).
      if (form.targetAc !== null) req.target_ac = form.targetAc;
      if (form.ammoItemId) req.ammo_item_id = form.ammoItemId;
      if (form.situational.value !== 0) req.situational = form.situational;
      if (get().pointBlank && view.preview.pointBlank) req.point_blank = true;
      if (get().backstab && view.preview.backstab) req.backstab = true;
      const ok = await useRollStore.getState().rollAttack(req);
      if (ok && req.backstab) set({ backstab: false });
      await get().load();
      return ok;
    },
  };
});
