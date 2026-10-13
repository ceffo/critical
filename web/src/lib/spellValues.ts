// Display formatting of the values the server computed at a caster level; no game rules here.
import type { Spell, SpellValue } from "@/types/spells";

/** Short label of a unit ("yard" -> "yd", "cu foot" -> "cu ft"). */
export type UnitLabel = (word: string) => string;

/** "210 yd", "3 rd + 1d4", "1-5". */
export function formatValue(v: SpellValue, unit: UnitLabel): string {
  const parts: string[] = [];
  if (v.amount !== 0 || !v.dice) {
    parts.push(v.upper ? `${v.amount}-${v.upper}` : `${v.amount}`);
  }
  if (v.dice) parts.push(v.dice);
  const u = v.unit ? v.unit.split(" ").map(unit).join(" ") : "";
  return parts.join(" + ") + (u ? ` ${u}` : "");
}

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
  unit: UnitLabel,
): string | null {
  const v = spell.values;
  if (!v) return null;
  const st = spell.stats;
  switch (key) {
    case "range":
      return st.range.kind === "distance" ? formatValue(v.range, unit) : null;
    case "duration":
      return st.duration.kind === "timed"
        ? formatValue(v.duration, unit)
        : null;
    case "castingTime":
      return st.casting_time.kind === "timed"
        ? formatValue(v.casting_time, unit)
        : null;
    case "area": {
      const parts: string[] = [];
      if (v.area.targets) {
        parts.push(
          [formatValue(v.area.targets, unit), st.area.noun]
            .filter(Boolean)
            .join(" "),
        );
      }
      if (v.area.dims?.length) {
        parts.push(
          [v.area.dims.map((d) => formatValue(d, unit)).join(" × "), st.area.shape]
            .filter(Boolean)
            .join(" "),
        );
      }
      return parts.length ? parts.join(" · ") : null;
    }
  }
}

/** Raw book text of a stat. */
export function statRaw(spell: Spell, key: StatKey): string {
  switch (key) {
    case "range":
      return spell.range;
    case "duration":
      return spell.duration;
    case "area":
      return spell.area_of_effect;
    case "castingTime":
      return spell.casting_time;
    case "save":
      return spell.saving_throw;
  }
}
