// The equipment editor's abilities (docs/adr/0049-item-abilities.md steps 4, 5 and 16): what the
// form holds for an item's abilities and how it turns them into the `abilities` list of the request
// body and back. The server validates every ability (limit kind and numbers, dice, the bounds) and
// names the field; this only parses what was typed. The first editor builds the limit, the `dice`
// effect and the note (Q4).
import { editLang } from "@/i18n/localized";
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

/** One dice effect row of an ability as the form holds it. */
export interface DiceDraft {
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
      dice: e.dice,
      save: e.save ?? "",
      label: e.label ?? "",
    })),
  };
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
    const effects: ItemAbilityEffect[] = d.effects
      .filter((e) => e.dice.trim() !== "")
      .map((e) => ({
        kind: "dice",
        dice: e.dice.trim(),
        ...(e.save ? { save: e.save } : {}),
        ...(e.label ? { label: e.label } : {}),
      }));
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
        dice: e.dice,
        save: e.save ?? "",
        label: e.label ?? "",
      })),
    })),
  );
}
