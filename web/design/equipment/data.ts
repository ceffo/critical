// The equipment mock-ups' data: Kelsen, a level 15 half-elf bard, after the conversion of ADR 0033
// step 26 (his items and weapons in one possessions list, the chain mail guessed from his skill
// armour). Numbers are PHB values (Table 44, 46, Table 2's Dexterity 17: -3).

export type L = { fr: string; en: string };
export const l = (fr: string, en: string): L => ({ fr, en });

export type Category =
  | "weapon"
  | "ammunition"
  | "armour"
  | "shield"
  | "magic"
  | "gear";
export type State = "worn" | "wielded" | "carried";

export interface Piece {
  id: string;
  name: L;
  category: Category;
  /** One line of what matters at a glance (stats), in the UI language. */
  stats: L;
  /** The kind under the category: "Mêlée", "Anneau", "Flèche", "CA 5"... */
  kind: L;
  state: State;
  count?: { value: number; max?: number; behaviour: "stock" | "charges" | "daily" };
  dice?: string;
  review?: boolean;
  warning?: L;
}

export const CHAR = {
  name: "Kelsen Balstriel",
  cls: l("Barde", "Bard"),
  level: 15,
  abilities: [
    [l("FOR", "STR"), 12],
    [l("DEX", "DEX"), 17],
    [l("CON", "CON"), 16],
    [l("INT", "INT"), 14],
    [l("SAG", "WIS"), 8],
    [l("CHA", "CHA"), 16],
    [l("TAC0", "THAC0"), 13],
  ] as [L, number][],
  hp: 77,
  hpMax: 85,
  ac: -2,
};

export const SAVES: [L, string][] = [
  [l("Paralysie/Poison", "Paralyze/Poison"), "10+"],
  [l("Bâton/Sceptre/Baguette", "Rod/Staff/Wand"), "8+"],
  [l("Pétrification/Métamorphose", "Petrify/Polymorph"), "9+"],
  [l("Souffle", "Breath Weapon"), "13+"],
  [l("Sorts", "Spells"), "9+"],
];

export const CATEGORIES: { id: Category | "all"; name: L }[] = [
  { id: "all", name: l("Tout", "All") },
  { id: "weapon", name: l("Armes", "Weapons") },
  { id: "ammunition", name: l("Munitions", "Ammunition") },
  { id: "armour", name: l("Armures", "Armour") },
  { id: "shield", name: l("Boucliers", "Shields") },
  { id: "magic", name: l("Magie", "Magic") },
  { id: "gear", name: l("Équipement", "Gear") },
];

export const CATEGORY_NAME: Record<Category, L> = {
  weapon: l("Arme", "Weapon"),
  ammunition: l("Munition", "Ammunition"),
  armour: l("Armure", "Armour"),
  shield: l("Bouclier", "Shield"),
  magic: l("Objet magique", "Magic item"),
  gear: l("Équipement", "Gear"),
};

