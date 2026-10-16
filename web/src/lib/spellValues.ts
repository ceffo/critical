// Display formatting of the values the server computed at a caster level; no game rules here.
import type { Spell, SpellValue } from "@/types/spells";
import { formatValue, shapedText, type StatLocale } from "@/lib/statText";

/** The five stats of the lookup's detail grid. */
export type StatKey = "range" | "duration" | "area" | "castingTime" | "save";

/**
 * The value at the caster level of a stat, or null when the server computed none
 * (touch, special, instantaneous, no caster level...). The save has no level value: its
 * typed kinds are returned as keys for the caller to label.
 */
export function statValue(
  spell: Spell,
  key: Exclude<StatKey, "save">,
  loc: StatLocale,
): string | null {
  const v = spell.values;
  if (!v) return null;
  const st = spell.stats;
  switch (key) {
    case "range":
      return st.range.kind === "distance" ? formatValue(loc, v.range) : null;
    case "duration":
      return st.duration.kind === "timed" ? formatValue(loc, v.duration) : null;
    case "castingTime":
      return st.casting_time.kind === "timed"
        ? formatValue(loc, v.casting_time)
        : null;
    case "area": {
      // A partial reading has no value (the server sends none; ADR 0021 step 8).
      if (st.area.partial) return null;
      const parts: string[] = [];
      if (v.area.targets) {
        parts.push(formatValue(loc, v.area.targets, st.area.noun || undefined));
      }
      if (v.area.dims?.length) {
        const dims = v.area.dims.map((d) => formatValue(loc, d)).join(" × ");
        const units = v.area.dims.map((d) => d.unit);
        parts.push(shapedText(loc, dims, st.area.shape, units) ?? dims);
      }
      return parts.length ? parts.join(" · ") : null;
    }
  }
}

/** A stat whose value at the caster level has dice to roll, and that roll as a composer formula. */
export interface StatDice {
  key: Exclude<StatKey, "save">;
  /** The server's value, worded ("8 + 1d8 rd"). */
  value: string;
  /** "1d8+8": the server's dice then its flat amount, for the dice composer (the server parses it). */
  formula: string;
}

/** The dice text and flat amount of a value as one formula; null without dice or for a range. */
export function valueFormula(v: SpellValue | undefined): string | null {
  if (!v?.dice || v.upper) return null;
  const dice = v.dice.replace(/\s+/g, "");
  return v.amount ? `${dice}${v.amount > 0 ? "+" : ""}${v.amount}` : dice;
}

/**
 * The values at the caster level that still have dice to roll (a duration of "8 + 1d8 rd", an
 * area of "15d6 creatures"), in the order of the detail grid. Only what the server computed.
 */
export function statDice(spell: Spell, loc: StatLocale): StatDice[] {
  const v = spell.values;
  if (!v) return [];
  const out: StatDice[] = [];
  const keys = ["range", "duration", "area", "castingTime"] as const;
  for (const key of keys) {
    const value = statValue(spell, key, loc);
    if (!value) continue;
    const served =
      key === "range"
        ? v.range
        : key === "duration"
          ? v.duration
          : key === "castingTime"
            ? v.casting_time
            : v.area.targets;
    const formula = valueFormula(served);
    if (formula) out.push({ key, value, formula });
  }
  return out;
}
