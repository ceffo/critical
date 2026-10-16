// Wording of a spell's typed stats (ADR 0021 step 8): the server sends typed values (units, counts,
// per-level terms, kinds); the words, the number format and the length system are the UI
// language's. No game rule here: the values at a caster level are the server's, only converted to
// metres for display.
import type { TFunction } from "i18next";
import { labelKey, translateValue } from "@/lib/lookupLabels";
import type { StatKey } from "@/lib/spellValues";
import type {
  Spell,
  SpellQuantity,
  SpellStats,
  SpellTerm,
  SpellValue,
} from "@/types/spells";

/** How a language words stats: its `t`, its number format and its length system. */
export interface StatLocale {
  t: TFunction;
  lang: string;
  /** Lengths in metres (`lookup.lengths` is "metric"), else in the book's feet and yards. */
  metric: boolean;
}

export function statLocale(t: TFunction, lang: string): StatLocale {
  return { t, lang, metric: t("lookup.lengths") === "metric" };
}

/**
 * The book's units in the metric system: the factor to multiply by and the metric unit, the French
 * books' own convention (ADR 0021 step 8). A yard is a metre and a foot 0.3 m ("10 yards" is
 * "10 mètres", "30 feet" is "9 mètres"), a mile 1.5 km; an inch is 2.5 cm. Areas are the squares
 * (a square yard is a square metre); a cubic foot is 27 litres and a cubic yard a cubic metre.
 */
export const METRIC: Readonly<Record<string, [number, string]>> = {
  inch: [2.5, "cm"],
  foot: [0.3, "m"],
  yard: [1, "m"],
  mile: [1.5, "km"],
  "sq inch": [6.25, "sq cm"],
  "sq foot": [0.09, "sq m"],
  "sq yard": [1, "sq m"],
  "sq mile": [2.25, "sq km"],
  "cu inch": [15.625, "cu cm"],
  "cu foot": [27, "litre"],
  "cu yard": [1, "cu m"],
};

/** A volume unit, book or metric: its numbers keep significant digits, not a fixed decimal count. */
function isVolume(unit: string | undefined): boolean {
  return !!unit && (unit.startsWith("cu ") || unit === "litre");
}

/**
 * A number in the language's format: "4,5" in French, no grouping ("1000"). Lengths and areas
 * keep at most 2 decimals; a volume (`unit`) keeps 3 significant digits and every integer digit
 * ("156" cm³, "0,0156" m³, "1234" litres), so a small volume is not rounded away.
 */
export function formatNumber(
  loc: StatLocale,
  n: number,
  unit?: string,
): string {
  return new Intl.NumberFormat(
    loc.lang,
    isVolume(unit)
      ? {
          maximumSignificantDigits: 3,
          maximumFractionDigits: 0,
          roundingPriority: "morePrecision",
          useGrouping: false,
        }
      : { maximumFractionDigits: 2, useGrouping: false },
  ).format(n);
}

/**
 * Factor and unit a value in `unit` is shown in: converted to metric where the language is. The
 * French books do not round the converted number ("75 feet" is "22,5 mètres", "260 feet" "78
 * mètres"), but write a length under a metre in centimetres ("1 foot" is "30 cm", never "0,3 m";
 * ADR 0021 step 8). `largest` is the largest number shown in that unit (a range's upper bound).
 */
export function shownUnit(
  loc: StatLocale,
  unit: string,
  largest: number,
): [number, string] {
  const [f, shown] = (loc.metric && METRIC[unit]) || [1, unit];
  if (shown === "m" && largest > 0 && largest * f < 1) return [f * 100, "cm"];
  return [f, shown];
}

/**
 * A unit word for `count` ("rd", "rounds", "m²"), or null when the language has none for it
 * (a word the parser kept but that is no unit: the stat is then shown as the book's text).
 */
