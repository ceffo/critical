// The equipment editor's powers (docs/adr/0033-equipment.md step 17): what the form holds for a
// piece's powers, the armour's or shield's "+N", and the ready-made rows. The server validates every
// power (as the seed's, ADR 0016 step 9) and decides what each one adds; this only turns what was
// typed into the powers list of the request body and back.
import type { BuffEffectKey, BuffEffects, WeaponKind } from "@/types/buffs";
import type { Category, ItemPowerDef } from "@/types/items";

/** The reserved id of an armour's or a shield's "+N" power (itemsvc MagicPowerID). */
export const MAGIC_POWER_ID = "magic";

/** The highest "+N" the server accepts (itemsvc MaxBonus). */
export const MAX_PLUS = 10;

/**
 * The effects a power may carry from the editor: ADR 0016 step 9's list plus base_ac (ADR 0033 step
 * 11). The Strength effects are not offered (a power never has one).
 */
export const POWER_EFFECT_KEYS = [
  "ac",
  "save_bonus",
  "to_hit",
  "damage",
  "weapon_buff",
  "multiplier",
  "spells_buff",
  "base_ac",
] as const satisfies readonly BuffEffectKey[];
export type PowerEffectKey = (typeof POWER_EFFECT_KEYS)[number];

/** Whether the piece's category has the "+N" field (ADR 0033 step 17: armour and shield). */
export function hasPlus(category: Category): boolean {
  return category === "armour" || category === "shield";
}

/** One effect row of a power in the form: its key and the value as typed. */
export interface EffectDraft {
  key: PowerEffectKey;
  value: string;
}

/** One power as the form holds it: numbers stay text until submit. */
export interface PowerDraft {
  /** The stored id; empty for a new power (the server gives it one). */
  id: string;
  /** The trigger's STR (ADR 0016 step 10); empty: always on. */
  minStr: string;
  effects: EffectDraft[];
  /** The weapon-kind condition (ADR 0004 step 18); empty: every roll. */
  weaponKind: WeaponKind | "";
  /** The AC effects' condition (ADR 0033 step 12). */
  withoutMagicArmour: boolean;
}

/** The "+N" an item's powers hold (its magic power's ac -N), 0 for none. */
export function plusOf(powers: readonly ItemPowerDef[]): number {
  const magic = powers.find((p) => p.id === MAGIC_POWER_ID);
  return magic?.effects.ac !== undefined ? -magic.effects.ac : 0;
}

/** The magic power of a "+N", none for 0 (ADR 0033 step 17: one effect ac -N, no trigger). */
export function plusPower(n: number): ItemPowerDef[] {
  return n > 0 ? [{ id: MAGIC_POWER_ID, effects: { ac: -n } }] : [];
}

/** A stored power as the form holds it. */
export function powerDraft(p: ItemPowerDef): PowerDraft {
  const effects: EffectDraft[] = [];
  for (const key of POWER_EFFECT_KEYS) {
    const v = p.effects[key];
    if (v !== undefined) effects.push({ key, value: String(v) });
  }
  return {
    id: p.id,
    minStr: p.minStr !== undefined ? String(p.minStr) : "",
    effects,
    weaponKind: p.effects.weapon_kind ?? "",
    withoutMagicArmour: p.effects.without_magic_armour ?? false,
  };
}

/**
 * The form's power drafts for a piece: every power but an armour's or a shield's magic one, which
 * is its "+N" field.
 */
export function powerDrafts(
  category: Category,
  powers: readonly ItemPowerDef[],
): PowerDraft[] {
  return powers
    .filter((p) => !(hasPlus(category) && p.id === MAGIC_POWER_ID))
    .map(powerDraft);
}

/** A new power with one effect. */
export function newPowerDraft(): PowerDraft {
  return {
    id: "",
    minStr: "",
    effects: [{ key: "ac", value: "-1" }],
    weaponKind: "",
    withoutMagicArmour: false,
  };
}

/** A number as typed; NaN for anything else (including empty). */
function parseNumber(s: string): number {
  const text = s.trim().replace(",", ".");
  return /^[+-]?\d+(\.\d+)?$/.test(text) ? Number(text) : Number.NaN;
}

/**
 * The powers list a form sends (ADR 0033 step 17): the "+N" first (armour and shield), then the
 * drafts in their order; null when a value typed is not a number. A power with no effect left is
 * not sent.
 */
export function draftsToPowers(
  category: Category,
  plus: number,
  drafts: readonly PowerDraft[],
): ItemPowerDef[] | null {
  const out: ItemPowerDef[] = hasPlus(category) ? plusPower(plus) : [];
  for (const d of drafts) {
    if (d.effects.length === 0) continue;
    const effects: BuffEffects = {};
    for (const e of d.effects) {
      const v = parseNumber(e.value);
      if (Number.isNaN(v)) return null;
      effects[e.key] = v;
    }
    if (d.weaponKind) effects.weapon_kind = d.weaponKind;
    if (d.withoutMagicArmour) effects.without_magic_armour = true;
    const p: ItemPowerDef = { id: d.id, effects };
    if (d.minStr.trim() !== "") {
      const s = parseNumber(d.minStr);
      if (!Number.isInteger(s)) return null;
      p.minStr = s;
    }
    out.push(p);
  }
  return out;
}

/** A power list in one canonical text (sorted keys), to tell whether an edit changed it. */
export function powersKey(powers: readonly ItemPowerDef[]): string {
  return JSON.stringify(
    powers.map((p) => ({
      id: p.id,
      minStr: p.minStr ?? null,
      effects: Object.fromEntries(
        Object.entries(p.effects)
          .filter(([, v]) => v !== undefined && v !== false)
          .sort(([a], [b]) => a.localeCompare(b)),
      ),
    })),
  );
}

/** The ready-made rows (ADR 0033 step 17): web constants of effect codes, not catalogue data. */
export const POWER_TEMPLATES = [
  "ring_of_protection",
  "cloak_of_protection",
  "bracers_of_defense",
] as const;
export type PowerTemplate = (typeof POWER_TEMPLATES)[number];

/**
 * The power a template gives for its N: a ring of protection +N is ac -N without magic armour and
 * saves +N (DMG: its save bonus counts over magic armour; the condition limits the AC effects only,
 * ADR 0033 step 12); a cloak of protection +N, ac -N and saves +N; bracers of defense AC N, base_ac N.
 */
export function templatePower(t: PowerTemplate, n: number): PowerDraft {
  const base = newPowerDraft();
  switch (t) {
    case "ring_of_protection":
      return {
        ...base,
        effects: [
          { key: "ac", value: String(-n) },
          { key: "save_bonus", value: String(n) },
        ],
        withoutMagicArmour: true,
      };
    case "cloak_of_protection":
      return {
        ...base,
        effects: [
          { key: "ac", value: String(-n) },
          { key: "save_bonus", value: String(n) },
        ],
      };
    case "bracers_of_defense":
      return { ...base, effects: [{ key: "base_ac", value: String(n) }] };
  }
}

/** The N a template starts with: +1 for protection, AC 6 for bracers (the player edits it). */
export function templateDefault(t: PowerTemplate): number {
  return t === "bracers_of_defense" ? 6 : 1;
}
