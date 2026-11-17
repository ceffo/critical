// The Skills round's data. The non-weapon catalogue (ability, modifier, cost, groups) and the class
// groups are the repo's seeds (api/testdata/seeds/proficiencies.json); names are the app's locale
// files. Weapon names, class weapon lists, the skill bases and the race, Dexterity and armour terms
// are copied from the catalogue (format 24) for the four fixtures only.
import seeds from "../../../api/testdata/seeds/proficiencies.json";
import en from "@/i18n/locales/en/common.json";
import fr from "@/i18n/locales/fr/common.json";
import { LANG } from "./lang";

export type Ability = "str" | "dex" | "con" | "int" | "wis" | "cha";
export type Group =
  | "general"
  | "warrior"
  | "priest"
  | "rogue"
  | "wizard"
  | "psionicist";
export const GROUPS: Group[] = [
  "general",
  "warrior",
  "priest",
  "rogue",
  "wizard",
  "psionicist",
];

const loc = LANG === "fr" ? fr : en;
const names = loc.skills.names as Record<string, string>;
export const skillName = (id: string) => names[id] ?? id;

export const ABILITY: Record<Ability, [string, string]> = {
  str: ["FOR", "STR"],
  dex: ["DEX", "DEX"],
  con: ["CON", "CON"],
  int: ["INT", "INT"],
  wis: ["SAG", "WIS"],
  cha: ["CHA", "CHA"],
};
export const abil = (a: Ability) => ABILITY[a][LANG === "fr" ? 0 : 1];

export const GROUP_NAME: Record<Group, [string, string]> = {
  general: ["Générales", "General"],
  warrior: ["Combattant", "Warrior"],
  priest: ["Prêtre", "Priest"],
  rogue: ["Roublard", "Rogue"],
  wizard: ["Magicien", "Wizard"],
  psionicist: ["Psioniste", "Psionicist"],
};
export const groupName = (g: Group) => GROUP_NAME[g][LANG === "fr" ? 0 : 1];

export interface NonWeapon {
  id: string;
  ability: Ability;
  modifier: number;
  slots: number;
  groups: Group[];
}
export const CATALOGUE: NonWeapon[] = (seeds.nonweapon as NonWeapon[])
  .slice()
  .sort((a, b) => skillName(a.id).localeCompare(skillName(b.id), LANG));

export const classGroups = (cls: string): Group[] =>
  (seeds.crossovers.find((c) => c.class === cls)?.groups ?? []) as Group[];

export interface Weapon {
  code: string;
  fr: string;
  en: string;
  family: "sword" | "axe" | "blunt" | "polearm" | "bow" | "crossbow" | "thrown" | "other";
  type: string;
}
const W = (
  code: string,
  fr: string,
  en: string,
  family: Weapon["family"],
  type = "other",
): Weapon => ({ code, fr, en, family, type });
export const WEAPONS: Weapon[] = [
  W("bastard_sword_one_handed", "Épée bâtarde à une main", "Bastard sword, one-handed", "sword"),
  W("bastard_sword_two_handed", "Épée bâtarde à deux mains", "Bastard sword, two-handed", "sword"),
  W("broad_sword", "Épée large", "Broad sword", "sword"),
  W("khopesh", "Khopesh", "Khopesh", "sword"),
  W("long_sword", "Épée longue", "Long sword", "sword", "long_sword"),
  W("scimitar", "Cimeterre", "Scimitar", "sword"),
  W("short_sword", "Épée courte", "Short sword", "sword", "short_sword"),
  W("two_handed_sword", "Épée à deux mains", "Two-handed sword", "sword"),
  W("dagger", "Dague", "Dagger", "sword", "dagger"),
  W("knife", "Couteau", "Knife", "sword"),
  W("battle_axe", "Hache d'armes", "Battle axe", "axe"),
  W("hand_axe", "Hachette", "Hand axe", "axe"),
  W("club", "Gourdin", "Club", "blunt"),
  W("footmans_flail", "Fléau de fantassin", "Footman's flail", "blunt"),
  W("footmans_mace", "Masse de fantassin", "Footman's mace", "blunt"),
  W("horsemans_mace", "Masse de cavalier", "Horseman's mace", "blunt"),
  W("morning_star", "Étoile du matin", "Morning star", "blunt"),
  W("quarterstaff", "Bâton", "Quarterstaff", "blunt"),
  W("warhammer", "Marteau de guerre", "Warhammer", "blunt"),
  W("whip", "Fouet", "Whip", "other"),
  W("awl_pike", "Pique", "Awl pike", "polearm"),
  W("bardiche", "Bardiche", "Bardiche", "polearm"),
  W("glaive", "Doloire", "Glaive", "polearm"),
  W("halberd", "Hallebarde", "Halberd", "polearm"),
  W("partisan", "Pertuisane", "Partisan", "polearm"),
  W("spear", "Lance de fantassin", "Spear", "polearm"),
  W("trident", "Trident", "Trident", "polearm"),
  W("long_bow", "Arc long", "Long bow", "bow"),
  W("short_bow", "Arc court", "Short bow", "bow", "short_bow"),
  W("composite_long_bow", "Arc long composite", "Composite long bow", "bow"),
  W("composite_short_bow", "Arc court composite", "Composite short bow", "bow", "short_bow"),
  W("hand_crossbow", "Arbalète de poing", "Hand crossbow", "crossbow", "hand_crossbow"),
  W("light_crossbow", "Arbalète légère", "Light crossbow", "crossbow"),
  W("heavy_crossbow", "Arbalète lourde", "Heavy crossbow", "crossbow"),
  W("dart", "Dard", "Dart", "thrown", "dart"),
  W("javelin", "Javelot", "Javelin", "thrown"),
  W("sling", "Fronde", "Sling", "thrown", "sling"),
  W("blowgun", "Sarbacane", "Blowgun", "thrown"),
];
export const weaponName = (w: Weapon) => (LANG === "fr" ? w.fr : w.en);
export const weapon = (code: string) => WEAPONS.find((w) => w.code === code)!;

