// Input parsing for the equipment editor form (one editor for every piece, ADR 0033 step 1). The
// server validates every value (and names the field); this only turns what was typed into the
// request body and flags text that is not a number.
import { editLang, localized } from "@/i18n/localized";
import type {
  Category,
  Component,
  EquipmentRules,
  Item,
  ItemBody,
  MagicKind,
  StrengthRating,
} from "@/types/items";
import type { WeaponType } from "@/types/combat";
import { formatStrength } from "./format";
import {
  abilitiesKey,
  abilityDrafts,
  draftsToAbilities,
  type AbilityDraft,
} from "./abilityEditor";
import {
  draftsToPowers,
  plusOf,
  powerDrafts,
  powersKey,
  type PowerDraft,
} from "./powerEditor";

/**
 * What the form holds: numbers stay text until submit; the name in the UI language (ADR 0023 step
 * 11). The fields of every category are held; only the chosen category's are sent.
 */
export interface ItemDraft {
  name: string;
  category: Category;
  magicKind: MagicKind | "";
  /** The spell component it is, any category (ADR 0033 step 1.1); empty for none. */
  component: Component | "";
  /** The quantity as typed (ADR 0049 step 2), 0 to the rules' `maxQuantity`. */
  count: string;
  notes: string;
  // Ammunition.
  ammoKind: string;
  magicalBonus: string;
  // Weapon and ammunition.
  damageSm: string;
  damageL: string;
  damageTypes: string[];
  // Weapon.
  weaponType: WeaponType;
  weaponKind: "melee" | "missile";
  size: string;
  speed: string;
  magicToHit: string;
  magicDamage: string;
  twoHanded: boolean;
  fires: string;
  /** A bow's Strength rating as typed: 17, 18/50, 18/00; empty for none (ADR 0033 step 3). */
  strengthRating: string;
  // Armour.
  baseAc: string;
  skillColumn: string;
  armorRow: string;
  // Shield.
  shieldKind: string;
  acMelee: string;
  acMissile: string;
  // Powers (ADR 0033 step 17): an armour's or a shield's "+N", and the other powers.
  plus: number;
  powers: PowerDraft[];
  // Abilities (ADR 0049 step 4): the limit, the dice effect and the note of each.
  abilities: AbilityDraft[];
}

/** Fields of the form, named like the API's. */
export type ItemField = keyof ItemDraft;

/** The fields each category sends besides the shared ones (ADR 0033 steps 3 to 5). */
const CATEGORY_FIELDS: Record<Category, ItemField[]> = {
  weapon: [
    "damageSm",
    "damageL",
    "damageTypes",
    "weaponType",
    "weaponKind",
    "size",
    "speed",
    "magicToHit",
    "magicDamage",
    "twoHanded",
    "fires",
    "strengthRating",
  ],
  ammunition: [
    "ammoKind",
    "magicalBonus",
    "damageSm",
    "damageL",
    "damageTypes",
  ],
  armour: ["baseAc", "skillColumn", "armorRow"],
  shield: ["shieldKind", "acMelee", "acMissile"],
  magic: ["magicKind"],
  gear: [],
};

/**
 * Whether the piece has a quantity the player sets, as the server's rules say (ADR 0049 step 2,
 * GET /api/equipment/rules): the web holds no rule of its own. Before the rules are read it is false.
 */
export function hasQuantity(
  rules: EquipmentRules | null,
  d: Pick<ItemDraft, "category" | "magicKind">,
): boolean {
  const c = rules?.categories.find((r) => r.category === d.category);
  if (!c) return false;
  if (c.magicKinds.length === 0) return c.hasQuantity;
  return c.magicKinds.find((k) => k.kind === d.magicKind)?.hasQuantity ?? false;
}

/**
 * The weapon types of the bows, which may carry a Strength rating (ADR 0033 step 3). A copy of
 * the server's list, `bowTypes` behind itemsvc.IsBowType (api/internal/domain/service/itemsvc/
 * categories.go): change both together. Only which field the form offers depends on it; the
 * server refuses a rating on any other piece. A composite bow is typed as its short or long bow; a
 * crossbow is not one.
 */
const BOW_TYPES: readonly WeaponType[] = ["long_bow", "short_bow"];

/** Whether the form offers a Strength rating: a bow. */
export function ratesStrength(
  d: Pick<ItemDraft, "category" | "weaponType">,
): boolean {
  return d.category === "weapon" && BOW_TYPES.includes(d.weaponType);
}

