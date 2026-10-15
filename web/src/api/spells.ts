import type {
  CastResult,
  ExposureRequest,
  ExposureResult,
  PreparationInfo,
  SpellSlot,
} from "../types/spells";
import { post, request } from "./http";

export function fetchSpellSlots(): Promise<SpellSlot[]> {
  return request<SpellSlot[]>("/api/spells/slots");
}

/** What a cast can carry besides the slot (all optional). */
export interface CastOptions {
  /** Whether a buff spell lands on the Bard (omitted: the spell's default). */
  onSelf?: boolean;
  /** The spell cast from a blank extra slot. */
  spellId?: string;
  /** A spell that grants slots: one of its slot_splits (omitted: the first). */
  extraSlots?: number[];
  /** A spell that consumes an item: the item spent (omitted: the only one of its type left). */
  itemId?: string;
  /** A spell with an attack roll: the target's AC. */
  targetAc?: number;
  /** A spell with cast-time modes: the mode's key (omitted: the first). */
  mode?: string;
}

/** Spend slot `index`. */
export function castSpell(
  index: number,
  opts: CastOptions = {},
): Promise<CastResult> {
  return post<CastResult>(`/api/spells/${index}/cast`, opts);
}

/** Apply a spell a party member cast on the Bard, at their caster level (`mode`: the mode they picked). */
export function receiveSpell(
  spellId: string,
  casterLevel: number,
  mode?: string,
): Promise<CastResult> {
  return post<CastResult>("/api/spells/receive", {
    spellId,
    casterLevel,
    mode,
  });
}

/** Roll one round of exposure to a spell's area (logged; on the Bard, off his HP). */
export function rollExposure(req: ExposureRequest): Promise<ExposureResult> {
  return post<ExposureResult>("/api/spells/exposure", req);
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
