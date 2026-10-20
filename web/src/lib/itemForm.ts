// Input parsing for the equipment editor form (one editor for every piece, ADR 0033 step 1). The
// server validates every value (and names the field); this only turns what was typed into the
// request body and flags text that is not a number.
import { editLang, localized } from "@/i18n/localized";
import type {
  Category,
  Component,
  Item,
  ItemBehaviour,
  ItemBody,
  MagicKind,
} from "@/types/items";
import type { WeaponType } from "@/types/combat";
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
  /** The spell a magic scroll holds (ADR 0033 step 2), a catalogue id; empty for none. */
  spellId: string;
  behaviour: ItemBehaviour;
  count: string;
  maxCount: string;
  effectDice: string;
  effectSave: string;
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

/** Whether the piece may name the spell it holds: a magic scroll (ADR 0033 step 2). */
export function holdsSpell(
  d: Pick<ItemDraft, "category" | "magicKind">,
): boolean {
  return d.category === "magic" && d.magicKind === "scroll";
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
    spellId: "",
    behaviour: "stock",
    count: "1",
    maxCount: "1",
    effectDice: "",
    effectSave: "",
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
    baseAc: "10",
    skillColumn: "none",
    armorRow: "leather",
    shieldKind: "small",
    acMelee: "1",
    acMissile: "1",
    plus: 0,
    powers: [],
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
    spellId: i.spellId ?? "",
    behaviour: i.behaviour,
    count: String(i.count),
    maxCount: String(i.maxCount),
    effectDice: i.effectDice,
    effectSave: i.effectSave,
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
    baseAc: i.category === "armour" ? String(i.baseAc) : empty.baseAc,
    skillColumn: i.skillColumn || empty.skillColumn,
    armorRow: i.armorRow || empty.armorRow,
    shieldKind: i.shieldKind || empty.shieldKind,
    acMelee: i.category === "shield" ? String(i.acMelee) : empty.acMelee,
    acMissile: i.category === "shield" ? String(i.acMissile) : empty.acMissile,
    plus: plusOf(i.powers ?? []),
    powers: powerDrafts(i.category, i.powers ?? []),
  };
}

/** Parse a whole number; NaN for anything else (including empty). */
export function parseInteger(s: string): number {
  const text = s.trim();
  return /^[+-]?\d+$/.test(text) ? Number(text) : Number.NaN;
}

/** i18n key suffix per field that failed to parse as a number (a power's value: field powers). */
export type DraftErrors = Partial<Record<ItemField, "notANumber">>;

/**
 * The request body for a draft, and the number fields that are not numbers. The name is sent in the
 * UI language `lang` only (ADR 0023 step 11: the server keeps the other languages). The daily number
 * is sent for daily items only; the category's own fields only (a melee weapon fires nothing).
 */
export function draftToBody(
  d: ItemDraft,
  lang: string,
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
    behaviour: d.behaviour,
    count: num("count", d.count),
    effectDice: d.effectDice,
    effectSave: d.effectSave,
    notes: d.notes,
  };
  const out = body as Record<string, unknown>;
  for (const f of CATEGORY_FIELDS[d.category])
    out[f] = (all as Record<string, unknown>)[f];
  if (d.behaviour === "daily") body.maxCount = num("maxCount", d.maxCount);
  // Only a magic scroll holds a spell; another piece sends none and the server drops it.
  if (holdsSpell(d)) body.spellId = d.spellId;
  // Every piece has powers (ADR 0033 step 17); the "+N" is an armour's or a shield's.
  const powers = draftsToPowers(d.category, d.plus, d.powers);
  if (powers) body.powers = powers;
  else errors.powers = "notANumber";
  // Only the category's number fields count as errors.
  for (const f of Object.keys(errors) as ItemField[])
    if (
      !["count", "maxCount", "powers"].includes(f) &&
      !CATEGORY_FIELDS[d.category].includes(f)
    )
      delete errors[f];
  if (d.behaviour !== "daily") delete errors.maxCount;
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
    } else if (Array.isArray(v)) {
      const was = stored[k];
      if (!Array.isArray(was) || was.join("/") !== v.join("/")) out[k] = v;
    } else if (stored[k] !== v) out[k] = v;
  }
  // Becoming daily always sends the daily number (the old max of a stock item is not one).
  if (out.behaviour === "daily" && body.maxCount !== undefined)
    out.maxCount = body.maxCount;
  return out as ItemBody;
}

/** The one body a form's name holds. */
export function sentName(body: {
  name?: Readonly<Record<string, string>>;
}): string {
  return Object.values(body.name ?? {})[0] ?? "";
}
