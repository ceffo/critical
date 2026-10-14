// Input parsing for the weapon editor form. The server validates every value (and names the field);
// this only turns what was typed into the request body and flags text that is not a number.
import { parseInteger } from "@/lib/itemForm";
import type { Weapon, WeaponBody } from "@/types/combat";

/** What the form holds: numbers stay text until submit. */
export interface WeaponDraft {
  name: string;
  damage_sm: string;
  damage_l: string;
  magic_to_hit: string;
  magic_damage: string;
  kind: "melee" | "missile";
  two_handed: boolean;
  ammo_kind: string;
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
};

export function draftFromWeapon(w: Weapon): WeaponDraft {
  return {
    name: w.name,
    damage_sm: w.damage_sm,
    damage_l: w.damage_l,
    magic_to_hit: String(w.magic_to_hit),
    magic_damage: String(w.magic_damage),
    kind: w.kind,
    two_handed: w.two_handed,
    ammo_kind: w.ammo_kind,
  };
}

/** The number fields that are not numbers. */
export type WeaponDraftErrors = Partial<Record<WeaponField, "notANumber">>;

/**
 * The request body for a draft, and the number fields that are not numbers. A melee weapon sends no
 * ammo kind (it has none to fire).
 */
export function weaponDraftToBody(d: WeaponDraft): {
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
    name: d.name,
    damage_sm: d.damage_sm,
    damage_l: d.damage_l,
    magic_to_hit: num("magic_to_hit", d.magic_to_hit),
    magic_damage: num("magic_damage", d.magic_damage),
    kind: d.kind,
    two_handed: d.two_handed,
    ammo_kind: d.kind === "missile" ? d.ammo_kind : "",
  };
  return { body, errors };
}

/** The part of `body` that differs from the weapon (what a PATCH sends). Empty = nothing to save. */
export function changedWeaponFields(w: Weapon, body: WeaponBody): WeaponBody {
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(body)) {
    if ((w as unknown as Record<string, unknown>)[k] !== v) out[k] = v;
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
