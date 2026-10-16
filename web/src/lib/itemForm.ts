// Input parsing for the item editor form. The server validates every value (and names the field);
// this only turns what was typed into the request body and flags text that is not a number.
import { editLang, localized } from "@/i18n/localized";
import type { Item, ItemBehaviour, ItemBody } from "@/types/items";

/**
 * What the form holds: numbers stay text until submit; the name in the UI language (ADR 0023 step
 * 11).
 */
export interface ItemDraft {
  name: string;
  type: string;
  behaviour: ItemBehaviour;
  count: string;
  maxCount: string;
  magicalBonus: string;
  ammoKind: string;
  effectDice: string;
  effectSave: string;
  notes: string;
}

/** Fields of the form, named like the API's. */
export type ItemField = keyof ItemDraft;

export const emptyDraft: ItemDraft = {
  name: "",
  type: "",
  behaviour: "stock",
  count: "1",
  maxCount: "1",
  magicalBonus: "0",
  ammoKind: "",
  effectDice: "",
  effectSave: "",
  notes: "",
};

/** The form for an item, its name as shown in the UI language `lang`. */
export function draftFromItem(i: Item, lang: string): ItemDraft {
  return {
    name: localized(lang, i.name),
    type: i.type,
    behaviour: i.behaviour,
    count: String(i.count),
    maxCount: String(i.maxCount),
    magicalBonus: String(i.magicalBonus),
    ammoKind: i.ammoKind,
    effectDice: i.effectDice,
    effectSave: i.effectSave,
    notes: i.notes,
  };
}

/** Parse a whole number; NaN for anything else (including empty). */
export function parseInteger(s: string): number {
  const text = s.trim();
  return /^[+-]?\d+$/.test(text) ? Number(text) : Number.NaN;
}

/** i18n key suffix per field that failed to parse as a number. */
export type DraftErrors = Partial<Record<ItemField, "notANumber">>;

/**
 * The request body for a draft, and the number fields that are not numbers. The name is sent in the
 * UI language `lang` only (ADR 0023 step 11: the server keeps the other languages). The daily number
 * is sent for daily items only.
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
  const body: ItemBody = {
    name: { [editLang(lang)]: d.name },
    type: d.type,
    behaviour: d.behaviour,
    count: num("count", d.count),
    magicalBonus: num("magicalBonus", d.magicalBonus, true),
    ammoKind: d.ammoKind,
    effectDice: d.effectDice,
    effectSave: d.effectSave,
    notes: d.notes,
  };
  if (d.behaviour === "daily") body.maxCount = num("maxCount", d.maxCount);
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
  for (const [k, v] of Object.entries(body)) {
    if (k === "name") {
      if (sentName(body) !== localized(lang, item.name)) out[k] = v;
    } else if ((item as unknown as Record<string, unknown>)[k] !== v)
      out[k] = v;
  }
  // Becoming daily always sends the daily number (the old max of a stock item is not one).
  if (out.behaviour === "daily" && body.maxCount !== undefined)
    out.maxCount = body.maxCount;
  return out as ItemBody;
}

/** Existing item types, each once, for the type field's suggestions. */
export function typeSuggestions(items: Item[]): string[] {
  return [
    ...new Set(items.map((i) => i.type.trim()).filter((t) => t !== "")),
  ].sort();
}

/** The one body a form's name holds. */
export function sentName(body: {
  name?: Readonly<Record<string, string>>;
}): string {
  return Object.values(body.name ?? {})[0] ?? "";
}
