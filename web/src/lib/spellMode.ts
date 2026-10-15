// Wording of a spell's cast-time modes (ADR 0008 step 14); the server picks and rolls them.
import type { TFunction } from "i18next";
import type { Spell, SpellDamage, SpellExposure, SpellMode } from "@/types/spells";
import { damageText, exposureDamageText } from "@/lib/spellDice";

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

/**
 * A spell's exposure damage in one line (ADR 0008 step 16): "1d10 poison per round of exposure",
 * or each mode with its damage ("Within 10 ft: 2d4 fire · Passing through: 2d6+15 fire").
 */
export function exposureText(t: TFunction, spellId: string, x: SpellExposure): string {
  if (x.modes && x.modes.length > 0) {
    return x.modes
      .map((m) =>
        t("spellMode.withDamage", {
          mode: modeName(t, spellId, m.key),
          damage: m.damage ? exposureDamageText(t, m.damage) : t("spellMode.noDamage"),
        }),
      )
      .join(" · ");
  }
  if (!x.damage) return "";
  // Dice steps already say "by round of exposure".
  if (x.damage.steps && x.damage.steps.length > 0) return exposureDamageText(t, x.damage);
  return t("spellDice.exposure", { damage: exposureDamageText(t, x.damage) });
}

/**
 * The exposure rows of a spell's lookup: one per exposure mode ("Exposure (Within 10 ft)"), else
 * one ("Exposure"); none without an exposure.
 */
export function exposureRows(
  t: TFunction,
  spell: Pick<Spell, "id" | "exposure">,
): { key: string; label: string; damage: SpellDamage }[] {
  const x = spell.exposure;
  if (!x) return [];
  if (x.modes) {
    return x.modes.flatMap((m) =>
      m.damage
        ? [
            {
              key: `exposure-${m.key}`,
              label: t("lookup.exposureMode", { mode: modeName(t, spell.id, m.key) }),
              damage: m.damage,
            },
          ]
        : [],
    );
  }
  return x.damage ? [{ key: "exposure", label: t("lookup.exposure"), damage: x.damage }] : [];
}
