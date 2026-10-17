import { create } from "zustand";
import type { AttackRequest, RollResult, SaveCategory } from "@/types/combat";
import type {
  CastResult,
  ExposureRequest,
  ExposureResult,
} from "@/types/spells";
import { rollExposure as postExposure } from "@/api/spells";
import { resolveAttack } from "@/api/combat";
import {
  adjustAttack,
  adjustSave,
  attackVerdict,
  clearAttackAc,
  rollSavingThrow,
} from "@/api/roll";
import { postUseItem } from "@/api/items";
import type { UseResult } from "@/types/items";
import type { Ability, AbilityCheck, SkillCheck } from "@/types/skills";
import {
  adjustAbility,
  adjustSkill,
  rollAbility,
  rollSkill,
} from "@/api/skills";
import { useTableStore } from "./table";

/** The last roll and what was asked for, so the result card can show its context. */
export type LastRoll =
  | { kind: "attack"; result: RollResult }
  | { kind: "save"; category: SaveCategory; result: RollResult }
  | { kind: "cast"; result: CastResult }
  | { kind: "item"; result: UseResult }
  | { kind: "skill"; result: SkillCheck }
  | { kind: "ability"; result: AbilityCheck }
  | { kind: "exposure"; result: ExposureResult };

interface RollStore {
  last: LastRoll | null;
  rolling: boolean;
  /** Resolves true when the attack was rolled (and logged). */
  rollAttack: (req: AttackRequest) => Promise<boolean>;
  rollSave: (category: SaveCategory) => Promise<void>;
  /**
   * Sets the adjustment of the latest save (log entry id; the server refuses any other); resolves
   * true on success. The recomputed save becomes the last roll.
   */
  adjustLastSave: (
    entryId: number,
    category: SaveCategory,
    adjustment: number,
    reason: string,
  ) => Promise<boolean>;
  /** Corrects the target AC of the latest attack (log entry id); resolves true on success. */
  adjustLastAttack: (entryId: number, targetAc: number) => Promise<boolean>;
  /** Records the GM's hit or miss on the latest attack while its AC is unknown; resolves true on success. */
  verdictLastAttack: (entryId: number, hit: boolean) => Promise<boolean>;
  /** Makes the AC of the latest attack unknown again. */
  clearLastAttackAc: (entryId: number) => Promise<boolean>;
  /** Uses one charge of an item (rolls its effect, logs it). Resolves true on success. */
  spendItem: (id: string) => Promise<boolean>;
  /** Rolls a proficiency or bard skill check (the server rolls and logs it); a non-zero adjustment moves its target. */
  rollSkill: (id: string, adjustment?: number) => Promise<boolean>;
  /** Sets the adjustment of the latest skill check (log entry id; the server refuses any other). */
  adjustLastSkill: (
    entryId: number,
    adjustment: number,
    reason: string,
  ) => Promise<boolean>;
  /** Rolls a check of the ability (the server rolls it against the score in effect and logs it). */
  rollAbility: (ability: Ability) => Promise<boolean>;
  /** Sets the adjustment of the latest ability check (log entry id; the server refuses any other). */
  adjustLastAbility: (
    entryId: number,
    adjustment: number,
    reason: string,
  ) => Promise<boolean>;
  /** Rolls one round of exposure to a spell's area (logged; on the Bard, off his HP). */
  rollExposure: (req: ExposureRequest) => Promise<boolean>;
}

export const useRollStore = create<RollStore>((set) => {
  async function roll(
    fn: () => Promise<LastRoll>,
    refresh: boolean,
  ): Promise<boolean> {
    set({ rolling: true });
    const table = useTableStore.getState();
    table.clearError();
    let ok = true;
    try {
      set({ last: await fn() });
    } catch (e) {
      ok = false;
      useTableStore.setState({
        error: e instanceof Error ? e.message : String(e),
      });
    } finally {
      set({ rolling: false });
    }
    // An attack is logged and may spend ammo: show the new log entry and counts.
    if (refresh) await table.refresh();
    return ok;
  }
  return {
    last: null,
    rolling: false,
    rollAttack: (req) =>
      roll(
        async () => ({ kind: "attack", result: await resolveAttack(req) }),
        true,
      ),
    spendItem: (id) =>
      roll(async () => ({ kind: "item", result: await postUseItem(id) }), true),
    rollSkill: (id, adjustment) =>
      roll(
        async () => ({
          kind: "skill",
          result: await rollSkill(id, adjustment ? { adjustment } : undefined),
        }),
        true,
      ),
    adjustLastSkill: (id, adjustment, reason) =>
      roll(
        async () => ({
          kind: "skill",
          result: await adjustSkill(id, adjustment, reason),
        }),
        true,
      ),
    rollAbility: (ability) =>
      roll(
        async () => ({ kind: "ability", result: await rollAbility(ability) }),
        true,
      ),
    adjustLastAbility: (id, adjustment, reason) =>
      roll(
        async () => ({
          kind: "ability",
          result: await adjustAbility(id, adjustment, reason),
        }),
        true,
      ),
    rollExposure: (req) =>
      roll(
        async () => ({ kind: "exposure", result: await postExposure(req) }),
        true,
      ),
    // A save is logged: refresh to show its entry.
    rollSave: async (category) => {
      await roll(
        async () => ({
          kind: "save",
          category,
          result: await rollSavingThrow(category),
        }),
        true,
      );
    },
    adjustLastAttack: (id, targetAc) =>
      roll(
        async () => ({
          kind: "attack",
          result: await adjustAttack(id, targetAc),
        }),
        true,
      ),
    clearLastAttackAc: (id) =>
      roll(
        async () => ({ kind: "attack", result: await clearAttackAc(id) }),
        true,
      ),
    verdictLastAttack: (id, hit) =>
      roll(
        async () => ({ kind: "attack", result: await attackVerdict(id, hit) }),
        true,
      ),
    adjustLastSave: (id, category, adjustment, reason) =>
      roll(
        async () => ({
          kind: "save",
          category,
          result: await adjustSave(id, adjustment, reason),
        }),
        true,
      ),
  };
});
