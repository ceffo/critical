// The mage school mock-ups' data (docs/design/mage-school/README.md). The specialists are ADR 0050's
// table (PHB Table 22 restated: school, races, second ability, opposition), the school and class
// names are the app's locale strings (lookup.schoolNames, details.classes), and the two characters
// are the ones of the "where is the switch" screenshots: an elf mage 6, and an
// elf fighter 6 who took Mage as a second class.

export type L = { fr: string; en: string };
export const l = (fr: string, en: string): L => ({ fr, en });

export type Ability = "str" | "dex" | "con" | "int" | "wis" | "cha";
export type School =
  | "abjuration"
  | "alteration"
  | "conjuration_summoning"
  | "greater_divination"
  | "enchantment_charm"
  | "illusion"
  | "invocation_evocation"
  | "necromancy";

export const SCHOOLS: Record<School, L> = {
  abjuration: l("Abjuration", "Abjuration"),
  alteration: l("Altération", "Alteration"),
  conjuration_summoning: l("Conjuration", "Conjuration/Summoning"),
  greater_divination: l("Divination majeure", "Greater divination"),
  enchantment_charm: l("Enchantement/Charme", "Enchantment/Charm"),
  illusion: l("Illusion", "Illusion"),
  invocation_evocation: l("Invocation/Évocation", "Invocation/Evocation"),
  necromancy: l("Nécromancie", "Necromancy"),
};

export const ABILITY_NAMES: Record<Ability, L> = {
  str: l("FOR", "STR"),
  dex: l("DEX", "DEX"),
  con: l("CON", "CON"),
  int: l("INT", "INT"),
  wis: l("SAG", "WIS"),
  cha: l("CHA", "CHA"),
};

export type Race = "human" | "half_elf" | "elf" | "gnome";
export const RACES: Record<Race, L> = {
  human: l("Humain", "Human"),
  half_elf: l("Demi-elfe", "Half-elf"),
  elf: l("Elfe", "Elf"),
  gnome: l("Gnome", "Gnome"),
};

export interface Specialist {
  id: string;
  name: L;
  school: School;
  races: Race[];
  minimum: [Ability, number];
  opposed: School[];
}

/** ADR 0050's table, in its order. */
export const SPECIALISTS: Specialist[] = [
  {
    id: "abjurer",
    name: l("Abjurateur", "Abjurer"),
    school: "abjuration",
    races: ["human"],
    minimum: ["wis", 15],
    opposed: ["alteration", "illusion"],
  },
  {
    id: "conjurer",
    name: l("Conjurateur", "Conjurer"),
    school: "conjuration_summoning",
    races: ["human", "half_elf"],
    minimum: ["con", 15],
    opposed: ["greater_divination", "invocation_evocation"],
  },
  {
    id: "diviner",
    name: l("Devin", "Diviner"),
    school: "greater_divination",
    races: ["human", "half_elf", "elf"],
    minimum: ["wis", 16],
    opposed: ["conjuration_summoning"],
  },
  {
    id: "enchanter",
    name: l("Enchanteur", "Enchanter"),
    school: "enchantment_charm",
    races: ["human", "half_elf", "elf"],
    minimum: ["cha", 16],
    opposed: ["invocation_evocation", "necromancy"],
  },
  {
    id: "illusionist",
    name: l("Illusionniste", "Illusionist"),
    school: "illusion",
    races: ["human", "gnome"],
    minimum: ["dex", 16],
    opposed: ["necromancy", "invocation_evocation", "abjuration"],
  },
  {
    id: "invoker",
    name: l("Invocateur", "Invoker"),
    school: "invocation_evocation",
    races: ["human"],
    minimum: ["con", 16],
    opposed: ["enchantment_charm", "conjuration_summoning"],
  },
  {
    id: "necromancer",
    name: l("Nécromancien", "Necromancer"),
    school: "necromancy",
    races: ["human"],
    minimum: ["wis", 16],
    opposed: ["illusion", "enchantment_charm"],
  },
  {
    id: "transmuter",
    name: l("Transmutateur", "Transmuter"),
    school: "alteration",
    races: ["human", "half_elf"],
    minimum: ["dex", 15],
    opposed: ["abjuration", "necromancy"],
  },
];

export interface Entry {
  name: L;
  level: number;
  /** The first class of a dual-class character, kept but no longer advancing. */
  frozen?: boolean;
}

export interface Fixture {
  name: string;
  race: Race;
  entries: Entry[];
  abilities: Record<Ability, number>;
  thac0: number;
  hp: number;
  hpMax: number;
  ac: number;
  saves: [L, string][];
}

const SAVES = (
  a: string,
  b: string,
  c: string,
  d: string,
  e: string,
): [L, string][] => [
  [l("Paralysie/Poison", "Paralysis/Poison"), a],
  [l("Bâton/Sceptre/Baguette", "Rod/Staff/Wand"), b],
  [l("Pétrification/Métamorphose", "Petrification/Polymorph"), c],
  [l("Souffle", "Breath weapon"), d],
  [l("Sorts", "Spell"), e],
];

/** The single-class mage of the screenshots, path A. */
export const MAGE: Fixture = {
  name: "Test Bard",
  race: "elf",
  entries: [{ name: l("Magicien", "Mage"), level: 6 }],
  abilities: { str: 12, dex: 17, con: 16, int: 17, wis: 8, cha: 16 },
  thac0: 19,
  hp: 85,
  hpMax: 85,
  ac: 7,
  saves: SAVES("13+", "9+", "11+", "13+", "10+"),
};

/** The fighter who took Mage as a second class, path B (the friend's case, smaller). */
export const DUAL: Fixture = {
  ...MAGE,
  entries: [
    { name: l("Guerrier", "Fighter"), level: 6, frozen: true },
    { name: l("Magicien", "Mage"), level: 1 },
  ],
  abilities: { str: 16, dex: 17, con: 16, int: 17, wis: 8, cha: 16 },
  thac0: 15,
  saves: SAVES("14+", "11+", "13+", "15+", "12+"),
};

/** A few spells of the mage's book, with their school (the catalogue's). */
export const BOOK: [L, number, School][] = [
  [l("Projectile magique", "Magic Missile"), 1, "invocation_evocation"],
  [l("Bouclier", "Shield"), 1, "invocation_evocation"],
  [l("Charme-personne", "Charm Person"), 1, "enchantment_charm"],
  [l("Feuille morte", "Feather Fall"), 1, "alteration"],
  [l("Image miroir", "Mirror Image"), 2, "illusion"],
  [l("Toile d'araignée", "Web"), 2, "alteration"],
  [l("Boule de feu", "Fireball"), 3, "invocation_evocation"],
  [l("Hâte", "Haste"), 3, "alteration"],
  [l("Dissipation de la magie", "Dispel Magic"), 3, "abjuration"],
];

export const meets = (f: Fixture, s: Specialist) =>
  f.abilities[s.minimum[0]] >= s.minimum[1];
export const raceListed = (f: Fixture, s: Specialist) =>
  s.races.includes(f.race);