export function unitText(
  loc: StatLocale,
  unit: string,
  count: number,
): string | null {
  const key = `lookup.unitWords.${labelKey(unit)}`;
  return loc.t(key, { count, defaultValue: "" }) || null;
}

/** What is counted ("creature" -> "créatures"), or null when the language has no word for it. */
export function nounText(
  loc: StatLocale,
  noun: string,
  count: number,
): string | null {
  const key = labelKey(noun);
  for (const k of [key, key.replace(/s$/, "")]) {
    const w = loc.t(`lookup.nouns.${k}`, { count, defaultValue: "" });
    if (w) return w;
  }
  return null;
}

/** "{{value}} {{unit}}" in the language's order; a value without a unit stands alone. */
function withUnit(loc: StatLocale, value: string, unit: string | null): string {
  return unit ? loc.t("lookup.stat.withUnit", { value, unit }) : value;
}

/**
 * A value at the caster level ("210 yd", "210 m", "3 rounds + 1d4"). A length with dice is not
 * converted (dice cannot be scaled), and a unit the language has no word for is printed as is.
 */
export function formatValue(
  loc: StatLocale,
  v: SpellValue,
  noun?: string,
): string {
  const [f, unit] = v.dice
    ? [1, v.unit ?? ""]
    : shownUnit(loc, v.unit ?? "", Math.max(v.amount, v.upper ?? 0));
  const parts: string[] = [];
  if (v.amount !== 0 || !v.dice) {
    const a = formatNumber(loc, v.amount * f, unit);
    parts.push(v.upper ? `${a}-${formatNumber(loc, v.upper * f, unit)}` : a);
  }
  if (v.dice) parts.push(v.dice);
  // Dice make it several; "1,5" is singular in French, as Intl.PluralRules says.
  const count = v.dice ? 2 : (v.upper || v.amount) * f;
  const word = noun
    ? (nounText(loc, noun, count) ?? noun)
    : unit
      ? (unitText(loc, unit, count) ?? unit)
      : null;
  return withUnit(loc, parts.join(" + "), word);
}

/**
 * One term of a book quantity ("10 yd/level", "1d4+1 turns", "1 creature/2 levels"), or null when
 * a word in it has no translation. `noun` is what a count counts (an area's targets).
 */
function termText(
  loc: StatLocale,
  term: SpellTerm,
  noun?: string,
): string | null {
  if (term.per) return null; // "/recipient": a divisor the language has no word for
  const unit = term.unit ?? "";
  const [f, shown] = term.dice
    ? [1, unit]
    : shownUnit(loc, unit, Math.max(term.count, term.upper ?? 0));
  const n = (x: number) => formatNumber(loc, x * f, shown);
  let value: string;
  if (term.dice)
    value = term.count ? `${term.dice}+${n(term.count)}` : term.dice;
  else value = term.upper ? `${n(term.count)}-${n(term.upper)}` : n(term.count);
  const count = term.dice ? 2 : (term.upper || term.count) * f;
  let word: string | null = null;
  if (noun) {
    word = nounText(loc, noun, count);
    if (!word) return null;
  } else if (shown) {
    word = unitText(loc, shown, count);
    if (!word) return null;
  }
  const text = withUnit(loc, value, word);
  if (!term.per_level) return text;
  return (term.step ?? 1) > 1
    ? loc.t("lookup.stat.perLevels", { value: text, step: term.step })
    : loc.t("lookup.stat.perLevel", { value: text });
}

/** A book quantity: its terms, "up to" and its cap; null when a word has no translation. */
export function quantityText(
  loc: StatLocale,
  q: SpellQuantity,
  noun?: string,
): string | null {
  if (!q.terms?.length) return null;
  const terms = q.terms.map((tm) => termText(loc, tm, noun));
  if (terms.some((x) => x === null)) return null;
  let text = terms.join(" + ");
  if (q.up_to) text = loc.t("lookup.stat.upTo", { value: text });
  if (q.max) {
    const [f, shown] = shownUnit(loc, q.max_unit ?? "", q.max);
    const word = shown ? unitText(loc, shown, q.max * f) : null;
    if (shown && !word) return null;
    const max = withUnit(loc, formatNumber(loc, q.max * f, shown), word);
    text = loc.t("lookup.stat.max", { value: text, max });
  }
  return text;
}

