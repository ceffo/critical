import { useAbilityFocus } from "@/stores/abilityFocus";
import { useBuffsStore } from "@/stores/buffs";
import { useCharacterStore } from "@/stores/character";

/** The scores in effect, by ability code, as the character card shows them: the sheet's, Strength
 * with its active effects (the server's, ADR 0030 step 9). Empty before the character reads. */
export function useAbilityScores(): Record<string, number> {
  const character = useCharacterStore((s) => s.character);
  const strength = useBuffsStore((s) => s.strength);
  if (!character) return {};
  return {
    str: strength?.effective ?? character.str,
    dex: character.dex,
    con: character.con,
    int: character.int,
    wis: character.wis,
    cha: character.cha,
  };
}

/** Props that make a row point at its ability on the character card while it is hovered or holds
 * focus. */
export function useAbilityHighlight(ability: string | undefined) {
  const focus = useAbilityFocus((s) => s.focus);
  const clear = useAbilityFocus((s) => s.clear);
  return {
    onMouseEnter: () => focus(ability),
    onMouseLeave: clear,
    onFocus: () => focus(ability),
    onBlur: clear,
  };
}