export const POSSESSIONS: Piece[] = [
  {
    id: "chain",
    name: l("Cotte de mailles", "Chain mail"),
    category: "armour",
    kind: l("CA 5", "AC 5"),
    stats: l("Talents : métal · type : mailles", "Skills: metal · type: chain"),
    state: "worn",
    review: true,
  },
  {
    id: "ring",
    name: l("Anneau de protection +2", "Ring of protection +2"),
    category: "magic",
    kind: l("Anneau", "Ring"),
    stats: l("CA −2 sans armure magique · sauvegardes +2", "AC −2 without magic armour · saves +2"),
    state: "worn",
  },
  {
    id: "gem",
    name: l("Gemme de pouvoir", "Power Gem"),
    category: "magic",
    kind: l("Divers", "Misc"),
    stats: l("Sauvegardes +1", "Saves +1"),
    state: "worn",
    count: { value: 1, max: 1, behaviour: "daily" },
  },
  {
    id: "longsword",
    name: l("Épée longue +2", "Long sword +2"),
    category: "weapon",
    kind: l("Mêlée", "Melee"),
    stats: l("1d8 · G 1d12 · +2/+2 · vitesse 3", "1d8 · L 1d12 · +2/+2 · speed 3"),
    state: "wielded",
  },
  {
    id: "shortbow",
    name: l("Arc court +2", "Short bow +2"),
    category: "weapon",
    kind: l("Tir · deux mains", "Missile · two-handed"),
    stats: l("tire des flèches · +2/+2 · vitesse 5", "fires arrows · +2/+2 · speed 5"),
    state: "wielded",
  },
  {
    id: "dagger",
    name: l("Dague +3", "Dagger +3"),
    category: "weapon",
    kind: l("Mêlée", "Melee"),
    stats: l("1d4 · G 1d3 · +3/+3 · vitesse 0", "1d4 · L 1d3 · +3/+3 · speed 0"),
    state: "wielded",
  },
  {
    id: "arrows1",
    name: l("Flèches +1", "Arrows +1"),
    category: "ammunition",
    kind: l("Flèche", "Arrow"),
    stats: l("1d6 · G 1d6 · +1", "1d6 · L 1d6 · +1"),
    state: "carried",
    count: { value: 18, behaviour: "stock" },
  },
  {
    id: "arrows2",
    name: l("Flèches +2", "Arrows +2"),
    category: "ammunition",
    kind: l("Flèche", "Arrow"),
    stats: l("1d6 · G 1d6 · +2", "1d6 · L 1d6 · +2"),
    state: "carried",
    count: { value: 16, behaviour: "stock" },
  },
  {
    id: "wand",
    name: l("Baguette de foudre", "Wand of lightning"),
    category: "magic",
    kind: l("Baguette", "Wand"),
    stats: l("6d6", "6d6"),
    state: "carried",
    count: { value: 4, behaviour: "charges" },
    dice: "6d6",
  },
  {
    id: "flasks",
    name: l("Flasques de feu", "Fire flasks"),
    category: "gear",
    kind: l("Projectile", "Thrown"),
    stats: l("5d6", "5d6"),
    state: "carried",
    count: { value: 5, behaviour: "stock" },
    dice: "5d6",
  },
  {
    id: "bastard",
    name: l("Épée bâtarde +2", "Bastard sword +2"),
    category: "weapon",
    kind: l("Mêlée · deux mains", "Melee · two-handed"),
    stats: l("2d4 · G 2d8 · +2/+2 · vitesse 6", "2d4 · L 2d8 · +2/+2 · speed 6"),
    state: "carried",
  },
  {
    id: "mace",
    name: l("Masse d'armes +1", "Mace +1"),
    category: "weapon",
    kind: l("Mêlée", "Melee"),
    stats: l("1d6 · G 1d6 · +1/+1 · vitesse ?", "1d6 · L 1d6 · +1/+1 · speed ?"),
    state: "carried",
    review: true,
  },
  {
    id: "bag",
    name: l("Sac sans fond", "Bag of holding"),
    category: "gear",
    kind: l("Non classé", "Uncategorised"),
    stats: l("ancien type : « sac »", "old type: “bag”"),
    state: "carried",
    review: true,
  },
  {
    id: "rope",
    name: l("Corde en soie (15 m)", "Silk rope (50 ft)"),
    category: "gear",
    kind: l("Équipement", "Gear"),
    stats: l("", ""),
    state: "carried",
  },
];

/** The conversion's review list (ADR 0033 step 27): the piece, its reason, the quick way to settle it. */
export const REVIEW: { id: string; reason: L; choices?: L[] }[] = [
  {
    id: "chain",
    reason: l(
      "Armure déduite de votre ancienne armure de talents (« métal ») : laquelle portez-vous ?",
      "Armour guessed from your old skill armour (“metal”): which one do you wear?",
    ),
    choices: [
      l("Cotte de mailles · CA 5", "Chain mail · AC 5"),
      l("Broigne · CA 7", "Ring mail · AC 7"),
      l("Armure d'écailles · CA 6", "Scale mail · AC 6"),
      l("Armure de peau · CA 6", "Hide armour · AC 6"),
    ],
  },
  {
    id: "mace",
    reason: l(
      "Arme non reconnue (type « autre mêlée ») : choisissez-la dans le catalogue pour sa vitesse et son type de dégâts.",
      "Weapon not recognised (type “other melee”): pick it in the catalogue for its speed and damage type.",
    ),
  },
  {
    id: "bag",
    reason: l(
      "Catégorie inconnue (ancien type « sac ») : rangée en équipement.",
      "Unknown category (old type “bag”): filed as gear.",
    ),
  },
];

