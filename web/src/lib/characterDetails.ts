import type { Character, CharacterDetails } from "@/types/character";
import {
  heightField,
  parseHeight,
  parseWeight,
  unitsFor,
  weightField,
} from "./measures";

/** The free-text fields, in display order (alignment and sex are codes picked from the server's lists;
 * height and weight are numbers shown per language, see `lib/measures`). */
export const TEXT_FIELDS = ["hair", "eyes"] as const;
export type TextField = (typeof TEXT_FIELDS)[number];

/** The form's state: everything as the text typed in the inputs; height and weight are typed in the
 * units of the language (`lib/measures`). */
export interface Draft {
  alignment: string;
  age: string;
  sex: string;
  height: string;
  weight: string;
  /** The exact figure (cm, kg) of a fresh roll that filled the field, which shows it rounded (whole
   * inches and pounds in en): kept while the field still shows that roll. */
  rolledHeight?: number;
  rolledWeight?: number;
  hair: string;
  eyes: string;
  /** The language catalogue codes picked (ADR 0032 step 4). */
  languages: string[];
}

/** The alignment and sex codes (ADR 0032): a stored value that is none (an old release's free text) is missing. */
export const ALIGNMENT_CODES = [
  "lawful_good",
  "neutral_good",
  "chaotic_good",
  "lawful_neutral",
  "true_neutral",
  "chaotic_neutral",
  "lawful_evil",
  "neutral_evil",
  "chaotic_evil",
];
export const SEX_CODES = ["male", "female"];

/** The code `v` is, "" when it is none. */
export function codeOrEmpty(v: string | undefined, codes: string[]): string {
  return v !== undefined && codes.includes(v) ? v : "";
}

export function toDraft(d: CharacterDetails, language = "fr"): Draft {
  const units = unitsFor(language);
  return {
    ...d,
    height: heightField(d.height_cm, units, language),
    weight: weightField(d.weight_kg, units, language),
    alignment: codeOrEmpty(d.alignment, ALIGNMENT_CODES),
    sex: codeOrEmpty(d.sex, SEX_CODES),
    age: d.age === null ? "" : String(d.age),
    languages: [...d.languages],
  };
}

/**
 * The PATCH body for a draft. Only the shape is checked here: an age that is not a whole number
 * from 1 up, or a height or weight that is no figure in the units of the language, is flagged for
 * the caller to report; a height or weight left as `toDraft` showed it keeps the `original` number; an empty age, height or weight is sent as null, which clears it. Every
 * limit is the server's.
 */
export function toBody(
  draft: Draft,
  language = "fr",
  original?: Pick<CharacterDetails, "height_cm" | "weight_kg">,
): {
  body: Partial<CharacterDetails>;
  ageInvalid: boolean;
  /** The measures typed that are no figure. */
  invalid: ("height" | "weight")[];
} {
  const age = draft.age.trim();
  const ageInvalid = age !== "" && !/^0*[1-9]\d*$/.test(age);
  const units = unitsFor(language);
  // A field left as `toDraft` showed it keeps the stored number: the imperial field is rounded to
  // whole inches and pounds, and sending that back would drift the figure on an unrelated edit.
  // A fresh roll still shown in its field beats the stored number: both can show the same rounded
  // figure (5'11" for 180 and 180.3 cm), but the roll is what the player just got.
  const height =
    draft.rolledHeight !== undefined &&
    draft.height === heightField(draft.rolledHeight, units, language)
      ? { value: draft.rolledHeight, invalid: false }
      : original &&
          draft.height === heightField(original.height_cm, units, language)
        ? { value: original.height_cm, invalid: false }
        : parseHeight(draft.height, units);
  const weight =
    draft.rolledWeight !== undefined &&
    draft.weight === weightField(draft.rolledWeight, units, language)
      ? { value: draft.rolledWeight, invalid: false }
      : original &&
          draft.weight === weightField(original.weight_kg, units, language)
        ? { value: original.weight_kg, invalid: false }
        : parseWeight(draft.weight, units);
  return {
    ageInvalid,
    invalid: [
      ...(height.invalid ? (["height"] as const) : []),
      ...(weight.invalid ? (["weight"] as const) : []),
    ],
    body: {
      // A character that lacks its alignment or sex may still edit the rest: an unpicked value is
      // left out, not sent empty (the server refuses an empty one).
      ...(draft.alignment === "" ? {} : { alignment: draft.alignment }),
      ...(draft.sex === "" ? {} : { sex: draft.sex }),
      height_cm: height.value,
      weight_kg: weight.value,
      hair: draft.hair,
      eyes: draft.eyes,
      age: age === "" || ageInvalid ? null : Number(age),
      languages: draft.languages,
    },
  };
}

/**
 * Whether the class no longer admits the character's alignment. `warnings` is absent from the
 * answer of a release that predates it (a rolling deploy, ADR 0024): absent reads as none.
 */
export function hasAlignmentWarning(c: {
  warnings?: { code: string; field: string }[];
}): boolean {
  return (c.warnings ?? []).some(
    (w) => w.code === "alignment_not_allowed" && w.field === "alignment",
  );
}

/** The figures of the server's `languages_over_limit` warning (ADR 0032 step 4.2), none without it. */
export function languageWarning(c: {
  warnings?: { code: string; count?: number; limit?: number }[];
}): { count: number; limit: number } | undefined {
  const w = (c.warnings ?? []).find((x) => x.code === "languages_over_limit");
  return w && w.count !== undefined && w.limit !== undefined
    ? { count: w.count, limit: w.limit }
    : undefined;
}

/** A number outside its table range (ADR 0032 step 5.2): the server's verdict, stored units. */
export interface RangeWarning {
  field: "height" | "weight" | "age";
  value: number;
  min: number;
  max: number;
}

const RANGE_CODES: Record<string, RangeWarning["field"]> = {
  height_outside_range: "height",
  weight_outside_range: "weight",
  age_outside_range: "age",
};

/**
 * The range warnings of a character, by field: `height_outside_range`, `weight_outside_range` and
 * `age_outside_range` with the value and the range. An answer without `warnings` (a previous
 * release) reads as none.
 */
export function rangeWarnings(
  c: Pick<Character, "warnings"> | { warnings?: undefined },
): Partial<Record<RangeWarning["field"], RangeWarning>> {
  const out: Partial<Record<RangeWarning["field"], RangeWarning>> = {};
  for (const w of c.warnings ?? []) {
    const field = RANGE_CODES[w.code];
    if (
      field !== undefined &&
      w.value !== undefined &&
      w.min !== undefined &&
      w.max !== undefined
    )
      out[field] = { field, value: w.value, min: w.min, max: w.max };
  }
  return out;
}
