import { parseWhole } from "@/lib/wholeNumber";
import {
  ABILITIES,
  type AbilityCode,
  type CreationOptions,
  type ScoresRoll,
} from "@/types/character";

/** The creation route's steps (ADR 0034 step 3), in order; Créer is on the review. */
export const STEPS = [
  "class",
  "race",
  "scores",
  "extras",
  "identity",
  "review",
] as const;
export type StepId = (typeof STEPS)[number];

/**
 * The step a server refusal's field belongs to (the form's field names: `raceCode`, `skillPoints.x`,
 * `details.x`), so the flow opens that step and shows the refusal there. An unknown field stays on
 * the review, where Créer is.
 */
export function stepOfField(field: string): StepId {
  if (field === "class" || field === "xp") return "class";
  if (field === "raceCode" || field === "race") return "race";
  if (
    (ABILITIES as readonly string[]).includes(field) ||
    field === "placed" ||
    field.startsWith("placed.")
  )
    return "scores";
  if (
    field === "hpMax" ||
    field === "strExceptional" ||
    field === "skillArmor" ||
    field === "skillPoints" ||
    field.startsWith("skillPoints.")
  )
    return "extras";
  if (field === "name" || field.startsWith("details.")) return "identity";
  return "review";
}

/** The class's options entry, or undefined. */
export function classOf(options: CreationOptions | null, cls: string) {
  return options?.classes.find((c) => c.class === cls);
}

/**
 * ADR 0034 step 4: the XP a picked level stands for, the class's lowest for that level, read from the
 * server's table; undefined when the class or the table is missing (an older server: XP is typed).
 */
export function xpOfLevel(
  options: CreationOptions | null,
  cls: string,
  level: number,
): number | undefined {
  return classOf(options, cls)?.levelXp?.find((r) => r.level === level)?.xp;
}

/**
 * How a class's alignment limit reads on its card: all of them, all but a few (at most two), or the
 * list it admits. A wording of the server's list, never a rule of its own.
 */
export function alignmentLimit(
  admitted: string[],
  all: readonly string[],
):
  | { kind: "all" }
  | { kind: "except"; codes: string[] }
  | { kind: "only"; codes: string[] } {
  const missing = all.filter((a) => !admitted.includes(a));
  if (missing.length === 0) return { kind: "all" };
  if (missing.length <= 2) return { kind: "except", codes: missing };
  return { kind: "only", codes: all.filter((a) => admitted.includes(a)) };
}

/** Which totals of a roll sit on each ability (indexes into the roll's totals). */
export type Placement = Record<AbilityCode, number[]>;

export function emptyPlacement(): Placement {
  return { str: [], dex: [], con: [], int: [], wis: [], cha: [] };
}

/**
 * Places total `index` of the roll on `ability` (ADR 0032 step 6: the arrangement is the player's
 * choice, made in the web; the scores it gives are the server's, `arrangeScores`), by the method's
 * `assign` mode; null when the method does not allow it:
 * - `keep_per_ability` (II): a total goes only to its own ability and replaces the one kept there;
 * - `arrange` (III to V): one total per ability, each total used once (moved if already placed);
 * - `add_to_base` (VI): each die is added whole to one ability (moved if already placed); the same
 *   die again takes it back off. The server refuses a sum above the method's max.
 * Method I places nothing: its totals are on their abilities (`inOrderPlacement`).
 */
export function place(
  roll: ScoresRoll,
  placement: Placement,
  index: number,
  ability: AbilityCode,
): Placement | null {
  const total = roll.rolls[index];
  if (!total) return null;
  const without = (p: Placement): Placement => {
    const out = emptyPlacement();
    for (const a of ABILITIES) out[a] = p[a].filter((i) => i !== index);
    return out;
  };
  switch (roll.method.assign) {
    case "keep_per_ability":
      if (total.ability !== ability) return null;
      return { ...placement, [ability]: [index] };
    case "arrange":
      return { ...without(placement), [ability]: [index] };
    case "add_to_base": {
      if (placement[ability].includes(index)) return without(placement);
      const next = without(placement);
      return { ...next, [ability]: [...next[ability], index] };
    }
    default:
      return null;
  }
}

/** A fresh roll's placement: method I's totals on the abilities the server gave them, else none. */
export function initialPlacement(roll: ScoresRoll): Placement {
  const out = emptyPlacement();
  if (roll.method.assign !== "in_order") return out;
  roll.rolls.forEach((r, i) => {
    if ((ABILITIES as readonly string[]).includes(r.ability))
      out[r.ability as AbilityCode].push(i);
  });
  return out;
}

/** True when total `index` already sits on an ability. */
export function isPlaced(placement: Placement, index: number): boolean {
  return ABILITIES.some((a) => placement[a].includes(index));
}

/** What the arrangement request carries: by ability, the rolled values placed there (no sum). */
export function placedValues(
  roll: ScoresRoll,
  placement: Placement,
): Record<AbilityCode, number[]> {
  const out = emptyPlacement();
  for (const a of ABILITIES)
    out[a] = placement[a].flatMap((i) => {
      const r = roll.rolls[i];
      return r ? [r.total] : [];
    });
  return out;
}

/** The entered values a step needs before Suivant; the server checks the ranges and the rules. */
export interface StepInputs {
  cls: string;
  race: string;
  xp: string;
  scores: Record<AbilityCode, string>;
  hpMax: string;
  name: string;
  alignment: string;
  sex: string;
  /** The alignments the class admits, from the server's options. */
  alignments: string[];
}

/** A number typed in a field: a whole number, not blank. */
function filled(v: string): boolean {
  const n = parseWhole(v);
  return n !== null && !Number.isNaN(n);
}

/**
 * Why Suivant is blocked on a step (an i18n key under `creation.blocked`), or null. Only missing or
 * unreadable input blocks; a value out of range or against a rule is the server's refusal, shown on
 * its step after Créer, and warnings never block (ADR 0034 step 6).
 */
export function blockedReason(step: StepId, d: StepInputs): string | null {
  switch (step) {
    case "class":
      if (d.cls === "") return "class";
      return filled(d.xp) ? null : "xp";
    case "race":
      return d.race === "" ? "race" : null;
    case "scores":
      return ABILITIES.every((a) => filled(d.scores[a])) ? null : "scores";
    case "extras":
      return filled(d.hpMax) ? null : "hpMax";
    case "identity":
      if (d.name.trim() === "") return "name";
      if (d.alignment === "") return "alignment";
      // The class's limit is a refusal (ADR 0034 step 6): the server's list, not a rule of the web.
      if (!d.alignments.includes(d.alignment)) return "alignmentAllowed";
      return d.sex === "" ? "sex" : null;
    default:
      return null;
  }
}

/** The six races in the server's order (Table 8's list, human first), else the classes' union. */
export function allRaces(options: CreationOptions): string[] {
  const listed = options.raceAdjustments?.map((r) => r.race);
  if (listed && listed.length > 0) return listed;
  const out: string[] = [];
  for (const c of options.classes)
    for (const r of c.races) if (!out.includes(r)) out.push(r);
  return out;
}

/** A signed whole number with a true minus sign: +1, −1, +0. */
export function signed(n: number): string {
  return n < 0 ? `−${-n}` : `+${n}`;
}
