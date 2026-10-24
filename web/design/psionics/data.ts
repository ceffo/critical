// The psionics mock-ups' data, shaped as the catalogue serves it (format 18: ADR 0043 step 8,
// ADR 0044 step 1). The screens are templates over these records: every row, button, form and tag
// is chosen by a field (kind, discipline, initial_cost's shape, maintenance), never by a power's id
// ("a power's rules are its data", ADR 0043 step 8). The powers and rule-table rows below are copied
// from the catalogue (names en/fr, scores, costs, maintenance); the two characters are fixtures:
// two different sets of known powers that the same templates must lay out unchanged.

export type L = { fr: string; en: string };
export const l = (fr: string, en: string): L => ({ fr, en });

export type Ability = "str" | "dex" | "con" | "int" | "wis" | "cha";
export type Discipline =
  | "clairsentience"
  | "psychokinesis"
  | "psychometabolism"
  | "psychoportation"
  | "telepathy"
  | "metapsionics";
export type Kind = "science" | "devotion" | "defence_mode" | "attack_mode";

/** `initial_cost`: a number, `variable`, a rule table that gives it, or contact (+ an amount). */
export type InitialCost =
  | number
  | "variable"
  | { table: "contact_cost" }
  | { kind: "contact"; plus?: number };

export interface Power {
  id: string;
  name: L;
  discipline: Discipline;
  kind: Kind;
  score: { ability: Ability; modifier: number };
  initial_cost: InitialCost;
  /** Absent: the power cannot be maintained. `period` in rounds. */
  maintenance?: { cost: number | "variable"; period: number };
}

/** The six disciplines' names (catalogue `disciplines`). */
export const DISCIPLINES: Record<Discipline, L> = {
  clairsentience: l("Clairsentience", "Clairsentience"),
  psychokinesis: l("Psychokinésie", "Psychokinesis"),
  psychometabolism: l("Psychométabolisme", "Psychometabolism"),
  psychoportation: l("Psychotranslation", "Psychoportation"),
  telepathy: l("Télépathie", "Telepathy"),
  metapsionics: l("Métapsionisme", "Metapsionics"),
};

const P = (
  id: string,
  fr: string,
  en: string,
  discipline: Discipline,
  kind: Kind,
  ability: Ability,
  modifier: number,
  initial_cost: InitialCost,
  maintenance?: Power["maintenance"],
): Power => ({ id, name: l(fr, en), discipline, kind, score: { ability, modifier }, initial_cost, maintenance });

/** Catalogue rows (copied from format 18). */
export const CATALOGUE: Power[] = [
  P("mind_thrust", "Pression mentale", "Mind Thrust", "telepathy", "attack_mode", "wis", -2, 2),
  P("ego_whip", "Cinglement de l'ego", "Ego Whip", "telepathy", "attack_mode", "wis", -3, 4),
  P("psionic_blast", "Souffle psionique", "Psionic Blast", "telepathy", "attack_mode", "wis", -5, 10),
  P("mind_blank", "Néant spirituel", "Mind Blank", "telepathy", "defence_mode", "wis", -7, 0, { cost: 0, period: 1 }),
  P("thought_shield", "Bouclier de la pensée", "Thought Shield", "telepathy", "defence_mode", "wis", -3, 1),
  P("mental_barrier", "Barrière mentale", "Mental Barrier", "telepathy", "defence_mode", "wis", -2, 3),
  P("intellect_fortress", "Forteresse intellectuelle", "Intellect Fortress", "telepathy", "defence_mode", "wis", -3, 4),
  P("tower_of_iron_will", "Tour de la volonté de fer", "Tower of Iron Will", "telepathy", "defence_mode", "wis", -2, 6),
  P("contact", "Contact", "Contact", "telepathy", "devotion", "wis", 0, { table: "contact_cost" }, { cost: 1, period: 1 }),
  P("mindlink", "Lien spirituel", "Mindlink", "telepathy", "science", "wis", -5, { kind: "contact" }, { cost: 8, period: 1 }),
  P("switch_personality", "Echange de personnalité", "Switch Personality", "telepathy", "science", "con", -4, { kind: "contact", plus: 30 }),
  P("aura_sight", "Vision de l'Aura", "Aura Sight", "clairsentience", "science", "wis", -5, 9, { cost: 9, period: 1 }),
  P("danger_sense", "Sens du danger", "Danger Sense", "clairsentience", "devotion", "wis", -3, 4, { cost: 3, period: 10 }),
  P("clairvoyance", "Clairvoyance", "Clairvoyance", "clairsentience", "science", "wis", -4, 7, { cost: 4, period: 1 }),
  P("telekinesis", "Télékinésie", "Telekinesis", "psychokinesis", "science", "wis", -3, "variable", { cost: "variable", period: 1 }),
  P("cell_adjustment", "Ajustement cellulaire", "Cell Adjustment", "psychometabolism", "devotion", "con", -3, 5, { cost: "variable", period: 1 }),
  P("biofeedback", "Biorétroaction", "Biofeedback", "psychometabolism", "devotion", "con", -2, 6, { cost: 3, period: 1 }),
  P("body_equilibrium", "Equilibre corporel", "Body Equilibrium", "psychometabolism", "devotion", "con", -3, 2, { cost: 2, period: 1 }),
  P("flesh_armor", "Armure charnelle", "Flesh Armor", "psychometabolism", "devotion", "con", -3, 8, { cost: 4, period: 1 }),
  P("displacement", "Eclipse", "Displacement", "psychometabolism", "devotion", "con", -3, 6, { cost: 3, period: 1 }),
  P("teleport", "Téléportation", "Teleport", "psychoportation", "science", "int", 0, "variable"),
];

