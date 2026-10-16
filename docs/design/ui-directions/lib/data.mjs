// Sample data shared by every direction page: the Bard at level 15 as in seeds/character.json,
// his real weapons, items and spellbook, a turn in progress with three effects, and a journal
// of about fifteen mixed entries. Spell texts are short stats only; descriptions are placeholder
// prose (the catalogue's text is never copied into this repo).

export const character = {
  name: "Kelsen Balstriel",
  klass: "Bard",
  level: 15,
  abilities: [
    ["STR", 12],
    ["DEX", 17],
    ["CON", 16],
    ["INT", 14],
    ["WIS", 8],
    ["CHA", 16],
  ],
  thac0: 13,
  hp: { current: 70, max: 85, floor: -10, lastDelta: -9, lastSource: "Orc axe" },
  saves: [
    { name: "Paralyze / Poison", short: "Paralyze", target: "10+", last: null },
    { name: "Rod / Staff / Wand", short: "Rod / Wand", target: "8+", last: null },
    { name: "Petrify / Polymorph", short: "Petrify", target: "9+", last: null },
    { name: "Breath Weapon", short: "Breath", target: "13+", last: { roll: 15, ok: true } },
    { name: "Spells", short: "Spells", target: "9+", last: null },
  ],
};

export const turn = { number: 3, initiative: 4 };

export const effects = [
  { name: "Haste", kind: "timed", left: 7, start: 2, label: "7 turns" },
  { name: "Stoneskin", kind: "charges", charges: 6, label: "6 charges · until spent" },
  { name: "Power Gem", kind: "waiting", label: "Waiting · needs STR 18" },
];

// The latest result: an attack with the weapon in hand against a known AC.
export const lastResult = {
  kind: "Attack",
  subject: "Long Sword +2",
  when: "T3 · 21:31",
  d20: 16,
  hitsAc: -5,
  targetAc: 5,
  verdict: "Hit",
  damageDie: "1d8",
  damageRoll: 6,
  damageBonus: 2,
  damageTotal: 8,
  calculation: [
    ["d20", "16"],
    ["Base THAC0", "13"],
    ["Weapon +2", "−2"],
    ["Effective THAC0", "11"],
    ["Hits AC", "11 − 16 = −5"],
  ],
};

// Newest first; `turn` 0 is out of combat.
export const journal = [
  { turn: 3, icon: "sword", text: "Long Sword +2 vs AC 5: hit, 8 damage", time: "21:31", value: "8", die: 8, tone: "highlight", current: true },
  { turn: 3, icon: "heart", text: "Orc axe", time: "21:30", value: "−9", tone: "danger" },
  { turn: 3, icon: "bolt", text: "Initiative", time: "21:29", value: "4", die: 10 },
  { turn: 2, icon: "dice", text: "Free roll 3d6+5", time: "21:28", value: "17", die: 6, reroll: true },
  { turn: 2, icon: "shield", text: "Stoneskin absorbed a hit (6 left)", time: "21:27" },
  { turn: 2, icon: "sparkle", text: "Cast Haste · 3 rd + 1 rd/level", time: "21:26" },
  { turn: 2, icon: "bolt", text: "Initiative", time: "21:25", value: "7", die: 10 },
  { turn: 1, icon: "bow", text: "Short Bow +2 vs AC 3: miss (d20 4)", time: "21:24", value: "4", die: 20, tone: "muted" },
  { turn: 1, icon: "bow", text: "Short Bow +2 vs AC 3: hit, 5 damage", time: "21:23", value: "5", die: 6, tone: "highlight" },
  { turn: 1, icon: "shield", text: "Breath Weapon save: 15 vs 13+", time: "21:22", value: "15", die: 20, tone: "success" },
  { turn: 1, icon: "flask", text: "Fire flask 5d6", time: "21:21", value: "19", die: 6, tone: "highlight" },
  { turn: 1, icon: "bolt", text: "Initiative", time: "21:20", value: "2", die: 10 },
  { turn: 0, icon: "sparkle", text: "Stoneskin cast on me · 9 layers", time: "20:58" },
  { turn: 0, icon: "heart", text: "Potion of healing", time: "20:45", value: "+12", tone: "success" },
  { turn: 0, icon: "moon", text: "Long rest · spell slots reset", time: "20:30" },
  { turn: 0, icon: "gem", text: "Power Gem: Dispel Magic added", time: "20:12" },
];

export const weapons = {
  inHand: {
    name: "Long Sword +2",
    kind: "Melee",
    damageSm: "1d8",
    damageL: "1d12",
    bonus: "+2 / +2",
    effectiveThac0: 11,
    needs: "needs 6 vs AC 5",
  },
  previous: { name: "Short Bow +2", dice: "1d6" },
  others: ["Bastard Sword +2", "Dagger +3", "Mace +1", "Darts"],
  target: { ac: 5, size: "S / M", situational: 0 },
};

