import type { LevelGrants } from "@/types/character";

/** Whether any of the counts is above 0 or a free spell is listed: a reading of the server's
 * numbers, nothing computed. */
export function hasGrants(g: LevelGrants | undefined): g is LevelGrants {
  return (
    !!g &&
    (g.weaponSlots > 0 ||
      g.nonWeaponSlots > 0 ||
      g.skillPoints > 0 ||
      g.freeSpells.length > 0)
  );
}