export const FAMILY: Record<Weapon["family"], [string, string]> = {
  sword: ["Épées et lames", "Swords and blades"],
  axe: ["Haches", "Axes"],
  blunt: ["Armes contondantes", "Blunt weapons"],
  polearm: ["Armes d'hast et lances", "Polearms and spears"],
  bow: ["Arcs", "Bows"],
  crossbow: ["Arbalètes", "Crossbows"],
  thrown: ["Armes de jet", "Thrown and missile"],
  other: ["Autres", "Other"],
};

/** The thief's weapons (catalogue: types and named weapons). */
const THIEF_TYPES = ["dagger", "dart", "hand_crossbow", "short_bow", "sling", "short_sword", "long_sword"];
const THIEF_NAMED = ["club", "knife", "broad_sword", "quarterstaff"];
export const thiefMay = (w: Weapon) =>
  THIEF_TYPES.includes(w.type) || THIEF_NAMED.includes(w.code);

export interface RogueSkill {
  id: string;
  base: number;
  points: number;
  race: number;
  dex: number;
  armour: number;
}
export const percent = (s: RogueSkill) =>
  Math.min(95, Math.max(0, s.base + s.points + s.race + s.dex + s.armour));

export interface SkillSet {
  id: string;
  name: [string, string];
  armour: [string, string];
  available: number;
  formula: [string, string];
  perSkillCap: number;
  capFormula?: [string, string];
  skills: RogueSkill[];
}

export interface Pick {
  id: string;
  spent: number;
  free?: boolean;
}

export interface Fixture {
  name: string;
  race: [string, string];
  classes: { name: [string, string]; level: number }[];
  /** The groups the proficiencies are at class cost; one class's ids, or the union. */
  classIds: string[];
  abilities: Record<Ability, number>;
  thac0: number;
  hp: number;
  hpMax: number;
  ac: number;
  saves: [[string, string], string][];
  weaponSlots: { available: number; formula: [string, string]; penalty: number };
  weaponPicks: { code: string; slots: number; spec?: boolean }[];
  nonWeaponSlots: { available: number; formula: [string, string] };
  picks: Pick[];
  languages: { code: [string, string]; native?: boolean; paid?: boolean }[];
  sets: SkillSet[];
}

const SAVES = (a: string, b: string, c: string, d: string, e: string): Fixture["saves"] => [
  [["Paralysie/Poison", "Paralysis/Poison"], a],
  [["Bâton/Sceptre/Baguette", "Rod/Staff/Wand"], b],
  [["Pétrification/Métamorphose", "Petrification/Polymorph"], c],
  [["Souffle", "Breath weapon"], d],
  [["Sorts", "Spells"], e],
];