/** The book's own "Special": the whole text is that one word. */
function isSpecialWord(raw: string): boolean {
  return labelKey(raw) === "special";
}

/** "Special", or "Special (8 turns)" when the book gives a number with it. */
function specialText(
  loc: StatLocale,
  field: "castingTime" | "duration",
  raw: string,
  time: SpellQuantity | undefined,
): string | null {
  const special = loc.t(`lookup.values.${field}.special`);
  if (isSpecialWord(raw)) return special;
  if (!time?.terms?.length) return null;
  const detail = quantityText(loc, time);
  return detail && loc.t("lookup.stat.specialDetail", { special, detail });
}

export function castingTimeText(
  loc: StatLocale,
  s: SpellStats["casting_time"],
): string | null {
  if (s.kind === "timed") return quantityText(loc, s.time);
  return specialText(loc, "castingTime", s.raw, s.time);
}

export function rangeText(
  loc: StatLocale,
  s: SpellStats["range"],
): string | null {
  switch (s.kind) {
    case "self":
    case "touch":
    case "unlimited":
      return loc.t(`lookup.stat.range.${s.kind}`);
    case "distance": {
      const d = quantityText(loc, s.distance);
      if (!d || !s.radius) return d;
      return loc.t("lookup.stat.shaped", {
        dims: d,
        shape: loc.t("lookup.shapes.radius"),
      });
    }
    default:
      return isSpecialWord(s.raw) ? loc.t("lookup.values.range.special") : null;
  }
}

/** An "until" condition ("dispelled"), or null when the language has no words for it. */
function conditionText(loc: StatLocale, condition: string): string | null {
  return (
    loc.t(`lookup.conditions.${labelKey(condition)}`, { defaultValue: "" }) ||
    null
  );
}

export function durationText(
  loc: StatLocale,
  s: SpellStats["duration"],
): string | null {
  switch (s.kind) {
    case "timed":
      return quantityText(loc, s.time);
    case "instantaneous":
    case "concentration":
      return loc.t(`lookup.stat.duration.${s.kind}`);
    case "permanent": {
      if (!s.condition) return loc.t("lookup.stat.duration.permanent");
      const c = conditionText(loc, s.condition);
      return (
        c && loc.t("lookup.stat.duration.permanentUntil", { condition: c })
      );
    }
    case "until": {
      const c = conditionText(loc, s.condition ?? "");
      return c && loc.t("lookup.stat.duration.until", { condition: c });
    }
    default:
      return specialText(loc, "duration", s.raw, s.time);
  }
}

/**
 * Extents with their shape, in the language's order ("rayon de 6 m"). A "square" of square units
 * ("10 sq ft square") is said once, by the unit.
 */
export function shapedText(
  loc: StatLocale,
  dims: string,
  shape: string | undefined,
  units: (string | undefined)[],
): string | null {
  if (
    !shape ||
    (shape === "square" && units.every((u) => u?.startsWith("sq ")))
  )
    return dims;
  const word = loc.t(`lookup.shapes.${shape}`, { defaultValue: "" });
  return word ? loc.t("lookup.stat.shaped", { dims, shape: word }) : null;
}

