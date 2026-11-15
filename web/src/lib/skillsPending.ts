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

/** `skillsPending` of the loaded character, for the rail's pill. */
export function useSkillsPending(): boolean {
  return useCharacterStore((s) => skillsPending(s.character));
}
