// Wording of a spell's cast-time modes (ADR 0008 step 14); the server picks and rolls them.
import type { TFunction } from "i18next";
import type { Spell, SpellDamage, SpellMode } from "@/types/spells";
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

/**
 * The damage rows of a spell's lookup: one per cast-time mode with damage ("Damage (Ice sheet)"),
 * else the spell's damage ("Damage"); none when a cast rolls none.
 */
export function damageRows(
  t: TFunction,
  spell: Pick<Spell, "id" | "damage" | "modes">,
): { key: string; label: string; damage: SpellDamage }[] {
  if (spell.modes) {
    return spell.modes.flatMap((m) =>
      m.damage
        ? [
            {
              key: `damage-${m.key}`,
              label: t("lookup.damageMode", {
                mode: modeName(t, spell.id, m.key),
              }),
              damage: m.damage,
            },
          ]
        : [],
    );
  }
  return spell.damage
    ? [{ key: "damage", label: t("lookup.damage"), damage: spell.damage }]
    : [];
}
