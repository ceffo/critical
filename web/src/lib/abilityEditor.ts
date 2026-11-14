// The equipment editor's abilities (docs/adr/0049-item-abilities.md steps 4, 5 and 16): what the
// form holds for an item's abilities and how it turns them into the `abilities` list of the request
// body and back. The server validates every ability (limit kind and numbers, dice, the bounds) and
// names the field; this only parses what was typed. The editor builds the limit, the `dice` and
// `heal` and `buff` effects and the note (Q4); `spell` comes with its slice.
import { editLang } from "@/i18n/localized";
import type { BuffEffectKey, BuffEffects } from "@/types/buffs";
import type { LocalizedText } from "@/types/localized";
import type {
  AbilityLimitKind,
  ItemAbility,
  ItemAbilityEffect,
} from "@/types/items";

/** The limit kinds, in the order the form offers them. */
export const LIMIT_KINDS: readonly AbilityLimitKind[] = [
  "unlimited",
  "per_day",
  "charges",
  "with_item",
];

/** The save categories a dice effect may allow (the server rejects anything else). */
export const SAVE_CATEGORIES = [
  "para_poison",
  "rod_staff_wand",
  "petri_poly",
  "breath",
  "spell",
] as const;

/**
 * The modifiers a buff effect may carry from the editor: the powers' list plus the Strength effects
 * (ADR 0049 step 9: a potion of Strength is a buff; ADR 0004 step 11). The server judges the values.
 */
export const BUFF_EFFECT_KEYS = [
  "ac",
  "save_bonus",
  "to_hit",
  "damage",
  "weapon_buff",
  "multiplier",
  "spells_buff",
  "base_ac",
  "initiative",
  "str_bonus",
  "str_max",
] as const satisfies readonly BuffEffectKey[];
export type BuffModKey = (typeof BUFF_EFFECT_KEYS)[number];

/** One modifier row of a buff effect: its key and the value as typed. */
export interface ModDraft {
  key: BuffModKey;
  value: string;
}

/** The buff keys the editor has no field for (conditions, the STR gate): kept as stored. */
function keptConditions(b: BuffEffects | undefined): BuffEffects {
  const kept: BuffEffects = {};
  if (b?.min_str !== undefined) kept.min_str = b.min_str;
  if (b?.weapon_kind) kept.weapon_kind = b.weapon_kind;
  if (b?.without_magic_armour) kept.without_magic_armour = true;
  return kept;
}

/** One effect row of an ability as the form holds it: dice rolled, a heal (dice or full) or a buff. */
export interface DiceDraft {
  kind: "dice" | "heal" | "buff" | "spell";
  /** A spell effect's spell and caster level, kept as stored (the picker comes with its slice). */
  spellId: string;
  level: number;
  /** A buff's name; empty: the ability's, else the item's. */
  name: string;
  /** A buff's rounds as typed; 0: until dismissed. */
  rounds: string;
  /** A buff's modifiers. */
  mods: ModDraft[];
  /** A buff's stored conditions, which the form has no field for. */
  kept: BuffEffects;
  /** A heal that fills the gap to max HP: no dice (heal rows only). */
  full: boolean;
  dice: string;
  /** A save category; empty for none. */
  save: string;
  /** Kept as stored: the first editor has no field for it. */
  label: string;
}

/** One ability as the form holds it: numbers stay text until submit. */
export interface AbilityDraft {
  /** The stored id; empty for a new ability (the server gives it one). */
  id: string;
  /** The stored name map, kept so a save writes only the UI language and keeps the others. */
  names: LocalizedText;
  /** The name in the UI language; empty: the item's own name shows. */
  name: string;
  note: string;
  kind: AbilityLimitKind;
  max: string;
  left: string;
  cost: string;
  effects: DiceDraft[];
}

/** The ability's form for a stored one, its name as shown in the UI language `lang`. */
export function abilityDraft(a: ItemAbility, lang: string): AbilityDraft {
  const { kind, max, left, cost } = a.limit;
  return {
    id: a.id,
    names: a.name ?? {},
    name: (a.name ?? {})[editLang(lang)] ?? "",
    note: a.note ?? "",
    kind,
    max: max !== undefined ? String(max) : "1",
    left: left !== undefined ? String(left) : "1",
    cost: cost !== undefined && cost > 0 ? String(cost) : "1",
    effects: a.effects.map((e) => ({
      kind: e.kind,
      spellId: e.spellId ?? "",
      level: e.level ?? 0,
      name: e.name ?? "",
      rounds: String(e.rounds ?? 0),
      mods: BUFF_EFFECT_KEYS.flatMap((key) =>
        e.buff?.[key] !== undefined
          ? [{ key, value: String(e.buff[key]) }]
          : [],
      ),
      kept: keptConditions(e.buff),
      full: e.full === true,
      dice: e.dice ?? "",
      save: e.save ?? "",
      label: e.label ?? "",
    })),
  };
}

/** A new effect row: dice rolled, a heal, or a buff (10 rounds of saves +1, to edit). */
export function newEffectDraft(kind: DiceDraft["kind"]): DiceDraft {
  return {
    kind,
    spellId: "",
    level: 0,
    name: "",
    rounds: kind === "buff" ? "10" : "0",
    mods: kind === "buff" ? [{ key: "save_bonus", value: "1" }] : [],
    kept: {},
    full: false,
    dice: "",
    save: "",
    label: "",
  };
}

