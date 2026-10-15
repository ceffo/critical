import type { CharacterDetails } from "@/types/character";

/** The text fields, in display order. */
export const TEXT_FIELDS = ["race", "alignment", "sex", "height", "weight", "hair", "eyes"] as const;
export type TextField = (typeof TEXT_FIELDS)[number];

/** The form's state: everything as the text typed in the inputs. */
export interface Draft {
  race: string;
  alignment: string;
  age: string;
  sex: string;
  height: string;
  weight: string;
  hair: string;
  eyes: string;
  languages: string;
}

export function toDraft(d: CharacterDetails): Draft {
  return { ...d, age: d.age === null ? "" : String(d.age), languages: d.languages.join(", ") };
}

/**
 * The PATCH body for a draft. Only the shape is checked here (an age that is not a whole number
 * gives null, the caller reports it), every limit is the server's.
 */
export function toBody(draft: Draft): { body: CharacterDetails; ageInvalid: boolean } {
  const age = draft.age.trim();
  const ageInvalid = age !== "" && !/^\d+$/.test(age);
  const languages = draft.languages.trim() === "" ? [] : draft.languages.split(",").map((l) => l.trim());
  return {
    ageInvalid,
    body: {
      race: draft.race,
      alignment: draft.alignment,
      sex: draft.sex,
      height: draft.height,
      weight: draft.weight,
      hair: draft.hair,
      eyes: draft.eyes,
      age: age === "" || ageInvalid ? null : Number(age),
      languages,
    },
  };
}

/** i18n key suffix of a language name: lower case, accents and spaces removed. */
export function languageKey(name: string): string {
  return name
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]/g, "");
}
