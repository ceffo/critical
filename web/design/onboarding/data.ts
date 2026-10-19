// The mock-ups' data: a half-elf bard created at level 5 (10 000 XP, the level's lowest), with PHB values (Tables 4,
// 8, 10, 11; DMG chapter 1 method V; ADR 0030 step 18's bard budget: 20 + 15 per level after 1).
// The rolls are fixed so every screenshot shows the same breakdown.

export const CHAR = {
  name: "Aëlwyn de Valcourt-Lestrange",
};

export const STEPS = [
  {
    id: "class",
    fr: "Classe",
    en: "Class",
    done: { fr: "Barde · niveau 5", en: "Bard · level 5" },
  },
  {
    id: "race",
    fr: "Race",
    en: "Race",
    done: { fr: "Demi-elfe", en: "Half-elf" },
  },
  {
    id: "scores",
    fr: "Caractéristiques",
    en: "Ability scores",
    done: { fr: "Méthode V", en: "Method V" },
  },
  {
    id: "hp",
    fr: "Points de vie et talents",
    en: "Hit points and skills",
    done: { fr: "20 PV · 78 points", en: "20 HP · 78 points" },
  },
  {
    id: "identity",
    fr: "Identité",
    en: "Identity",
    done: { fr: "Aëlwyn", en: "Aëlwyn" },
  },
  { id: "review", fr: "Récapitulatif", en: "Review", done: { fr: "", en: "" } },
] as const;

export const ABILITIES = [
  {
    id: "str",
    fr: "FOR",
    en: "STR",
    name: { fr: "Force", en: "Strength" },
    score: 10,
    min: 0,
  },
  {
    id: "dex",
    fr: "DEX",
    en: "DEX",
    name: { fr: "Dextérité", en: "Dexterity" },
    score: 15,
    min: 12,
  },
  {
    id: "con",
    fr: "CON",
    en: "CON",
    name: { fr: "Constitution", en: "Constitution" },
    score: 13,
    min: 0,
  },
  {
    id: "int",
    fr: "INT",
    en: "INT",
    name: { fr: "Intelligence", en: "Intelligence" },
    score: 13,
    min: 13,
  },
  {
    id: "wis",
    fr: "SAG",
    en: "WIS",
    name: { fr: "Sagesse", en: "Wisdom" },
    score: 12,
    min: 0,
  },
  {
    id: "cha",
    fr: "CHA",
    en: "CHA",
    name: { fr: "Charisme", en: "Charisma" },
    score: 16,
    min: 15,
  },
] as const;

/** Method V: 4d6 six times, the lowest die dropped (index of the dropped die), and where it went. */
export const ROLLS = [
  { dice: [6, 5, 4, 2], drop: 3, to: "dex" },
  { dice: [5, 1, 5, 3], drop: 1, to: "int" },
  { dice: [6, 6, 3, 4], drop: 2, to: "cha" },
  { dice: [4, 3, 2, 3], drop: 2, to: "str" },
  { dice: [6, 4, 2, 1], drop: 3, to: "wis" },
  { dice: [5, 4, 3, 4], drop: 2, to: "con" },
] as const;

export const total = (r: (typeof ROLLS)[number]) =>
  r.dice.reduce((s, d, i) => (i === r.drop ? s : s + d), 0);

export const METHODS = [
  { id: "I", fr: "3d6, dans l'ordre", en: "3d6, in order" },
  {
    id: "II",
    fr: "3d6 deux fois, garder le meilleur",
    en: "3d6 twice, keep one",
  },
  { id: "III", fr: "3d6 six fois, répartir", en: "3d6 six times, arrange" },
  {
    id: "IV",
    fr: "3d6 douze fois, garder six",
    en: "3d6 twelve times, keep six",
  },
  {
    id: "V",
    fr: "4d6 sans le plus bas, répartir",
    en: "4d6 drop lowest, arrange",
  },
  { id: "VI", fr: "8 partout + 7d6 à répartir", en: "8 each + 7d6 to add" },
] as const;