export const power = (id: string): Power => {
  const p = CATALOGUE.find((x) => x.id === id);
  if (!p) throw new Error(`no power ${id}`);
  return p;
};

/* ---------- rule tables (format 18), copied from the catalogue ---------- */

/** Table 6: PSP recovery per activity. */
export interface RecoveryRow {
  activity: string;
  per_hour: number;
  /** PROPOSED data field (not in format 18): the row needs this non-weapon proficiency known. */
  requires_proficiency?: string;
}
export const PSP_RECOVERY: RecoveryRow[] = [
  { activity: "exertion", per_hour: 0 },
  { activity: "walking", per_hour: 3 },
  { activity: "resting", per_hour: 6 },
  { activity: "sleeping", per_hour: 12 },
  // The book's Table 6 row "Rejuvenating** 12/hour" (** = the Rejuvenation proficiency): missing
  // from format 18 and ADR 0043 step 6. Drawn as a proposed row, gated by data, not by code.
  { activity: "rejuvenating", per_hour: 12, requires_proficiency: "rejuvenation" },
];

/** The web words the activity codes (ADR 0044 step 1.3: codes worded by the web). */
export const ACTIVITY: Record<string, L> = {
  exertion: l("Effort", "Exertion"),
  walking: l("Marche", "Walking"),
  resting: l("Repos", "Resting"),
  sleeping: l("Sommeil", "Sleeping"),
  rejuvenating: l("Méditation", "Meditation"),
};

/** Table 29. */
export const CONTACT_COST = [
  { min_level: 1, max_level: 5, cost: 3 },
  { min_level: 6, max_level: 10, cost: 8 },
  { min_level: 11, max_level: 15, cost: 13 },
  { min_level: 16, max_level: 20, cost: 18 },
];

/** Tables 30, 31 and the resistance line. */
export const CONTACT_MODIFIERS: { kind: string; code: string; modifier: number }[] = [
  { kind: "distance", code: "sight", modifier: 0 },
  { kind: "distance", code: "mile_1", modifier: -1 },
  { kind: "distance", code: "miles_10", modifier: -3 },
  { kind: "distance", code: "miles_100", modifier: -5 },
  { kind: "distance", code: "miles_1000", modifier: -7 },
  { kind: "distance", code: "miles_10000", modifier: -9 },
  { kind: "life_order", code: "mammal", modifier: -1 },
  { kind: "life_order", code: "marsupial", modifier: -2 },
  { kind: "life_order", code: "bird", modifier: -3 },
  { kind: "life_order", code: "reptile_amphibian", modifier: -4 },
  { kind: "life_order", code: "fish", modifier: -5 },
  { kind: "life_order", code: "arachnid_insect", modifier: -6 },
  { kind: "life_order", code: "monster", modifier: -7 },
  { kind: "life_order", code: "plant", modifier: -8 },
  { kind: "resistance", code: "resisting", modifier: -2 },
];
export const MODIFIER_WORDS: Record<string, L> = {
  distance: l("Distance", "Distance"),
  life_order: l("Ordre de vie", "Life order"),
  resistance: l("Résistance", "Resistance"),
  sight: l("À vue", "In sight"),
  mile_1: l("1,5 km", "1 mile"),
  miles_10: l("15 km", "10 miles"),
  miles_100: l("150 km", "100 miles"),
  miles_1000: l("1 500 km", "1,000 miles"),
  miles_10000: l("15 000 km", "10,000 miles"),
  mammal: l("Mammifère", "Mammal"),
  resisting: l("Résiste", "Resists"),
};

