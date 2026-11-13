import type {
  WeaponOption,
  WeaponPickFigures,
  WeaponPickInput,
} from "@/types/character";

/** What a plain weapon pick spends (ADR 0037 step 21.1: one slot per catalogue weapon). A
 * specialization spends its weapon's `specializationSlots` from the options (step 21.5); the web
 * never computes it. */
export const PLAIN_WEAPON_SLOTS = 1;

/** One row of the weapon editor: a catalogue weapon code, the class's option for it (absent for a
 * kept weapon the class may not pick), its pick if any, and whether the class may pick it. */
export interface WeaponRow {
  code: string;
  option?: WeaponOption;
  pick?: WeaponPickInput;
  allowed: boolean;
}

/**
 * The editor's rows: every weapon the class may pick, in the options' order, then each kept pick
 * outside that list (ADR 0037 step 21.2: kept, never added), so it can be seen and removed. A row
 * is allowed as the server's figures say (`allowed`), else when the class's list has it.
 */
export function weaponRows(
  weapons: WeaponOption[],
  picks: WeaponPickInput[],
  figures: WeaponPickFigures[] | undefined,
): WeaponRow[] {
  const pickOf = (code: string) => picks.find((p) => p.code === code);
  const allowedOf = (code: string, listed: boolean) =>
    figures?.find((f) => f.code === code)?.allowed ?? listed;
  const listed = new Set(weapons.map((o) => o.code));
  return [
    ...weapons.map((o) => ({
      code: o.code,
      option: o,
      pick: pickOf(o.code),
      allowed: allowedOf(o.code, true),
    })),
    ...picks
      .filter((p) => !listed.has(p.code))
      .map((p) => ({
        code: p.code,
        pick: p,
        allowed: allowedOf(p.code, false),
      })),
  ];
}

/** The picks with code added as a plain pick, at the end (the player's order). */
export function addWeapon(
  picks: WeaponPickInput[],
  code: string,
): WeaponPickInput[] {
  if (picks.some((p) => p.code === code)) return picks;
  return [...picks, { code, slots: PLAIN_WEAPON_SLOTS }];
}

/** The picks without code. */
export function removeWeapon(
  picks: WeaponPickInput[],
  code: string,
): WeaponPickInput[] {
  return picks.filter((p) => p.code !== code);
}

/**
 * The picks with code specialized (`slots`, the options' specialization cost) or back to a plain
 * pick (a weapon not yet picked is added). A second specialization is the server's to refuse (step
 * 21.5): the editor shows its refusal on the row.
 */
export function specialize(
  picks: WeaponPickInput[],
  code: string,
  slots: number | null,
): WeaponPickInput[] {
  const next = slots ?? PLAIN_WEAPON_SLOTS;
  if (!picks.some((p) => p.code === code))
    return [...picks, { code, slots: next }];
  return picks.map((p) => (p.code === code ? { ...p, slots: next } : p));
}

/** Whether a pick is the specialization: the server takes a weapon pick of one slot (plain) or of
 * its weapon's specialization cost, never another count (ADR 0037 step 21.5). */
export function isSpecialization(pick: WeaponPickInput | undefined): boolean {
  return pick !== undefined && pick.slots > PLAIN_WEAPON_SLOTS;
}
