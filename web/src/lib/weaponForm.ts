// Input parsing for the weapon editor form. The server validates every value (and names the field);
// this only turns what was typed into the request body and flags text that is not a number.
import { editLang, localized } from "@/i18n/localized";
import { parseInteger, sentName } from "@/lib/itemForm";
import type { Weapon, WeaponBody } from "@/types/combat";

/**
 * What the form holds: numbers stay text until submit; the name in the UI language (ADR 0023 step
 * 11).
 */
export interface WeaponDraft {
  name: string;
  damage_sm: string;
  damage_l: string;
  magic_to_hit: string;
  magic_damage: string;
  kind: "melee" | "missile";
  two_handed: boolean;
  ammo_kind: string;
  /** ADR 0030 step 12: offered to a class that specializes. */
  specialized: boolean;
}

/** Fields of the form, named like the API's. */
export type WeaponField = keyof WeaponDraft;

export const emptyWeaponDraft: WeaponDraft = {
  name: "",
  damage_sm: "",
  damage_l: "",
  magic_to_hit: "0",
  magic_damage: "0",
  kind: "melee",
  two_handed: false,
  ammo_kind: "",
  specialized: false,
};

/** The form for a weapon, its name as shown in the UI language `lang`. */
export function draftFromWeapon(w: Weapon, lang: string): WeaponDraft {
  return {
    name: localized(lang, w.name),
    damage_sm: w.damage_sm,
    damage_l: w.damage_l,
    magic_to_hit: String(w.magic_to_hit),
    magic_damage: String(w.magic_damage),
    kind: w.kind,
    two_handed: w.two_handed,
    ammo_kind: w.ammo_kind,
    specialized: w.specialized,
  };
}

/** The number fields that are not numbers. */
export type WeaponDraftErrors = Partial<Record<WeaponField, "notANumber">>;

/**
 * The request body for a draft, and the number fields that are not numbers. The name is sent in the
 * UI language `lang` only (ADR 0023 step 11). A melee weapon sends no ammo kind (it has none to
 * fire).
 */
export function weaponDraftToBody(
  d: WeaponDraft,
  lang: string,
): {
  body: WeaponBody;
  errors: WeaponDraftErrors;
} {
  const errors: WeaponDraftErrors = {};
  const num = (field: WeaponField, text: string): number => {
    if (text.trim() === "") return 0;
    const n = parseInteger(text);
    if (Number.isNaN(n)) errors[field] = "notANumber";
    return n;
  };
  const body: WeaponBody = {
    name: { [editLang(lang)]: d.name },
    damage_sm: d.damage_sm,
    damage_l: d.damage_l,
    magic_to_hit: num("magic_to_hit", d.magic_to_hit),
    magic_damage: num("magic_damage", d.magic_damage),
    kind: d.kind,
    two_handed: d.two_handed,
    ammo_kind: d.kind === "missile" ? d.ammo_kind : "",
    specialized: d.specialized,
  };
  return { body, errors };
}

/**
 * The part of `body` that differs from the weapon (what a PATCH sends). Empty = nothing to save. The
 * name differs when it is not the one the form showed in the UI language `lang`.
 */
export function changedWeaponFields(
  w: Weapon,
  body: WeaponBody,
  lang: string,
): WeaponBody {
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(body)) {
    if (k === "name") {
      if (sentName(body) !== localized(lang, w.name)) out[k] = v;
    } else if ((w as unknown as Record<string, unknown>)[k] !== v) out[k] = v;
  }
  return out as WeaponBody;
}

/** Ammo kinds worth suggesting: the usual ones plus those already in use, each once. */
export function ammoKindSuggestions(
  weapons: Weapon[],
  itemKinds: string[],
): string[] {
  return [
    ...new Set(
      ["arrow", "bolt", ...weapons.map((w) => w.ammo_kind), ...itemKinds]
        .map((k) => k.trim())
        .filter((k) => k !== ""),
    ),
  ].sort();
}