/** Table 14 cells used by the fixtures (attack, defence) → modifier. */
export const ATTACK_VS_DEFENCE: Record<string, number> = {
  "mind_thrust/mind_blank": 5,
  "mind_thrust/thought_shield": -2,
  "mind_thrust/mental_barrier": -4,
  "ego_whip/mind_blank": 5,
  "ego_whip/thought_shield": 0,
  "ego_whip/mental_barrier": -3,
  "psionic_blast/mind_blank": 2,
  "psionic_blast/thought_shield": 3,
  "psionic_blast/mental_barrier": 0,
};

/* ---------- characters (fixtures) ---------- */

export interface Fixture {
  name: string;
  classes: { name: L; level: number }[];
  abilities: Record<Ability, number>;
  thac0: number;
  hp: number;
  hpMax: number;
  ac: number;
  psp: number;
  pspMax: number;
  saves: [L, string][];
  /** Known powers, in the player's order (catalogue ids: data, not code). */
  known: string[];
  /** Known non-weapon proficiencies (ids). */
  proficiencies: string[];
}

const SAVES = (a: string, b: string, c: string, d: string, e: string): [L, string][] => [
  [l("Paralysie/Poison", "Paralyze/Poison"), a],
  [l("Bâton/Sceptre/Baguette", "Rod/Staff/Wand"), b],
  [l("Pétrification/Métamorphose", "Petrify/Polymorph"), c],
  [l("Souffle", "Breath Weapon"), d],
  [l("Sorts", "Spells"), e],
];

/** The table's thief/psionicist (ADR 0042 step 17, ADR 0043 step 19): a telepath. */
export const JORAN: Fixture = {
  name: "Joran Veil",
  classes: [
    { name: l("Voleur", "Thief"), level: 7 },
    { name: l("Psioniste", "Psionicist"), level: 8 },
  ],
  abilities: { str: 11, dex: 17, con: 16, int: 15, wis: 17, cha: 10 },
  thac0: 16,
  hp: 48,
  hpMax: 52,
  ac: 4,
  psp: 71,
  pspMax: 109,
  saves: SAVES("11+", "12+", "11+", "15+", "11+"),
  known: [
    "mind_thrust",
    "ego_whip",
    "psionic_blast",
    "mind_blank",
    "thought_shield",
    "mental_barrier",
    "contact",
    "mindlink",
    "switch_personality",
    "aura_sight",
    "danger_sense",
    "telekinesis",
  ],
  proficiencies: [],
};

/** A second fixture with nothing in common but the class: single-class, no attack mode, no
 * power that takes a mind, no standing defence, three other disciplines, Rejuvenation known. */
export const YSOLDE: Fixture = {
  name: "Ysolde Marr",
  classes: [{ name: l("Psioniste", "Psionicist"), level: 6 }],
  abilities: { str: 10, dex: 13, con: 17, int: 14, wis: 15, cha: 12 },
  thac0: 18,
  hp: 31,
  hpMax: 34,
  ac: 6,
  psp: 38,
  pspMax: 66,
  saves: SAVES("12+", "14+", "13+", "16+", "15+"),
  known: [
    "intellect_fortress",
    "tower_of_iron_will",
    "cell_adjustment",
    "biofeedback",
    "body_equilibrium",
    "flesh_armor",
    "displacement",
    "teleport",
    "clairvoyance",
  ],
  proficiencies: ["rejuvenation"],
};

/** The power score: the ability plus the power's modifier (relearns omitted in the fixtures). */
export const scoreOf = (f: Fixture, p: Power) => f.abilities[p.score.ability] + p.score.modifier;

export const knownPowers = (f: Fixture) => f.known.map(power);
