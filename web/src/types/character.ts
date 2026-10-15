export interface Character {
  id: number;
  name: string;
  str: number;
  dex: number;
  con: number;
  int: number;
  wis: number;
  cha: number;
  hpMax: number;
  thac0Base: number;
  level: number;
  xpTotal: number;

  // Saving Throws
  saveParaPoison: number;
  saveRodStaffWand: number;
  savePetriPoly: number;
  saveBreath: number;
  saveSpell: number;

  /** Class key (e.g. "bard"), read-only. */
  class: string;

  // Details the player edits; age is null when not set.
  race: string;
  alignment: string;
  age: number | null;
  sex: string;
  height: string;
  weight: string;
  hair: string;
  eyes: string;
  languages: string[];
}

/** The editable details, as the server stores them. */
export type CharacterDetails = Pick<
  Character,
  | "race"
  | "alignment"
  | "age"
  | "sex"
  | "height"
  | "weight"
  | "hair"
  | "eyes"
  | "languages"
>;
