import type {
  CastAttack,
  CastItem,
  CastResult,
  ExposureRequest,
  ExposureResult,
  PreparationInfo,
  Spell,
  SpellSlot,
} from "../types/spells";
import type { LearnResult } from "../types/character";
import { post, request } from "./http";

export function fetchSpellSlots(): Promise<SpellSlot[]> {
  return request<SpellSlot[]>("/api/spells/slots");
}

/**
 * TRANSITION ONLY (rolling deploy): a still-old machine answers a cast with the previous release's
 * names, entryId/source/name/consumedItem, `spell` as the catalogue spell and the attack's die as
 * d20Roll, where this release reads logEntryId/action/spell (the name)/item, catalogueSpell and d20.
 * Accept both here, at the boundary, so the rest of the web sees one shape. Remove with the
 * server's compat keys (castAnswer in casthdl) in the follow-up release.
 */
export function normalizeCastAnswer(raw: CastResult): CastResult {
  const old = raw as Partial<Omit<CastResult, "spell" | "attack">> & {
    spell?: string | Spell;
    attack?: Partial<CastAttack> & { d20Roll?: number };
    entryId?: number;
    source?: CastResult["action"];
    name?: string;
    consumedItem?: CastItem;
  };
  const oldSpell = typeof old.spell === "object" ? old.spell : undefined;
  const attack = old.attack
    ? ({
        ...old.attack,
        d20: old.attack.d20 ?? old.attack.d20Roll ?? 0,
      } as CastAttack)
    : undefined;
  return {
    ...raw,
    action: old.action ?? old.source ?? "cast",
    spell: typeof old.spell === "string" ? old.spell : (old.name ?? ""),
    logEntryId: old.logEntryId ?? old.entryId ?? 0,
    item: old.item ?? old.consumedItem,
    catalogueSpell: old.catalogueSpell ?? oldSpell,
    attack,
  };
}

/** TRANSITION ONLY: as normalizeCastAnswer, an old machine's entryId/name for logEntryId/spell. */
export function normalizeExposureAnswer(raw: ExposureResult): ExposureResult {
  const old = raw as Partial<ExposureResult> & {
    entryId?: number;
    name?: string;
  };
  return {
    ...raw,
    logEntryId: old.logEntryId ?? old.entryId ?? 0,
    spell: old.spell ?? old.name ?? "",
  };
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
  return post<CastResult>(`/api/spells/${index}/cast`, opts).then(
    normalizeCastAnswer,
  );
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
  }).then(normalizeCastAnswer);
}

/** Add a spell that adds a buff as an effect on the Bard, at his level (no slot, no damage). */
export function addSpellEffect(spellId: string): Promise<CastResult> {
  return post<CastResult>("/api/spells/effect", { spellId }).then(
    normalizeCastAnswer,
  );
}

/** Roll one round of exposure to a spell's area (logged; on the Bard, off his HP). */
export function rollExposure(req: ExposureRequest): Promise<ExposureResult> {
  return post<ExposureResult>("/api/spells/exposure", req).then(
    normalizeExposureAnswer,
  );
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

/** Roll the chance to learn `spellId` on the server (ADR 0030 step 20); a refusal is an ApiError. */
export function learnSpell(spellId: string): Promise<LearnResult> {
  return post<LearnResult>("/api/character/spellbook/learn", { spellId });
}
