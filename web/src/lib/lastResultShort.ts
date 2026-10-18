import type { TFunction } from "i18next";
import type { InventoryNamer } from "@/i18n/inventoryName";
import type { SpellNamer } from "@/i18n/spellName";
import { attackSettled } from "@/lib/attackAdjust";
import { rowValue, type RowValue } from "@/lib/filCard";
import {
  abilityCheckPayload,
  attackPayload,
  castPayload,
  dicePayload,
  exposurePayload,
  itemPayload,
  savePayload,
  skillCheckPayload,
  skillName,
  turnPayload,
  undeadName,
} from "@/lib/logSummary";
import type { LogEntry } from "@/types/log";

/** The verdict of a short result: a word in sentence case, coloured by its tone. */
export interface ShortVerdict {
  text: string;
  tone: "pass" | "fail" | "crit";
}

/**
 * The header's last result in short (ADR 0027 step 4.5): "<value> · <what> · <verdict>". The value
 * is the journal row's number, the what a word or a name, the verdict only for a roll that has one.
 * Built from the entry's structured payload; the journal keeps its full wording.
 */
export interface ShortResult {
  value: RowValue | null;
  what: string;
  verdict: ShortVerdict | null;
}

function verdict(
  t: TFunction,
  key: string,
  tone: ShortVerdict["tone"],
): ShortVerdict {
  return { text: t(`lastShort.verdict.${key}`), tone };
}

/** Every log kind has a short wording; the kinds without a roll are worded by their kind label. */
export function shortResult(
  e: LogEntry,
  t: TFunction,
  spellName: SpellNamer,
  names: InventoryNamer,
): ShortResult {
  const value = rowValue(e);
  const attack = attackPayload(e);
  if (attack) {
    const settled = attackSettled(attack);
    return {
      // The d20, as the card's roll; the damage is the result card's business.
      value: { text: String(attack.d20Roll), tone: "muted" },
      what: t("lastShort.attack"),
      verdict: !settled
        ? null
        : attack.hit
          ? verdict(
              t,
              attack.isCrit ? "crit" : "hit",
              attack.isCrit ? "crit" : "pass",
            )
          : verdict(t, "miss", "fail"),
    };
  }
  const save = savePayload(e);
  if (save)
    return {
      value,
      what: t("lastShort.save", {
        name: t(`saves.categories.${save.category}`, {
          defaultValue: save.category,
        }),
      }),
      verdict: save.passed
        ? verdict(t, "passed", "pass")
        : verdict(t, "failed", "fail"),
    };
  const skill = skillCheckPayload(e);
  if (skill)
    return {
      value,
      what: skillName(t, skill.id),
      verdict: skill.success
        ? verdict(t, "success", "pass")
        : verdict(t, "failure", "fail"),
    };
  const ability = abilityCheckPayload(e);
  if (ability)
    return {
      value,
      what: t(`attr.${ability.ability}`, { defaultValue: ability.ability }),
      verdict: ability.success
        ? verdict(t, "success", "pass")
        : verdict(t, "failure", "fail"),
    };
  const dice = dicePayload(e);
  if (dice) return { value, what: dice.formula, verdict: null };
  const item = itemPayload(e);
  if (item)
    return { value, what: names.item(item.itemId, item.name), verdict: null };
  const cast = castPayload(e);
  if (cast) {
    const spell = spellName(cast.spellId, cast.spell);
    return {
      value,
      what: spell,
      verdict: cast.failure?.failed
        ? verdict(t, "spellFailed", "fail")
        : cast.attack
          ? cast.attack.hit
            ? verdict(t, "hit", "pass")
            : verdict(t, "miss", "fail")
          : null,
    };
  }
  const exposure = exposurePayload(e);
  if (exposure)
    return {
      value,
      what: spellName(exposure.spellId, exposure.spell),
      verdict: null,
    };
  const turn = turnPayload(e);
  if (turn)
    return {
      value,
      what: t("lastShort.turn", { undead: undeadName(t, turn.undead) }),
      verdict: !turn.success
        ? verdict(t, "failure", "fail")
        : verdict(
            t,
            turn.effect === "destroy" ? "destroyed" : "turned",
            "pass",
          ),
    };
  if (e.kind === "hp_change")
    return {
      value,
      what: t(e.hpDelta < 0 ? "lastShort.damage" : "lastShort.heal"),
      verdict: null,
    };
  if (e.kind === "initiative")
    return { value, what: t("lastShort.initiative"), verdict: null };
  return { value, what: t(`log.kinds.${e.kind}`), verdict: null };
}