export function areaText(
  loc: StatLocale,
  s: SpellStats["area"],
): string | null {
  switch (s.kind) {
    case "area":
    case "targets": {
      if (s.partial) return null;
      const parts: string[] = [];
      if (s.dims?.length) {
        const dims = s.dims.map((d) => quantityText(loc, d));
        if (dims.some((x) => x === null)) return null;
        const units = s.dims.map((d) => d.terms[0]?.unit);
        const shaped = shapedText(loc, dims.join(" × "), s.shape, units);
        if (!shaped) return null;
        parts.push(shaped);
      }
      if (s.targets) {
        if (!s.noun) return null; // a bare count ("Creates 1d4+1"): what it counts is the book's
        const n = quantityText(loc, s.targets, s.noun);
        if (!n) return null;
        parts.push(n);
      }
      return parts.length ? parts.join(", ") : null;
    }
    default:
      // The caster, a creature touched...: the kind drops what the book says ("the caster's
      // hand"), so only a text the language lists is worded; the rest is the book's.
      return isSpecialWord(s.raw) ? loc.t("lookup.values.area.special") : null;
  }
}

export function componentsText(
  loc: StatLocale,
  s: SpellStats["components"],
): string | null {
  if (s.special) return null;
  const letters = [
    s.v && loc.t("lookup.stat.components.v"),
    s.s && loc.t("lookup.stat.components.s"),
    s.m && !s.m_optional && loc.t("lookup.stat.components.m"),
  ].filter(Boolean);
  if (!letters.length) return null;
  const text = letters.join(", ");
  return s.m_optional
    ? loc.t("lookup.stat.components.mOptional", { value: text })
    : text;
}

export function saveText(
  loc: StatLocale,
  s: SpellStats["save"],
): string | null {
  if (!s.kinds.length) return null;
  // "Special" with anything else ("None (special)") is the book's own wording.
  if (s.kinds.includes("special") && !isSpecialWord(s.raw)) return null;
  return s.kinds
    .map((k) => loc.t(`lookup.saveKinds.${k}`))
    .join(loc.t("lookup.stat.or"));
}

/** The six stat fields of the catalogue, by their wire names. */
export type StatField =
  | "casting_time"
  | "range"
  | "duration"
  | "area_of_effect"
  | "components"
  | "saving_throw";

/** A field worded from its typed value, or null where the language cannot word it. */
export function typedStatText(
  loc: StatLocale,
  stats: SpellStats,
  field: StatField,
): string | null {
  switch (field) {
    case "casting_time":
      return castingTimeText(loc, stats.casting_time);
    case "range":
      return rangeText(loc, stats.range);
    case "duration":
      return durationText(loc, stats.duration);
    case "area_of_effect":
      return areaText(loc, stats.area);
    case "components":
      return componentsText(loc, stats.components);
    case "saving_throw":
      return saveText(loc, stats.save);
  }
}

/** Where a stat's text comes from: its typed value, a listed label of the book's text, or the book. */
export type StatSource = "typed" | "label" | "book";

/** The catalogue's stat texts are the English books'. */
function isBookLanguage(lang: string): boolean {
  return lang.split("-")[0] === "en";
}

/** The label group (`lookup.values.<group>`) of a field's book texts; none for the components. */
const LABEL_GROUP: Record<StatField, StatKey | null> = {
  casting_time: "castingTime",
  range: "range",
  duration: "duration",
  area_of_effect: "area",
  components: null,
  saving_throw: "save",
};

/**
 * A stat's text in the UI language (ADR 0021 step 8). In the books' language, the book's text (its
 * common values labelled, as before). In another language, the typed value worded in it, with
 * lengths in its system; where the parser left the field untyped or the language has no words for
 * a part of it, the book's text, its common values labelled. Never shortened.
 */
export function statText(
  loc: StatLocale,
  spell: Pick<Spell, StatField | "stats">,
  field: StatField,
): { text: string; source: StatSource } {
  if (!isBookLanguage(loc.lang)) {
    const typed = typedStatText(loc, spell.stats, field);
    if (typed !== null) return { text: typed, source: "typed" };
  }
  const raw = spell[field];
  const group = LABEL_GROUP[field];
  const text = group ? translateValue(loc.t, group, raw) : raw;
  return { text, source: text === raw ? "book" : "label" };
}
