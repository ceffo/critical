import { useCharacterStore } from "@/stores/character";
import type { Character } from "@/types/character";

/**
 * Whether the Skills activity has something to settle (ADR 0037 step 15, amended): a weapon or
 * non-weapon slot, or a skill point, still to spend, or a pool over budget. A reading of the
 * server's figures (`derived.levelUp.toSpend` and each pool's `left`, over budget when below 0);
 * nothing is computed here. A free spell owed is spent in the Spells activity, not here.
 */
export function skillsPending(character: Character | null): boolean {
  if (!character) return false;
  const { levelUp, proficiencySlots: slots, skillSets } = character.derived;
  const grants = levelUp.toSpend;
  if (
    grants.weaponSlots > 0 ||
    grants.nonWeaponSlots > 0 ||
    grants.skillPoints > 0
  )
    return true;
  const lefts = [
    slots?.weapon?.left,
    slots?.nonWeapon.left,
    ...skillSets.map((s) => s.budget?.left),
  ];
  return lefts.some((left) => left !== undefined && left !== 0);
}

/** Whether one section of the Skills activity has something to settle: `weapon`, `nonWeapon` or a
 * skill set's code. The same reading as `skillsPending`, a pool at a time; the level-up's grants name
 * a pool for the slots only (a skill point's grant names no set, so a set is read by its budget). */
export function sectionPending(
  character: Character | null,
  section: "weapon" | "nonWeapon" | { set: string },
): boolean {
  if (!character) return false;
  const { levelUp, proficiencySlots: slots, skillSets } = character.derived;
  const grants = levelUp.toSpend;
  const open = (left: number | undefined) => left !== undefined && left !== 0;
  if (section === "weapon")
    return grants.weaponSlots > 0 || open(slots?.weapon?.left);
  if (section === "nonWeapon")
    return grants.nonWeaponSlots > 0 || open(slots?.nonWeapon.left);
  return open(skillSets.find((s) => s.set === section.set)?.budget?.left);
}

/** `skillsPending` of the loaded character, for the rail's pill. */
export function useSkillsPending(): boolean {
  return useCharacterStore((s) => skillsPending(s.character));
}