// The act Spells tab: one group per level, a pip per slot, rows with the damage a cast rolls.
export const spellLevels = [
  {
    level: 1,
    slots: 4,
    left: 3,
    spells: [
      { name: "Magic Missile", dice: "5 × 1d4+1 force" },
      { name: "Burning Hands", dice: "15 points fire" },
      { name: "Chromatic Orb", dice: "1d4 + special" },
      { name: "Detect Magic", used: true },
    ],
  },
  {
    level: 2,
    slots: 4,
    left: 4,
    spells: [
      { name: "Mirror Image" },
      { name: "Improved Phantasmal Force" },
      { name: "Tasha's Uncontrollable Hideous Laughter" },
      { name: "Melf's Acid Arrow", dice: "2d4 acid per round, 6 rounds" },
    ],
  },
  {
    level: 3,
    slots: 3,
    left: 2,
    spells: [
      { name: "Fireball", dice: "10d6 fire" },
      { name: "Haste", used: true },
      { name: "Protection From Evil, 10' Radius" },
    ],
  },
  {
    level: 4,
    slots: 2,
    left: 2,
    spells: [
      { name: "Minor Globe of Invulnerability" },
      { name: "Improved Invisibility" },
    ],
  },
  {
    level: 5,
    slots: 1,
    left: 1,
    spells: [{ name: "Leomund's Lamentable Belaborment" }],
  },
];

export const items = [
  { name: "Fire flasks", sub: "Stock · 5", dice: "5d6", verb: "Roll" },
  { name: "Wand of lightning", sub: "Charges · 4", dice: "6d6", verb: "Roll" },
  { name: "Power Gem: Dispel Magic", sub: "Daily · 1 / 1", dice: "", verb: "Use" },
];

export const skills = {
  proficiencies: [
    ["Musical instrument", "DEX 17 −1", 16],
    ["Tumbling", "DEX 17", 17],
    ["Ventriloquism", "INT 14 −2", 12],
    ["Riding, land-based", "WIS 8 +3", 11],
  ],
  bard: [
    ["Climb walls", 85],
    ["Detect noise", 90],
    ["Pick pockets", 50],
    ["Read languages", 75],
  ],
};

export const dice = {
  formula: "3d6+5",
  favourites: [
    ["Lightning bolt", "10d6"],
    ["Magic missile", "5d4+5"],
    ["Fire flask", "5d6"],
  ],
};

// The spell lookup: a search for "detect" in the Wizard list, the long-named spell selected.
export const lookup = {
  query: "detect",
  klass: "Wizard",
  count: 7,
  // Each result carries the stats a browse table shows (PHB / Tome of Magic values).
  results: [
    { level: 1, name: "Detect Magic", school: "Divination", range: "0", duration: "2 rds/level", casting: "1", save: "None" },
    { level: 1, name: "Detect Undead", school: "Necromancy", range: "0", duration: "3 turns", casting: "1 round", save: "None" },
    { level: 1, name: "Detect Secret Passages and Portals", school: "Divination", range: "0", duration: "1 turn", casting: "1", save: "None", selected: true },
    { level: 1, name: "Comprehend Languages", school: "Alteration", range: "Touch", duration: "5 rds/level", casting: "1 round", save: "None" },
    { level: 2, name: "Detect Evil", school: "Divination", range: "0", duration: "5 rds/level", casting: "2", save: "None" },
    { level: 2, name: "Detect Invisibility", school: "Divination", range: "10 yds/level", duration: "5 rds/level", casting: "2", save: "None" },
    { level: 4, name: "Detect Scrying", school: "Divination", range: "0", duration: "1d6 turns + 1 turn/level", casting: "4", save: "None" },
  ],
  // Facet counts for a browse sidebar: the Wizard list filtered on "detect".
  facets: {
    klass: [["Mine", 1], ["Wizard", 7], ["Priest", 4], ["All", 11]],
    levels: [[1, 4], [2, 2], [4, 1]],
    schools: [["Any school", 7], ["Divination", 5], ["Alteration", 1], ["Necromancy", 1]],
  },
  selected: {
    name: "Detect Secret Passages and Portals",
    meta: "Wizard · Level 1 · Divination · V, S",
    rows: [
      ["Range", "0", "0"],
      ["Duration", "1 turn", "1 turn"],
      ["Area", "150 × 10 ft", "10 x 10 ft. area/level"],
      ["Casting time", "1", "1"],
      ["Save", "None", "None"],
    ],
    diceRows: [],
    // Placeholder prose: the catalogue's description is not reproduced in this repo.
    description: [
      "Description placeholder. The real text comes from the spell catalogue at run time and is not copied into the design pages; this paragraph only shows how much room the description gets and how it wraps at the chosen measure.",
      "A second paragraph stands in for the usual rules detail: what the spell reveals, how far it reaches, what blocks it and what the caster must do to keep it going. Tables in the catalogue render inside this pane with their own borders.",
      "A third paragraph shows that a long description scrolls inside the pane while the stats above stay put, so a quick lookup never loses the numbers it came for.",
    ],
  },
};

export const xp = {
  total: "1,293,418",
  nextLevel: 16,
  toGo: "26,582",
  floor: "1,100,000",
  ceiling: "1,320,000",
  pct: 87,
  levels: [
    [13, "660K"],
    [14, "880K"],
    [15, "1.1M"],
    [16, "1.32M"],
    [17, "1.54M"],
    [18, "1.76M"],
    [19, "1.98M"],
    [20, "2.2M"],
  ],
  preview: "Add 2,750",
  ledger: [
    { reason: "Orc warband, Thornwood", meta: "12 Oct · S41", base: "9,000", bonus: "900", total: "9,900", fresh: true },
    { reason: "Rescued the merchant caravan", meta: "12 Oct · S41", base: "4,500", bonus: "450", total: "4,950", fresh: true },
    { reason: "Performance at the Gilded Harp", meta: "5 Oct · S40", base: "1,200", bonus: "120", total: "1,320" },
    { reason: "Troll bridge", meta: "5 Oct · S40", base: "6,800", bonus: "680", total: "7,480" },
    { reason: "Starting XP (character sheet)", meta: "", base: "", bonus: "", total: "1,269,768", readonly: true },
  ],
};
