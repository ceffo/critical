import type { CharacterDetails } from "@/types/character";

/** The free-text fields, in display order (alignment and sex are codes picked from the server's lists). */
export const TEXT_FIELDS = ["height", "weight", "hair", "eyes"] as const;
export type TextField = (typeof TEXT_FIELDS)[number];

/** The form's state: everything as the text typed in the inputs. */
export interface Draft {
  alignment: string;
  age: string;
  sex: string;
  height: string;
  weight: string;
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

export function toDraft(d: CharacterDetails): Draft {
  return {
    ...d,
    alignment: codeOrEmpty(d.alignment, ALIGNMENT_CODES),
    sex: codeOrEmpty(d.sex, SEX_CODES),
    age: d.age === null ? "" : String(d.age),
    languages: [...d.languages],
  };
}

/**
 * The PATCH body for a draft. Only the shape is checked here: an age that is not a whole number
 * from 1 up is flagged for the caller to report, and an empty age is sent as null, which clears it.
 * Every limit is the server's.
 */
export function toBody(draft: Draft): {
  body: Partial<CharacterDetails>;
  ageInvalid: boolean;
} {
  const age = draft.age.trim();
  const ageInvalid = age !== "" && !/^0*[1-9]\d*$/.test(age);
  return {
    ageInvalid,
    body: {
      // A character that lacks its alignment or sex may still edit the rest: an unpicked value is
      // left out, not sent empty (the server refuses an empty one).
      ...(draft.alignment === "" ? {} : { alignment: draft.alignment }),
      ...(draft.sex === "" ? {} : { sex: draft.sex }),
      height: draft.height,
      weight: draft.weight,
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