/** The AC breakdown (ADR 0033 step 10), one row per term with its source. */
export const AC_ROWS: { term: L; source: L; value: string; muted?: boolean }[] = [
  { term: l("Base", "Base"), source: l("Cotte de mailles", "Chain mail"), value: "5" },
  { term: l("Magie de l'armure", "Armour magic"), source: l("aucune", "none"), value: "—", muted: true },
  { term: l("Bouclier", "Shield"), source: l("aucun", "none"), value: "—", muted: true },
  { term: l("Dextérité", "Dexterity"), source: l("17", "17"), value: "−3" },
  { term: l("Protection", "Protection"), source: l("Anneau de protection +2", "Ring of protection +2"), value: "−2" },
  { term: l("Effets", "Effects"), source: l("Protection contre le mal", "Protection from evil"), value: "−2" },
];

/** Catalogue armour rows (PHB Table 44 / 46), with the bard's class warning (ADR 0033 step 20). */
export const CATALOGUE_ARMOUR: { name: L; ac: number; skill: L; row: L; warn?: boolean }[] = [
  { name: l("Armure matelassée", "Padded armour"), ac: 8, skill: l("Clouté", "Studded"), row: l("Cuir", "Leather") },
  { name: l("Armure de cuir", "Leather armour"), ac: 8, skill: l("Cuir", "Leather"), row: l("Cuir", "Leather") },
  { name: l("Armure de cuir clouté", "Studded leather"), ac: 7, skill: l("Clouté", "Studded"), row: l("Cuir clouté", "Studded leather") },
  { name: l("Broigne", "Ring mail"), ac: 7, skill: l("Métal", "Metal"), row: l("Anneaux", "Ring") },
  { name: l("Armure de peau", "Hide armour"), ac: 6, skill: l("Métal", "Metal"), row: l("Cuir", "Leather") },
  { name: l("Armure d'écailles", "Scale mail"), ac: 6, skill: l("Métal", "Metal"), row: l("Écailles", "Scale") },
  { name: l("Brigandine", "Brigandine"), ac: 6, skill: l("Interdite", "Not allowed"), row: l("Brigandine", "Brigandine"), warn: true },
  { name: l("Cotte de mailles", "Chain mail"), ac: 5, skill: l("Métal", "Metal"), row: l("Mailles", "Chain") },
  { name: l("Cotte de mailles elfique", "Elven chain mail"), ac: 5, skill: l("Mailles elfiques", "Elven chain"), row: l("Mailles", "Chain") },
  { name: l("Armure à bandes", "Banded mail"), ac: 4, skill: l("Interdite", "Not allowed"), row: l("Bandes", "Banded"), warn: true },
  { name: l("Armure de plates de bronze", "Bronze plate mail"), ac: 4, skill: l("Interdite", "Not allowed"), row: l("Plates", "Plate"), warn: true },
  { name: l("Clibanion", "Splint mail"), ac: 4, skill: l("Interdite", "Not allowed"), row: l("Lamelles", "Splint"), warn: true },
  { name: l("Armure de plates", "Plate mail"), ac: 3, skill: l("Interdite", "Not allowed"), row: l("Plates", "Plate"), warn: true },
  { name: l("Harnois", "Field plate"), ac: 2, skill: l("Interdite", "Not allowed"), row: l("Harnois", "Field plate"), warn: true },
  { name: l("Armure de plates complète", "Full plate"), ac: 1, skill: l("Interdite", "Not allowed"), row: l("Plates complètes", "Full plate"), warn: true },
];