export const BARD: Fixture = {
  name: "Aubépine Chantelune",
  race: ["Demi-elfe", "Half-elf"],
  classes: [{ name: ["Barde", "Bard"], level: 5 }],
  classIds: ["bard"],
  abilities: { str: 12, dex: 17, con: 14, int: 14, wis: 9, cha: 16 },
  thac0: 18,
  hp: 23,
  hpMax: 26,
  ac: 6,
  saves: SAVES("12", "13", "11", "15", "14"),
  weaponSlots: {
    available: 3,
    formula: ["2 au départ + 1 (niveau 5, un tous les 4)", "2 at start + 1 (level 5, one every 4)"],
    penalty: -3,
  },
  weaponPicks: [
    { code: "long_sword", slots: 1 },
    { code: "short_bow", slots: 1 },
    { code: "dagger", slots: 1 },
  ],
  nonWeaponSlots: {
    available: 8,
    formula: ["3 au départ + 1 (niveau 5) + 4 (INT 14)", "3 at start + 1 (level 5) + 4 (INT 14)"],
  },
  picks: [
    { id: "musical_instrument", spent: 0, free: true },
    { id: "singing", spent: 0, free: true },
    { id: "reading_writing", spent: 0, free: true },
    { id: "local_history", spent: 0, free: true },
    { id: "ventriloquism", spent: 1 },
    { id: "riding_land", spent: 1 },
    { id: "dancing", spent: 1 },
    { id: "healing", spent: 3 },
  ],
  languages: [
    { code: ["Commun", "Common"], native: true },
    { code: ["Elfe", "Elvish"], native: true },
    { code: ["Nain", "Dwarvish"], paid: true },
  ],
  sets: [
    {
      id: "bard",
      name: ["Talents de barde", "Bard skills"],
      armour: ["Cuir", "Leather"],
      available: 80,
      formula: ["20 + 15 × 4 (niveau 5)", "20 + 15 × 4 (level 5)"],
      perSkillCap: 0,
      skills: [
        { id: "climb_walls", base: 50, points: 15, race: 0, dex: 0, armour: 0 },
        { id: "detect_noise", base: 20, points: 20, race: 0, dex: 0, armour: 0 },
        { id: "pick_pockets", base: 10, points: 20, race: 10, dex: 5, armour: 0 },
        { id: "read_languages", base: 5, points: 10, race: 0, dex: 0, armour: 0 },
      ],
    },
  ],
};

const thiefSkills = (pts: number[], race: number[], dex: number[], armour: number[]): RogueSkill[] =>
  ["pick_pockets", "open_locks", "find_remove_traps", "move_silently", "hide_in_shadows", "detect_noise", "climb_walls", "read_languages"].map(
    (id, i) => ({
      id,
      base: [15, 10, 5, 10, 5, 15, 60, 0][i],
      points: pts[i],
      race: race[i],
      dex: dex[i],
      armour: armour[i],
    }),
  );

export const THIEF: Fixture = {
  name: "Pipin Bruyère-des-Combes",
  race: ["Halfelin", "Halfling"],
  classes: [{ name: ["Voleur", "Thief"], level: 4 }],
  classIds: ["thief"],
  abilities: { str: 10, dex: 18, con: 15, int: 13, wis: 11, cha: 12 },
  thac0: 19,
  hp: 17,
  hpMax: 17,
  ac: 4,
  saves: SAVES("12", "12", "11", "15", "13"),
  weaponSlots: {
    available: 2,
    formula: ["2 au départ (niveau 4, un tous les 4 dès le 5)", "2 at start (level 4, one every 4 from 5)"],
    penalty: -3,
  },
  weaponPicks: [
    { code: "short_sword", slots: 1 },
    { code: "sling", slots: 1 },
  ],
  nonWeaponSlots: {
    available: 6,
    formula: ["3 au départ + 0 (niveau 4) + 3 (INT 13)", "3 at start + 0 (level 4) + 3 (INT 13)"],
  },
  picks: [
    { id: "tightrope_walking", spent: 1 },
    { id: "appraising", spent: 1 },
    { id: "disguise", spent: 1 },
    { id: "jumping", spent: 1 },
  ],
  languages: [
    { code: ["Commun", "Common"], native: true },
    { code: ["Halfelin", "Halfling"], native: true },
    { code: ["Gnome", "Gnome"], paid: true },
  ],
  sets: [
    {
      id: "thief",
      name: ["Talents de voleur", "Thief skills"],
      armour: ["Cuir", "Leather"],
      available: 150,
      formula: ["60 + 30 × 3 (niveau 4)", "60 + 30 × 3 (level 4)"],
      perSkillCap: 75,
      capFormula: ["30 + 15 × 3", "30 + 15 × 3"],
      skills: thiefSkills(
        [30, 25, 10, 25, 25, 5, 0, 0],
        [5, 5, 5, 10, 15, 5, -15, -5],
        [10, 15, 5, 10, 10, 0, 0, 0],
        [0, 0, 0, 0, 0, 0, 0, 0],
      ),
    },
  ],
};