/**
 * A typed Strength rating: "" is none (null); a score, or 18 with a two-digit exceptional part
 * (18/00 is 100). Undefined when the text is neither; the server judges the range.
 */
export function parseStrengthRating(
  text: string,
): StrengthRating | null | undefined {
  const s = text.trim();
  if (s === "") return null;
  const m = /^(\d{1,2})(?:\/(\d{2}))?$/.exec(s);
  if (!m) return undefined;
  const exceptional = m[2] === undefined ? 0 : Number(m[2]) || 100;
  return { score: Number(m[1]), exceptional };
}

/** A Strength rating as the form shows it: 17, 18/50, 18/00; empty for none. */
export function strengthRatingText(
  r: StrengthRating | null | undefined,
): string {
  return r ? formatStrength(r.score, r.exceptional) : "";
}

/** Whether the form shows (and sends) field for category. */
export function hasField(category: Category, field: ItemField): boolean {
  return CATEGORY_FIELDS[category].includes(field);
}

/** An empty form for a piece of category (gear unless given). */
export function emptyDraft(category: Category = "gear"): ItemDraft {
  return {
    name: "",
    category,
    magicKind: category === "magic" ? "misc" : "",
    component: "",
    count: "1",
    notes: "",
    ammoKind: "arrow",
    magicalBonus: "0",
    damageSm: "",
    damageL: "",
    damageTypes: [],
    weaponType: "",
    weaponKind: "melee",
    size: "",
    speed: "0",
    magicToHit: "0",
    magicDamage: "0",
    twoHanded: false,
    fires: "",
    strengthRating: "",
    baseAc: "10",
    skillColumn: "none",
    armorRow: "leather",
    shieldKind: "small",
    acMelee: "1",
    acMissile: "1",
    plus: 0,
    powers: [],
    abilities: [],
  };
}

/** The form for a piece, its name as shown in the UI language `lang`. */
export function draftFromItem(i: Item, lang: string): ItemDraft {
  const empty = emptyDraft(i.category);
  return {
    name: localized(lang, i.name),
    category: i.category,
    magicKind: i.magicKind,
    component: i.component ?? "",
    count: String(i.count),
    notes: i.notes,
    ammoKind: i.ammoKind || empty.ammoKind,
    magicalBonus: String(i.magicalBonus),
    damageSm: i.damageSm,
    damageL: i.damageL,
    damageTypes: [...i.damageTypes],
    weaponType: i.weaponType,
    weaponKind: i.weaponKind || "melee",
    size: i.size,
    speed: String(i.speed),
    magicToHit: String(i.magicToHit),
    magicDamage: String(i.magicDamage),
    twoHanded: i.twoHanded,
    fires: i.fires,
    strengthRating: strengthRatingText(i.strengthRating),
    baseAc: i.category === "armour" ? String(i.baseAc) : empty.baseAc,
    skillColumn: i.skillColumn || empty.skillColumn,
    armorRow: i.armorRow || empty.armorRow,
    shieldKind: i.shieldKind || empty.shieldKind,
    acMelee: i.category === "shield" ? String(i.acMelee) : empty.acMelee,
    acMissile: i.category === "shield" ? String(i.acMissile) : empty.acMissile,
    plus: plusOf(i.powers ?? []),
    powers: powerDrafts(i.category, i.powers ?? []),
    abilities: abilityDrafts(i.abilities, lang),
  };
}

/** Parse a whole number; NaN for anything else (including empty). */
export function parseInteger(s: string): number {
  const text = s.trim();
  return /^[+-]?\d+$/.test(text) ? Number(text) : Number.NaN;
}

/**
 * i18n key suffix per field that failed to parse (a power's value: field powers): not a number, or
 * not a Strength (17, 18/50).
 */
export type DraftErrors = Partial<
  Record<ItemField, "notANumber" | "notAStrength" | "notAQuantity">
>;

/**
 * The request body for a draft, and the number fields that are not numbers. The name is sent in the
 * UI language `lang` only (ADR 0023 step 11: the server keeps the other languages). The category's
 * own fields only (a melee weapon fires nothing). The abilities are sent as a whole list, which
 * replaces the stored one (ADR 0049 step 4). `maxQuantity` is the server's bound (the rules'), the
 * one a quantity above it is refused by here; left out, the server alone refuses.
 */