export const CLASSES = [
  {
    id: "bard",
    fr: "Barde",
    en: "Bard",
    group: { fr: "Roublard", en: "Rogue" },
    hd: "d6",
    req: { fr: "DEX 12 · INT 13 · CHA 15", en: "DEX 12 · INT 13 · CHA 15" },
    al: { fr: "En partie neutre", en: "Partly neutral" },
    races: { fr: "Humain, demi-elfe", en: "Human, half-elf" },
  },
  {
    id: "fighter",
    fr: "Guerrier",
    en: "Fighter",
    group: { fr: "Combattant", en: "Warrior" },
    hd: "d10",
    req: { fr: "FOR 9", en: "STR 9" },
    al: { fr: "Tous", en: "Any" },
    races: { fr: "Toutes", en: "All" },
  },
  {
    id: "mage",
    fr: "Magicien",
    en: "Mage",
    group: { fr: "Magicien", en: "Wizard" },
    hd: "d4",
    req: { fr: "INT 9", en: "INT 9" },
    al: { fr: "Tous", en: "Any" },
    races: { fr: "Humain, elfe, demi-elfe", en: "Human, elf, half-elf" },
  },
  {
    id: "cleric",
    fr: "Clerc",
    en: "Cleric",
    group: { fr: "Prêtre", en: "Priest" },
    hd: "d8",
    req: { fr: "SAG 9", en: "WIS 9" },
    al: { fr: "Celui de sa divinité", en: "Their deity's" },
    races: { fr: "Toutes", en: "All" },
  },
  {
    id: "thief",
    fr: "Voleur",
    en: "Thief",
    group: { fr: "Roublard", en: "Rogue" },
    hd: "d6",
    req: { fr: "DEX 9", en: "DEX 9" },
    al: { fr: "Tous sauf loyal bon", en: "Any but lawful good" },
    races: { fr: "Toutes", en: "All" },
  },
] as const;

/** PHB chapter 2 and Tables 8, 10, 11; heights for both sexes (no sex chosen yet). */
export const RACES = [
  {
    id: "human",
    fr: "Humain",
    en: "Human",
    ok: true,
    adj: { fr: "Aucun", en: "None" },
    lang: { fr: "Commun", en: "Common" },
    h: { fr: "1,54 – 2,04 m", en: "5′1″ – 6′8″" },
    age: { fr: "16 – 19 ans", en: "16 – 19 years" },
  },
  {
    id: "half_elf",
    fr: "Demi-elfe",
    en: "Half-elf",
    ok: true,
    adj: { fr: "Aucun", en: "None" },
    lang: { fr: "Commun", en: "Common" },
    h: { fr: "1,52 – 1,83 m", en: "4′11″ – 6′0″" },
    age: { fr: "16 – 21 ans", en: "16 – 21 years" },
  },
  {
    id: "elf",
    fr: "Elfe",
    en: "Elf",
    ok: false,
    adj: { fr: "DEX +1 · CON −1", en: "DEX +1 · CON −1" },
    lang: { fr: "Elfe", en: "Elf" },
    h: { fr: "1,29 – 1,66 m", en: "4′3″ – 5′5″" },
    age: { fr: "105 – 130 ans", en: "105 – 130 years" },
  },
  {
    id: "dwarf",
    fr: "Nain",
    en: "Dwarf",
    ok: false,
    adj: { fr: "CON +1 · CHA −1", en: "CON +1 · CHA −1" },
    lang: { fr: "Nain", en: "Dwarf" },
    h: { fr: "1,06 – 1,35 m", en: "3′6″ – 4′5″" },
    age: { fr: "45 – 70 ans", en: "45 – 70 years" },
  },
  {
    id: "gnome",
    fr: "Gnome",
    en: "Gnome",
    ok: false,
    adj: { fr: "INT +1 · SAG −1", en: "INT +1 · WIS −1" },
    lang: { fr: "Gnome", en: "Gnome" },
    h: { fr: "0,93 – 1,12 m", en: "3′1″ – 3′8″" },
    age: { fr: "63 – 96 ans", en: "63 – 96 years" },
  },
  {
    id: "halfling",
    fr: "Petite-gens",
    en: "Halfling",
    ok: false,
    adj: { fr: "DEX +1 · FOR −1", en: "DEX +1 · STR −1" },
    lang: { fr: "Halfelin", en: "Halfling" },
    h: { fr: "0,81 – 1,22 m", en: "2′8″ – 4′0″" },
    age: { fr: "23 – 32 ans", en: "23 – 32 years" },
  },
] as const;