/** A number as typed; NaN for anything else (including empty). */
function parseNumber(s: string): number {
  const text = s.trim().replace(",", ".");
  return /^[+-]?\d+(\.\d+)?$/.test(text) ? Number(text) : Number.NaN;
}

/** The form's drafts for a piece's abilities (none: not converted yet, shown as empty). */
export function abilityDrafts(
  abilities: readonly ItemAbility[] | undefined,
  lang: string,
): AbilityDraft[] {
  return (abilities ?? []).map((a) => abilityDraft(a, lang));
}

/** A new ability: a per-day one unless the piece is a stock (then with the item). */
export function newAbilityDraft(
  kind: AbilityLimitKind = "per_day",
): AbilityDraft {
  return {
    id: "",
    names: {},
    name: "",
    note: "",
    kind,
    max: "1",
    left: "1",
    cost: "1",
    effects: [],
  };
}

/** A whole number as typed; NaN for anything else (including empty). */
function parseInteger(s: string): number {
  const text = s.trim();
  return /^\d+$/.test(text) ? Number(text) : Number.NaN;
}

/** A buff effect's modifiers as sent: the typed rows and the kept conditions; null when a value is no number. */
function buffOf(e: DiceDraft): BuffEffects | null {
  const buff: BuffEffects = { ...e.kept };
  for (const m of e.mods) {
    const v = parseNumber(m.value);
    if (Number.isNaN(v)) return null;
    buff[m.key] = v;
  }
  return buff;
}

/**
 * The abilities list a form sends: the drafts in order; null when a number typed is not a whole
 * number. The name is written in the UI language `lang` only, the stored other languages are kept
 * (the list replaces the stored one as a whole). A dice effect with no dice typed is not sent.
 */
export function draftsToAbilities(
  drafts: readonly AbilityDraft[],
  lang: string,
): ItemAbility[] | null {
  const out: ItemAbility[] = [];
  for (const d of drafts) {
    const limit: ItemAbility["limit"] = { kind: d.kind };
    if (d.kind === "per_day" || d.kind === "charges") {
      limit.max = parseInteger(d.max);
      limit.left = parseInteger(d.left);
      if (d.kind === "charges") limit.cost = parseInteger(d.cost);
      if (
        Object.values(limit).some(
          (v) => typeof v === "number" && Number.isNaN(v),
        )
      )
        return null;
    }
    const names: Record<string, string> = { ...d.names };
    const typed = d.name.trim();
    if (typed) names[editLang(lang)] = typed;
    else delete names[editLang(lang)];
    // A row with no dice typed is not sent, except a full heal, which has none to type, and a buff,
    // which holds none: its numbers must be numbers.
    const effects: ItemAbilityEffect[] = [];
    for (const e of d.effects) {
      if (e.kind === "spell") {
        effects.push({ kind: "spell", spellId: e.spellId, level: e.level });
      } else if (e.kind === "buff") {
        const buff = buffOf(e);
        const rounds = parseInteger(e.rounds);
        if (!buff || Number.isNaN(rounds)) return null;
        effects.push({
          kind: "buff",
          rounds,
          buff,
          ...(e.name.trim() ? { name: e.name.trim() } : {}),
        });
      } else if ((e.kind === "heal" && e.full) || e.dice.trim() !== "") {
        effects.push(
          e.kind === "heal"
            ? {
                kind: "heal",
                ...(e.full ? { full: true } : { dice: e.dice.trim() }),
                ...(e.label ? { label: e.label } : {}),
              }
            : {
                kind: "dice",
                dice: e.dice.trim(),
                ...(e.save ? { save: e.save } : {}),
                ...(e.label ? { label: e.label } : {}),
              },
        );
      }
    }
    const a: ItemAbility = { id: d.id, limit, effects };
    if (Object.keys(names).length > 0) a.name = names;
    if (d.note.trim()) a.note = d.note.trim();
    out.push(a);
  }
  return out;
}

/**
 * A list of abilities in one canonical text (served flags dropped, absent and zero values alike),
 * to tell whether an edit changed it. An unconverted piece reads as an empty list.
 */
export function abilitiesKey(
  abilities: readonly ItemAbility[] | undefined,
): string {
  return JSON.stringify(
    (abilities ?? []).map((a) => ({
      id: a.id,
      name: Object.fromEntries(
        Object.entries(a.name ?? {}).sort(([x], [y]) => x.localeCompare(y)),
      ),
      note: a.note ?? "",
      limit: {
        kind: a.limit.kind,
        max: a.limit.max ?? 0,
        left: a.limit.left ?? 0,
        // The server serves cost 1 for a charges limit sent with none.
        cost: a.limit.cost && a.limit.cost > 0 ? a.limit.cost : 0,
      },
      effects: a.effects.map((e) => ({
        kind: e.kind,
        dice: e.dice ?? "",
        full: e.full === true,
        save: e.save ?? "",
        label: e.label ?? "",
        name: e.name ?? "",
        rounds: e.rounds ?? 0,
        spellId: e.spellId ?? "",
        level: e.level ?? 0,
        buff: Object.fromEntries(
          Object.entries(e.buff ?? {})
            .filter(([, v]) => v !== undefined && v !== false)
            .sort(([x], [y]) => x.localeCompare(y)),
        ),
      })),
    })),
  );
}
