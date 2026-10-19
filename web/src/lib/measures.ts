/**
 * Height and weight as the player's language shows them (ADR 0032 step 5). The server stores
 * centimetres and kilograms; fr shows them as they are, en as feet-inches and pounds. This is
 * display and typing only: every range and every rule is the server's.
 */

export type Units = "metric" | "imperial";

const CM_PER_INCH = 2.54;
const KG_PER_POUND = 0.45359237;

/** The units a language shows: en in feet-inches and pounds, every other language metric. */
export function unitsFor(language: string): Units {
  return /^en(-|$)/i.test(language) ? "imperial" : "metric";
}

/** A stored figure is kept to a tenth, as the server does. */
const tenth = (v: number) => Math.round(v * 10) / 10;

/** The number in the language's own notation (fr: 177,8), at most one decimal. */
function localNumber(v: number, language: string): string {
  return new Intl.NumberFormat(language, { maximumFractionDigits: 1 }).format(
    v,
  );
}

/** Feet and inches of a height in centimetres, rounded to the nearest inch. */
export function feetInches(cm: number): { feet: number; inches: number } {
  const total = Math.round(cm / CM_PER_INCH);
  return { feet: Math.floor(total / 12), inches: total % 12 };
}

/** A height as shown on the card: "177,8 cm" or `5'10"`. */
export function formatHeight(
  cm: number,
  units: Units,
  language: string,
): string {
  if (units === "metric") return `${localNumber(cm, language)} cm`;
  const { feet, inches } = feetInches(cm);
  return `${feet}'${inches}"`;
}

/** A weight as shown on the card: "72,6 kg" or "160 lb". */
export function formatWeight(
  kg: number,
  units: Units,
  language: string,
): string {
  return units === "metric"
    ? `${localNumber(kg, language)} kg`
    : `${Math.round(kg / KG_PER_POUND)} lb`;
}

/** A height as typed into a field of the units (the unit is in the label): "177,8" or `5'10"`. */
export function heightField(
  cm: number | null,
  units: Units,
  language: string,
): string {
  if (cm === null) return "";
  if (units === "metric") return localNumber(cm, language);
  const { feet, inches } = feetInches(cm);
  return `${feet}'${inches}"`;
}

/** A weight as typed into a field of the units: "72,6" or "160". */
export function weightField(
  kg: number | null,
  units: Units,
  language: string,
): string {
  if (kg === null) return "";
  return units === "metric"
    ? localNumber(kg, language)
    : String(Math.round(kg / KG_PER_POUND));
}

/** What a field holds: empty (null, which clears), a figure, or text that is no figure. */
export type Parsed = { value: number | null; invalid: boolean };

const EMPTY: Parsed = { value: null, invalid: false };
const INVALID: Parsed = { value: null, invalid: true };
const NUM = String.raw`(\d+(?:[.,]\d+)?)`;
const num = (s: string) => Number(s.replace(",", "."));

const FEET_INCHES = new RegExp(
  String.raw`^${NUM}\s*(?:'|′|ft)\s*(?:${NUM}\s*(?:"|″|''|in)?)?$`,
  "i",
);
const INCHES = new RegExp(String.raw`^${NUM}\s*(?:"|″|''|in)?$`, "i");
const FEET_SPACE_INCHES = new RegExp(String.raw`^(\d+)\s+${NUM}$`);
const PLAIN = new RegExp(String.raw`^${NUM}\s*(?:cm|kg|lbs?)?$`, "i");

/**
 * A height typed in the units: centimetres, or feet and inches (`5'10"`, `5 10`, `5'`; a lone
 * number is inches). Returns centimetres to a tenth.
 */
export function parseHeight(text: string, units: Units): Parsed {
  const s = text.trim();
  if (s === "") return EMPTY;
  let cm: number;
  if (units === "metric") {
    const m = PLAIN.exec(s);
    if (!m) return INVALID;
    cm = num(m[1]);
  } else {
    const fi = FEET_INCHES.exec(s) ?? FEET_SPACE_INCHES.exec(s);
    if (fi) {
      cm =
        (num(fi[1]) * 12 + (fi[2] === undefined ? 0 : num(fi[2]))) *
        CM_PER_INCH;
    } else {
      const inches = INCHES.exec(s);
      if (!inches) return INVALID;
      cm = num(inches[1]) * CM_PER_INCH;
    }
  }
  return { value: tenth(cm), invalid: false };
}

/** A weight typed in the units: kilograms or pounds. Returns kilograms to a tenth. */
export function parseWeight(text: string, units: Units): Parsed {
  const s = text.trim();
  if (s === "") return EMPTY;
  const m = PLAIN.exec(s);
  if (!m) return INVALID;
  const v = num(m[1]);
  return {
    value: tenth(units === "metric" ? v : v * KG_PER_POUND),
    invalid: false,
  };
}
