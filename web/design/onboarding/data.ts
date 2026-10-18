// The mock-ups' data: a half-elf bard created at 2 500 XP (level 3), with PHB values (Tables 4,
// 8, 10, 11; DMG chapter 1 method V; ADR 0030 step 18's bard budget: 20 + 15 per level after 1).
// The rolls are fixed so every screenshot shows the same breakdown.

export const CHAR = {
  name: "Aëlwyn de Valcourt-Lestrange",
};

export const STEPS = [
  { id: "class", fr: "Classe", en: "Class", doneFr: "Barde · niveau 3", doneEn: "Bard · level 3" },
  { id: "race", fr: "Race", en: "Race", doneFr: "Demi-elfe", doneEn: "Half-elf" },
  { id: "scores", fr: "Caractéristiques", en: "Ability scores", doneFr: "Méthode V", doneEn: "Method V" },
  { id: "hp", fr: "Points de vie et talents", en: "Hit points and skills", doneFr: "14 PV · 48 points", doneEn: "14 HP · 48 points" },
  { id: "identity", fr: "Identité", en: "Identity", doneFr: "Aëlwyn", doneEn: "Aëlwyn" },
  { id: "review", fr: "Récapitulatif", en: "Review", doneFr: "", doneEn: "" },
] as const;

export const ABILITIES = [
  { id: "str", fr: "FOR", en: "STR", nameFr: "Force", nameEn: "Strength", score: 10, min: 0 },
  { id: "dex", fr: "DEX", en: "DEX", nameFr: "Dextérité", nameEn: "Dexterity", score: 15, min: 12 },
  { id: "con", fr: "CON", en: "CON", nameFr: "Constitution", nameEn: "Constitution", score: 13, min: 0 },
  { id: "int", fr: "INT", en: "INT", nameFr: "Intelligence", nameEn: "Intelligence", score: 13, min: 13 },
  { id: "wis", fr: "SAG", en: "WIS", nameFr: "Sagesse", nameEn: "Wisdom", score: 12, min: 0 },
  { id: "cha", fr: "CHA", en: "CHA", nameFr: "Charisme", nameEn: "Charisma", score: 16, min: 15 },
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
  { id: "II", fr: "3d6 deux fois, garder le meilleur", en: "3d6 twice, keep one" },
  { id: "III", fr: "3d6 six fois, répartir", en: "3d6 six times, arrange" },
  { id: "IV", fr: "3d6 douze fois, garder six", en: "3d6 twelve times, keep six" },
  { id: "V", fr: "4d6 sans le plus bas, répartir", en: "4d6 drop lowest, arrange" },
  { id: "VI", fr: "8 partout + 7d6 à répartir", en: "8 each + 7d6 to add" },
] as const;

export const CLASSES = [
  {
    id: "bard",
    fr: "Barde",
    en: "Bard",
    groupFr: "Roublard",
    groupEn: "Rogue",
    hd: "d6",
    reqFr: "DEX 12 · INT 13 · CHA 15",
    reqEn: "DEX 12 · INT 13 · CHA 15",
    alFr: "En partie neutre",
    alEn: "Partly neutral",
    racesFr: "Humain, demi-elfe",
    racesEn: "Human, half-elf",
  },
  {
    id: "fighter",
    fr: "Guerrier",
    en: "Fighter",
    groupFr: "Combattant",
    groupEn: "Warrior",
    hd: "d10",
    reqFr: "FOR 9",
    reqEn: "STR 9",
    alFr: "Tous",
    alEn: "Any",
    racesFr: "Toutes",
    racesEn: "All",
  },
  {
    id: "mage",
    fr: "Magicien",
    en: "Mage",
    groupFr: "Magicien",
    groupEn: "Wizard",
    hd: "d4",
    reqFr: "INT 9",
    reqEn: "INT 9",
    alFr: "Tous",
    alEn: "Any",
    racesFr: "Humain, elfe, demi-elfe",
    racesEn: "Human, elf, half-elf",
  },
  {
    id: "cleric",
    fr: "Clerc",
    en: "Cleric",
    groupFr: "Prêtre",
    groupEn: "Priest",
    hd: "d8",
    reqFr: "SAG 9",
    reqEn: "WIS 9",
    alFr: "Celui de sa divinité",
    alEn: "Their deity's",
    racesFr: "Toutes",
    racesEn: "All",
  },
  {
    id: "thief",
    fr: "Voleur",
    en: "Thief",
    groupFr: "Roublard",
    groupEn: "Rogue",
    hd: "d6",
    reqFr: "DEX 9",
    reqEn: "DEX 9",
    alFr: "Tous sauf loyal bon",
    alEn: "Any but lawful good",
    racesFr: "Toutes",
    racesEn: "All",
  },
] as const;