export const FIGHTER: Fixture = {
  name: "Brunhilde von Eisenwald",
  race: ["Humaine", "Human"],
  classes: [{ name: ["Guerrière", "Fighter"], level: 6 }],
  classIds: ["fighter"],
  abilities: { str: 17, dex: 14, con: 16, int: 10, wis: 11, cha: 9 },
  thac0: 15,
  hp: 49,
  hpMax: 52,
  ac: 2,
  saves: SAVES("11", "13", "12", "13", "14"),
  weaponSlots: {
    available: 6,
    formula: ["4 au départ + 2 (niveau 6, un tous les 3)", "4 at start + 2 (level 6, one every 3)"],
    penalty: -2,
  },
  weaponPicks: [
    { code: "long_sword", slots: 2, spec: true },
    { code: "dagger", slots: 1 },
    { code: "long_bow", slots: 1 },
    { code: "spear", slots: 1 },
    { code: "footmans_mace", slots: 1 },
  ],
  nonWeaponSlots: {
    available: 7,
    formula: ["3 au départ + 2 (niveau 6) + 2 (INT 10)", "3 at start + 2 (level 6) + 2 (INT 10)"],
  },
  picks: [
    { id: "riding_land", spent: 1 },
    { id: "blind_fighting", spent: 2 },
    { id: "endurance", spent: 2 },
    { id: "armorer", spent: 2 },
  ],
  languages: [{ code: ["Commun", "Common"], native: true }],
  sets: [],
};

export const MULTI: Fixture = {
  name: "Taern Feuillesombre",
  race: ["Demi-elfe", "Half-elf"],
  classes: [
    { name: ["Guerrier", "Fighter"], level: 4 },
    { name: ["Voleur", "Thief"], level: 5 },
  ],
  classIds: ["fighter", "thief"],
  abilities: { str: 16, dex: 17, con: 14, int: 12, wis: 10, cha: 11 },
  thac0: 17,
  hp: 24,
  hpMax: 28,
  ac: 4,
  saves: SAVES("13", "14", "12", "16", "15"),
  weaponSlots: {
    available: 5,
    formula: [
      "Guerrier : 4 au départ + 1 (niveau 4, un tous les 3)",
      "Fighter: 4 at start + 1 (level 4, one every 3)",
    ],
    penalty: -2,
  },
  weaponPicks: [
    { code: "long_sword", slots: 1 },
    { code: "short_bow", slots: 1 },
    { code: "dagger", slots: 1 },
    { code: "spear", slots: 1 },
  ],
  nonWeaponSlots: {
    available: 7,
    formula: [
      "Guerrier : 3 + 1 (niveau 4, un tous les 3) · Voleur : 3 + 1 (niveau 5) → 4, + 3 (INT 12)",
      "Fighter: 3 + 1 (level 4, one every 3) · Thief: 3 + 1 (level 5) → 4, + 3 (INT 12)",
    ],
  },
  picks: [
    { id: "tracking", spent: 2 },
    { id: "endurance", spent: 2 },
    { id: "jumping", spent: 1 },
    { id: "tightrope_walking", spent: 1 },
  ],
  languages: [
    { code: ["Commun", "Common"], native: true },
    { code: ["Elfe", "Elvish"], native: true },
  ],
  sets: [
    {
      id: "thief",
      name: ["Talents de voleur", "Thief skills"],
      armour: ["Cuir clouté", "Studded leather"],
      available: 180,
      formula: ["60 + 30 × 4 (Voleur niveau 5)", "60 + 30 × 4 (Thief level 5)"],
      perSkillCap: 90,
      capFormula: ["30 + 15 × 4", "30 + 15 × 4"],
      skills: thiefSkills(
        [35, 20, 15, 30, 30, 10, 10, 0],
        [10, 0, 0, 0, 5, 0, 0, 0],
        [5, 10, 0, 5, 5, 0, 0, 0],
        [-30, -10, -10, -20, -20, -10, -30, 0],
      ),
    },
  ],
};

export const spentWeapon = (f: Fixture) => f.weaponPicks.reduce((n, p) => n + p.slots, 0);
export const spentNonWeapon = (f: Fixture) =>
  f.picks.reduce((n, p) => n + p.spent, 0) + f.languages.filter((l) => l.paid).length;
export const spentPoints = (s: SkillSet) => s.skills.reduce((n, k) => n + k.points, 0);
/** Cost of a proficiency for the fixture: its slots at class cost, one more outside the class's groups. */
export const costFor = (f: Fixture, p: NonWeapon) => {
  const own = new Set(f.classIds.flatMap(classGroups));
  return p.groups.some((g) => own.has(g)) ? p.slots : p.slots + 1;
};
export const prof = (id: string) => CATALOGUE.find((p) => p.id === id)!;
/** Something left to spend in any section: what lights the rail's mark. */
export const hasPending = (f: Fixture) =>
  spentWeapon(f) < f.weaponSlots.available ||
  spentNonWeapon(f) < f.nonWeaponSlots.available ||
  f.sets.some((s) => spentPoints(s) < s.available);