export function draftToBody(
  d: ItemDraft,
  lang: string,
  maxQuantity?: number,
): {
  body: ItemBody;
  errors: DraftErrors;
} {
  const errors: DraftErrors = {};
  const num = (field: ItemField, text: string, blankIsZero = false): number => {
    if (blankIsZero && text.trim() === "") return 0;
    const n = parseInteger(text);
    if (Number.isNaN(n)) errors[field] = "notANumber";
    return n;
  };
  const rating = (text: string): StrengthRating | null => {
    const r = parseStrengthRating(text);
    if (r === undefined) {
      errors.strengthRating = "notAStrength";
      return null;
    }
    return r;
  };
  const all: ItemBody = {
    magicKind: d.magicKind,
    ammoKind: d.ammoKind,
    magicalBonus: num("magicalBonus", d.magicalBonus, true),
    damageSm: d.damageSm,
    damageL: d.damageL,
    damageTypes: d.damageTypes,
    weaponType: d.weaponType,
    weaponKind: d.weaponKind,
    size: d.size,
    speed: num("speed", d.speed, true),
    magicToHit: num("magicToHit", d.magicToHit, true),
    magicDamage: num("magicDamage", d.magicDamage, true),
    twoHanded: d.twoHanded,
    fires: d.weaponKind === "missile" ? d.fires : "",
    // Only a bow is rated; any other weapon sends none, so a bow retyped drops its rating.
    strengthRating: ratesStrength(d) ? rating(d.strengthRating) : null,
    baseAc: num("baseAc", d.baseAc),
    skillColumn: d.skillColumn,
    armorRow: d.armorRow,
    shieldKind: d.shieldKind,
    acMelee: num("acMelee", d.acMelee, true),
    acMissile: num("acMissile", d.acMissile, true),
  };
  const body: ItemBody = {
    name: { [editLang(lang)]: d.name },
    category: d.category,
    component: d.component,
    count: num("count", d.count),
    notes: d.notes,
  };
  if (
    maxQuantity !== undefined &&
    body.count !== undefined &&
    body.count > maxQuantity
  )
    errors.count = "notAQuantity";
  const out = body as Record<string, unknown>;
  for (const f of CATEGORY_FIELDS[d.category])
    out[f] = (all as Record<string, unknown>)[f];
  // A scroll's spell is the spell effect of its ability (ADR 0049 step 9): the form sends the abilities
  // and the server keeps the old spell column in step.
  // Every piece has powers (ADR 0033 step 17); the "+N" is an armour's or a shield's.
  const powers = draftsToPowers(d.category, d.plus, d.powers);
  if (powers) body.powers = powers;
  else errors.powers = "notANumber";
  const abilities = draftsToAbilities(d.abilities, lang);
  if (abilities) body.abilities = abilities;
  else errors.abilities = "notANumber";
  // Only the category's number fields count as errors.
  for (const f of Object.keys(errors) as ItemField[])
    if (
      !["count", "powers", "abilities"].includes(f) &&
      !CATEGORY_FIELDS[d.category].includes(f)
    )
      delete errors[f];
  return { body, errors };
}

/**
 * The part of `body` that differs from the item (what a PATCH sends). Empty = nothing to save. The
 * name differs when it is not the one the form showed in the UI language `lang`, so an untouched
 * form never writes a fallback name into another language.
 */
export function changedFields(
  item: Item,
  body: ItemBody,
  lang: string,
): ItemBody {
  const out: Record<string, unknown> = {};
  const stored = item as unknown as Record<string, unknown>;
  for (const [k, v] of Object.entries(body)) {
    if (k === "name") {
      if (sentName(body) !== localized(lang, item.name)) out[k] = v;
    } else if (k === "powers") {
      if (powersKey(body.powers ?? []) !== powersKey(item.powers ?? []))
        out[k] = v;
    } else if (k === "abilities") {
      if (abilitiesKey(body.abilities) !== abilitiesKey(item.abilities))
        out[k] = v;
    } else if (k === "strengthRating") {
      const r = v as StrengthRating | null;
      if (strengthRatingText(r) !== strengthRatingText(item.strengthRating))
        out[k] = v;
    } else if (Array.isArray(v)) {
      const was = stored[k];
      if (!Array.isArray(was) || was.join("/") !== v.join("/")) out[k] = v;
    } else if (stored[k] !== v) out[k] = v;
  }
  return out as ItemBody;
}

/** The one body a form's name holds. */
export function sentName(body: {
  name?: Readonly<Record<string, string>>;
}): string {
  return Object.values(body.name ?? {})[0] ?? "";
}
