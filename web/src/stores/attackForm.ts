import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";
import type { TargetSize } from "@/types/combat";

/** What the attack form holds. Kept outside the panel so switching tabs doesn't reset it. */
export interface AttackFormState {
  weaponId: string;
  ammoId: string;
  targetAc: string;
  size: TargetSize;
  sitValue: string;
  sitLabel: string;
}

export const defaultAttackForm: AttackFormState = {
  weaponId: "",
  ammoId: "",
  targetAc: "5",
  size: "sm",
  sitValue: "0",
  sitLabel: "",
};

interface AttackFormStore {
  form: AttackFormState;
  set: (patch: Partial<AttackFormState>) => void;
}

/** Last attack-form choices, kept for the browser session (sessionStorage) and across tab switches. */
export const useAttackFormStore = create<AttackFormStore>()(
  persist(
    (set) => ({
      form: defaultAttackForm,
      set: (patch) => set((s) => ({ form: { ...s.form, ...patch } })),
    }),
    { name: "critical.attackForm", storage: createJSONStorage(() => sessionStorage) },
  ),
);
