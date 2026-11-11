import { parseDecimal } from "@/lib/decimalInput";
import type { CritRow } from "@/types/combat";
import type {
  RacialLift,
  Ruleset,
  SuccessEffect,
  WeaponLift,
} from "@/types/ruleset";

/** A crit row being edited: the inputs hold text so a half-typed number is not forced back. */
export interface CritDraft {
  from: string;
  to: string;
  multiplier: string;
}

export interface EffectDraft {
  proficiencyId: string;
  ac: string;
  rounds: string;
}

/** What the page edits before the one Save: every card's values, as the inputs hold them. */
export interface RulesetDraft {
  crit: CritDraft[];
  racial: RacialLift;
  weapon: WeaponLift;
  effects: EffectDraft[];
}

/** The parts of the ruleset that are saved by their own PUT; the house rules card holds two. */
export type PartKey = "crit" | "racial" | "weapon" | "effects";

export const toCritDrafts = (rows: CritRow[]): CritDraft[] =>
  rows.map((r) => ({
    from: String(r.from),
    to: String(r.to),
    multiplier: String(r.multiplier),
  }));

export const toEffectDrafts = (effects: SuccessEffect[]): EffectDraft[] =>
  effects.map((e) => ({
    proficiencyId: e.proficiencyId,
    ac: String(e.ac),
    rounds: String(e.rounds),
  }));

export const draftOf = (r: Ruleset): RulesetDraft => ({
  crit: toCritDrafts(r.critTable.rows),
  racial: r.racialLift,
  weapon: r.weaponLift,
  effects: toEffectDrafts(r.successEffects),
});

/** Empty or non-numeric text becomes NaN, which the server refuses with a message naming the row. */
export const num = (s: string): number =>
  s.trim() === "" ? Number.NaN : Number(s);

export const critRowsOf = (rows: CritDraft[]): CritRow[] =>
  rows.map((r) => ({
    from: num(r.from),
    to: num(r.to),
    multiplier: parseDecimal(r.multiplier),
  }));

export const effectsOf = (rows: EffectDraft[]): SuccessEffect[] =>
  rows.map((r) => ({
    proficiencyId: r.proficiencyId,
    ac: num(r.ac),
    rounds: num(r.rounds),
  }));

/** The parts whose draft differs from the last saved state. */
export function dirtyParts(
  draft: RulesetDraft,
  saved: RulesetDraft,
): PartKey[] {
  const keys: PartKey[] = ["crit", "racial", "weapon", "effects"];
  return keys.filter(
    (k) => JSON.stringify(draft[k]) !== JSON.stringify(saved[k]),
  );
}
