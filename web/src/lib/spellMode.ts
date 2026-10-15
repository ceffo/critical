// Wording of a spell's cast-time modes (ADR 0008 step 14); the server picks and rolls them.
import type { TFunction } from "i18next";
import type { SpellMode } from "@/types/spells";
import { damageText } from "@/lib/spellDice";

/** The mode's name: `spellMode.<spell id>.<key>`, else the key with spaces. */
export function modeName(t: TFunction, spellId: string, key: string): string {
  return t(`spellMode.${spellId}.${key}`, {
    defaultValue: key.replace(/_/g, " "),
  });
}

/** "Ice sheet: 3d10 cold", "Ice plane: no damage", in the player's language. */
export function modeText(
  t: TFunction,
  spellId: string,
  mode: SpellMode,
): string {
  return t("spellMode.withDamage", {
    mode: modeName(t, spellId, mode.key),
    damage: mode.damage ? damageText(t, mode.damage) : t("spellMode.noDamage"),
  });
}