/** PHB chapter 2 and Tables 8, 10, 11; heights for both sexes (no sex chosen yet). */
export const RACES = [
  { id: "human", fr: "Humain", en: "Human", ok: true, adjFr: "Aucun", adjEn: "None", langFr: "Commun", langEn: "Common", hFr: "1,54 – 2,04 m", hEn: "5′1″ – 6′8″", ageFr: "16 – 19 ans", ageEn: "16 – 19 years" },
  { id: "half_elf", fr: "Demi-elfe", en: "Half-elf", ok: true, adjFr: "Aucun", adjEn: "None", langFr: "Commun", langEn: "Common", hFr: "1,52 – 1,83 m", hEn: "4′11″ – 6′0″", ageFr: "16 – 21 ans", ageEn: "16 – 21 years" },
  { id: "elf", fr: "Elfe", en: "Elf", ok: false, adjFr: "DEX +1 · CON −1", adjEn: "DEX +1 · CON −1", langFr: "Elfe", langEn: "Elf", hFr: "1,29 – 1,66 m", hEn: "4′3″ – 5′5″", ageFr: "105 – 130 ans", ageEn: "105 – 130 years" },
  { id: "dwarf", fr: "Nain", en: "Dwarf", ok: false, adjFr: "CON +1 · CHA −1", adjEn: "CON +1 · CHA −1", langFr: "Nain", langEn: "Dwarf", hFr: "1,06 – 1,35 m", hEn: "3′6″ – 4′5″", ageFr: "45 – 70 ans", ageEn: "45 – 70 years" },
  { id: "gnome", fr: "Gnome", en: "Gnome", ok: false, adjFr: "INT +1 · SAG −1", adjEn: "INT +1 · WIS −1", langFr: "Gnome", langEn: "Gnome", hFr: "0,93 – 1,12 m", hEn: "3′1″ – 3′8″", ageFr: "63 – 96 ans", ageEn: "63 – 96 years" },
  { id: "halfling", fr: "Petite-gens", en: "Halfling", ok: false, adjFr: "DEX +1 · FOR −1", adjEn: "DEX +1 · STR −1", langFr: "Halfelin", langEn: "Halfling", hFr: "0,81 – 1,22 m", hEn: "2′8″ – 4′0″", ageFr: "23 – 32 ans", ageEn: "23 – 32 years" },
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

/** The HP roll of a level 3 bard (rogue d6), Constitution 13 (no adjustment). */
export const HP_ROWS = [
  { level: 1, die: 5 },
  { level: 2, die: 3 },
  { level: 3, die: 6 },
] as const;

/** Bard skills (ADR 0030 step 18): base, race (half-elf), Dexterity 15, armour (leather), points. */
export const BARD_SKILLS = [
  { fr: "Escalade", en: "Climb walls", base: 50, race: 0, dex: 0, points: 15 },
  { fr: "Perception des bruits", en: "Detect noise", base: 20, race: 0, dex: 0, points: 15 },
  { fr: "Vol à la tire", en: "Pick pockets", base: 10, race: 10, dex: 0, points: 10 },
  { fr: "Lecture des langues", en: "Read languages", base: 5, race: 0, dex: 0, points: 8 },
] as const;

export const ROSTER = [
  { name: "Kelsen Balstriel", classFr: "Barde", classEn: "Bard", level: 15, raceFr: "Demi-elfe", raceEn: "Half-elf", alFr: "Neutre bon", alEn: "Neutral good", hp: "77 / 85", current: true, missing: false },
  { name: "Aëlwyn de Valcourt-Lestrange", classFr: "Barde", classEn: "Bard", level: 3, raceFr: "Demi-elfe", raceEn: "Half-elf", alFr: "Neutre bon", alEn: "Neutral good", hp: "14 / 14", current: false, missing: false },
  { name: "Thorgrim Barbe-de-Fer", classFr: "Guerrier", classEn: "Fighter", level: 4, raceFr: "Nain", raceEn: "Dwarf", alFr: "", alEn: "", hp: "31 / 38", current: false, missing: true },
  { name: "Mirelle des Hautes-Brumes", classFr: "Magicien", classEn: "Mage", level: 7, raceFr: "Elfe", raceEn: "Elf", alFr: "Chaotique bon", alEn: "Chaotic good", hp: "19 / 19", current: false, missing: false },
] as const;
