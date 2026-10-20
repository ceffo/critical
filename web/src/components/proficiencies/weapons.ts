import type {
  WeaponPickFigures,
  WeaponPickInput,
  WeaponTypeOption,
} from "@/types/character";

/** What a plain weapon pick spends (ADR 0037 step 7: one slot per weapon type). A specialization
 * spends its type's `specializationSlots` from the options (step 9); the web never computes it. */
export const PLAIN_WEAPON_SLOTS = 1;

/** One row of the weapon editor: a weapon type, the class's option for it (absent for a kept
 * type the class may not pick), its pick if any, and whether the class may pick it. */
export interface WeaponRow {
  type: string;
  option?: WeaponTypeOption;
  pick?: WeaponPickInput;
  allowed: boolean;
}

/**
 * The editor's rows: every type the class may pick, in the options' order, then each kept pick
 * outside that list (ADR 0037 step 7: kept, never added), so it can be seen and removed. A row is
 * allowed as the server's figures say (`allowed`), else when the class's list has it.
 */
export function weaponRows(
  types: WeaponTypeOption[],
  picks: WeaponPickInput[],
  figures: WeaponPickFigures[] | undefined,
): WeaponRow[] {
  const pickOf = (type: string) => picks.find((p) => p.type === type);
  const allowedOf = (type: string, listed: boolean) =>
    figures?.find((f) => f.type === type)?.allowed ?? listed;
  const listed = new Set(types.map((o) => o.type));
  return [
    ...types.map((o) => ({
      type: o.type,
      option: o,
      pick: pickOf(o.type),
      allowed: allowedOf(o.type, true),
    })),
    ...picks
      .filter((p) => !listed.has(p.type))
      .map((p) => ({
        type: p.type,
        pick: p,
        allowed: allowedOf(p.type, false),
      })),
  ];
}

/** The picks with type added as a plain pick, at the end (the player's order). */
export function addWeapon(
  picks: WeaponPickInput[],
  type: string,
): WeaponPickInput[] {
  if (picks.some((p) => p.type === type)) return picks;
  return [...picks, { type, slots: PLAIN_WEAPON_SLOTS }];
}

/** The picks without type. */
export function removeWeapon(
  picks: WeaponPickInput[],
  type: string,
): WeaponPickInput[] {
  return picks.filter((p) => p.type !== type);
}

/**
 * The picks with type specialized (`slots`, the options' specialization cost) or back to a plain
 * pick (a type not yet picked is added). A second specialization is the server's to refuse (step
 * 9): the editor shows its refusal on the row.
 */
export function specialize(
  picks: WeaponPickInput[],
  type: string,
  slots: number | null,
): WeaponPickInput[] {
  const next = slots ?? PLAIN_WEAPON_SLOTS;
  if (!picks.some((p) => p.type === type))
    return [...picks, { type, slots: next }];
  return picks.map((p) => (p.type === type ? { ...p, slots: next } : p));
}

/** Whether a pick is the specialization: the server takes a weapon pick of one slot (plain) or of
 * its type's specialization cost, never another count (ADR 0037 step 9). */
export function isSpecialization(pick: WeaponPickInput | undefined): boolean {
  return pick !== undefined && pick.slots > PLAIN_WEAPON_SLOTS;
}