/** The nine alignments, row by row (lawful, neutral, chaotic × good, neutral, evil); bard: partly neutral. */
export const ALIGNMENTS = [
  { fr: "Loyal bon", en: "Lawful good", bard: false },
  { fr: "Neutre bon", en: "Neutral good", bard: true },
  { fr: "Chaotique bon", en: "Chaotic good", bard: false },
  { fr: "Loyal neutre", en: "Lawful neutral", bard: true },
  { fr: "Neutre strict", en: "True neutral", bard: true },
  { fr: "Chaotique neutre", en: "Chaotic neutral", bard: true },
  { fr: "Loyal mauvais", en: "Lawful evil", bard: false },
  { fr: "Neutre mauvais", en: "Neutral evil", bard: true },
  { fr: "Chaotique mauvais", en: "Chaotic evil", bard: false },
] as const;

/** The language catalogue; `race`: in the half-elf's PHB list. */
export const LANGUAGES = [
  { fr: "Elfe", en: "Elf", race: true, on: true },
  { fr: "Gnome", en: "Gnome", race: true, on: true },
  { fr: "Halfelin", en: "Halfling", race: true, on: true },
  { fr: "Orque", en: "Orc", race: true, on: true },
  { fr: "Gobelin", en: "Goblin", race: true, on: false },
  { fr: "Hobgobelin", en: "Hobgoblin", race: true, on: false },
  { fr: "Gnoll", en: "Gnoll", race: true, on: false },
  { fr: "Nain", en: "Dwarf", race: false, on: false },
  { fr: "Kobold", en: "Kobold", race: false, on: false },
  { fr: "Géant", en: "Giant", race: false, on: false },
  { fr: "Draconique", en: "Draconic", race: false, on: false },
] as const;

/** The HP roll of a level 5 bard (rogue d6), Constitution 13 (no adjustment). */
export const HP_ROWS = [
  { level: 1, die: 5 },
  { level: 2, die: 3 },
  { level: 3, die: 6 },
  { level: 4, die: 2 },
  { level: 5, die: 4 },
] as const;

/** Bard skills (ADR 0030 step 18): base, race (half-elf), Dexterity 15, armour (leather), points. */
export const BARD_SKILLS = [
  { fr: "Escalade", en: "Climb walls", base: 50, race: 0, dex: 0, points: 25 },
  {
    fr: "Perception des bruits",
    en: "Detect noise",
    base: 20,
    race: 0,
    dex: 0,
    points: 20,
  },
  {
    fr: "Vol à la tire",
    en: "Pick pockets",
    base: 10,
    race: 10,
    dex: 0,
    points: 20,
  },
  {
    fr: "Lecture des langues",
    en: "Read languages",
    base: 5,
    race: 0,
    dex: 0,
    points: 13,
  },
] as const;

export const ROSTER = [
  {
    name: "Kelsen Balstriel",
    class: { fr: "Barde", en: "Bard" },
    level: 15,
    race: { fr: "Demi-elfe", en: "Half-elf" },
    al: { fr: "Neutre bon", en: "Neutral good" },
    hp: "77 / 85",
    current: true,
    missing: false,
  },
  {
    name: "Aëlwyn de Valcourt-Lestrange",
    class: { fr: "Barde", en: "Bard" },
    level: 5,
    race: { fr: "Demi-elfe", en: "Half-elf" },
    al: { fr: "Neutre bon", en: "Neutral good" },
    hp: "20 / 20",
    current: false,
    missing: false,
  },
  {
    name: "Thorgrim Barbe-de-Fer",
    class: { fr: "Guerrier", en: "Fighter" },
    level: 4,
    race: { fr: "Nain", en: "Dwarf" },
    al: { fr: "", en: "" },
    hp: "31 / 38",
    current: false,
    missing: true,
  },
  {
    name: "Mirelle des Hautes-Brumes",
    class: { fr: "Magicien", en: "Mage" },
    level: 7,
    race: { fr: "Elfe", en: "Elf" },
    al: { fr: "Chaotique bon", en: "Chaotic good" },
    hp: "19 / 19",
    current: false,
    missing: false,
  },
] as const;

/** The bard's (rogue) XP table, PHB Table 25: the lowest XP of each level, 1 to 20. */
export const BARD_LEVEL_XP = [
  0, 1250, 2500, 5000, 10000, 20000, 40000, 70000, 110000, 160000, 220000,
  440000, 660000, 880000, 1100000, 1320000, 1540000, 1760000, 1980000, 2200000,
] as const;
