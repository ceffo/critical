import { create } from "zustand";
import type { AttackRequest, SaveCategory } from "@/types/combat";
import type { ExposureRequest } from "@/types/spells";
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
import type { Ability } from "@/types/skills";
import {
  adjustAbility,
  adjustSkill,
  rollAbility,
  rollSkill,
} from "@/api/skills";
import type { LiveResult } from "@/journal/card";
import { useTableStore } from "./table";

/** The last roll and what was asked for, so the result card can show its context; its kind is the
 * log kind the roll is recorded as. */
export type LastRoll = LiveResult;

interface RollStore {
  last: LastRoll | null;
  rolling: boolean;
  /** Resolves true when the attack was rolled (and logged). */
  rollAttack: (req: AttackRequest) => Promise<boolean>;
  /** mindAffecting: the magic attacks the mind (ADR 0006 step 9.3); spellId: the spell the save is
   * against (step 10.2). Resolves true when the save was rolled (and logged). */
  rollSave: (
    category: SaveCategory,
    mindAffecting?: boolean,
    spellId?: string,
  ) => Promise<boolean>;
  /**
   * Sets the adjustment of the latest save (log entry id; the server refuses any other); resolves
   * true on success. The recomputed save becomes the last roll.
   */
  adjustLastSave: (
    entryId: number,
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
  rollSkill: (
    id: string,
    adjustment?: number,
    set?: string,
  ) => Promise<boolean>;
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
  // Every roll is logged (and an attack may spend ammo, a use a charge): the action module runs
  // it and reads again what the server says it changed.
  async function roll(fn: () => Promise<LastRoll>): Promise<boolean> {
    set({ rolling: true });
    try {
      return await useTableStore
        .getState()
        .run(async () => set({ last: await fn() }));
    } finally {
      set({ rolling: false });
    }
  }
  return {
    last: null,
    rolling: false,
    rollAttack: (req) =>
      roll(async () => ({ kind: "attack", result: await resolveAttack(req) })),
    spendItem: (id) =>
      roll(async () => ({ kind: "item_use", result: await postUseItem(id) })),
    rollSkill: (id, adjustment, set) =>
      roll(async () => ({
        kind: "skill_check",
        result: await rollSkill(
          id,
          adjustment ? { adjustment } : undefined,
          set,
        ),
      })),
    adjustLastSkill: (id, adjustment, reason) =>
      roll(async () => ({
        kind: "skill_check",
        result: await adjustSkill(id, adjustment, reason),
      })),
    rollAbility: (ability) =>
      roll(async () => ({
        kind: "ability_check",
        result: await rollAbility(ability),
      })),
    adjustLastAbility: (id, adjustment, reason) =>
      roll(async () => ({
        kind: "ability_check",
        result: await adjustAbility(id, adjustment, reason),
      })),
    rollExposure: (req) =>
      roll(async () => ({ kind: "exposure", result: await postExposure(req) })),
    rollSave: (category, mindAffecting = false, spellId) =>
      roll(async () => ({
        kind: "save",
        result: await rollSavingThrow(category, mindAffecting, spellId),
      })),
    adjustLastAttack: (id, targetAc) =>
      roll(async () => ({
        kind: "attack",
        result: await adjustAttack(id, targetAc),
      })),
    clearLastAttackAc: (id) =>
      roll(async () => ({ kind: "attack", result: await clearAttackAc(id) })),
    verdictLastAttack: (id, hit) =>
      roll(async () => ({
        kind: "attack",
        result: await attackVerdict(id, hit),
      })),
    adjustLastSave: (id, adjustment, reason) =>
      roll(async () => ({
        kind: "save",
        result: await adjustSave(id, adjustment, reason),
      })),
  };
});
