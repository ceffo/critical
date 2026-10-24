// The psionics mock-ups' data: Joran Veil, the table's human thief/psionicist (ADR 0042 step 17,
// ADR 0043 step 19), thief 7 / psionicist 8, Wisdom 17. Costs and scores are the catalogue's
// (format 13: Wisdom plus the power's modifier); the figures that depend on the rules engine
// (Table 14 cells, contact cost) are mock values for layout, not rulings.

export type L = { fr: string; en: string };
export const l = (fr: string, en: string): L => ({ fr, en });

export const CHAR = {
  name: "Joran Veil",
  classes: [
    { name: l("Voleur", "Thief"), level: 7 },
    { name: l("Psioniste", "Psionicist"), level: 8 },
  ],
  abilities: [
    [l("FOR", "STR"), 11],
    [l("DEX", "DEX"), 17],
    [l("CON", "CON"), 16],
    [l("INT", "INT"), 15],
    [l("SAG", "WIS"), 17],
    [l("CHA", "CHA"), 10],
    [l("TAC0", "THAC0"), 16],
  ] as [L, number][],
  hp: 48,
  hpMax: 52,
  ac: 4,
  psp: 71,
  pspMax: 109,
};

export const SAVES: [L, string][] = [
  [l("Paralysie/Poison", "Paralyze/Poison"), "11+"],
  [l("Bâton/Sceptre/Baguette", "Rod/Staff/Wand"), "12+"],
  [l("Pétrification/Métamorphose", "Petrify/Polymorph"), "11+"],
  [l("Souffle", "Breath Weapon"), "15+"],
  [l("Sorts", "Spells"), "11+"],
];

export type Kind = "attack" | "defence" | "science" | "devotion";

export interface Power {
  id: string;
  name: L;
  discipline: L;
  kind: Kind;
  /** Initial cost in PSPs; "contact" for a contact-cost power. */
  cost: number | "contact" | "variable";
  /** Maintenance per round, when the power can be kept up. */
  upkeep?: number;
  /** The power score, Wisdom 17 plus the power's modifier. */
  score: number;
}

const TELE = l("Télépathie", "Telepathy");
const CLAIR = l("Clairsentience", "Clairsentience");
const PK = l("Psychokinésie", "Psychokinesis");

export const POWERS: Power[] = [
  { id: "mind_thrust", name: l("Pression mentale", "Mind thrust"), discipline: TELE, kind: "attack", cost: 2, score: 15 },
  { id: "ego_whip", name: l("Fouet de l'ego", "Ego whip"), discipline: TELE, kind: "attack", cost: 4, score: 14 },
  { id: "psionic_blast", name: l("Souffle psionique", "Psionic blast"), discipline: TELE, kind: "attack", cost: 10, score: 12 },
  { id: "mind_blank", name: l("Néant spirituel", "Mind blank"), discipline: TELE, kind: "defence", cost: 0, score: 10 },
  { id: "thought_shield", name: l("Bouclier de pensée", "Thought shield"), discipline: TELE, kind: "defence", cost: 1, score: 14 },
  { id: "mental_barrier", name: l("Barrière mentale", "Mental barrier"), discipline: TELE, kind: "defence", cost: 3, score: 14 },
  { id: "contact", name: l("Contact", "Contact"), discipline: TELE, kind: "devotion", cost: "variable", upkeep: 1, score: 17 },
  { id: "mind_link", name: l("Lien spirituel", "Mind link"), discipline: TELE, kind: "science", cost: "contact", upkeep: 8, score: 12 },
  { id: "aura_sight", name: l("Vision de l'Aura", "Aura sight"), discipline: CLAIR, kind: "science", cost: 9, upkeep: 9, score: 12 },
  { id: "telekinesis", name: l("Télékinésie", "Telekinesis"), discipline: PK, kind: "science", cost: "variable", upkeep: 1, score: 14 },
];

export const power = (id: string): Power => POWERS.find((p) => p.id === id)!;
