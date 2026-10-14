import type { CastResult, PreparationInfo, SpellSlot } from "../types/spells";
import { post, request } from "./http";

export function fetchSpellSlots(): Promise<SpellSlot[]> {
  return request<SpellSlot[]>("/api/spells/slots");
}

/** What a cast can carry besides the slot (all optional). */
export interface CastOptions {
  /** Whether a buff spell lands on the Bard (omitted: the spell's default). */
  onSelf?: boolean;
  /** The spell cast from a blank Rary's extra slot. */
  spellId?: string;
  /** Rary's Mnemonic Enhancer: the levels of the slots granted ([1,1,1], [1,2] or [3]). */
  extraSlots?: number[];
}

/** Spend slot `index`. */
export function castSpell(index: number, opts: CastOptions = {}): Promise<CastResult> {
  return post<CastResult>(`/api/spells/${index}/cast`, opts);
}

/** Apply a spell a party member cast on the Bard, at their caster level. */
export function receiveSpell(spellId: string, casterLevel: number): Promise<CastResult> {
  return post<CastResult>("/api/spells/receive", { spellId, casterLevel });
}

export function resetSpellSlots(): Promise<SpellSlot[]> {
  return post<SpellSlot[]>("/api/spells/reset");
}

export function fetchPreparationInfo(): Promise<PreparationInfo> {
  return request<PreparationInfo>("/api/spells/preparation");
}

/** Replace today's preparation with these spell ids (duplicates allowed). */
export function prepareSpells(spells: string[]): Promise<SpellSlot[]> {
  return request<SpellSlot[]>("/api/spells/prepared", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ spells }),
  });
}
