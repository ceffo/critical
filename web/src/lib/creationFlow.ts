import { parseWhole } from "@/lib/wholeNumber";
import {
  ABILITIES,
  type AbilityCode,
  type CreationOptions,
  type ScoresRoll,
} from "@/types/character";

/**
 * The creation route's steps (ADR 0034 step 3), in order. The identity step stands for today's name
 * and details inputs until its own screen lands; the review comes with it.
 */
export const STEPS = ["class", "race", "scores", "extras", "identity"] as const;
export type StepId = (typeof STEPS)[number];

/**
 * The step a server refusal's field belongs to (the form's field names: `raceCode`, `skillPoints.x`,
 * `details.x`), so the flow opens that step and shows the refusal there. An unknown field goes to the
 * last step, where Create is.
 */
export function stepOfField(field: string): StepId {
  if (field === "class" || field === "xp") return "class";
  if (field === "raceCode" || field === "race") return "race";
  if ((ABILITIES as readonly string[]).includes(field)) return "scores";
  if (
    field === "hpMax" ||
    field === "strExceptional" ||
    field === "skillArmor" ||
    field === "skillPoints" ||
    field.startsWith("skillPoints.")
  )
    return "extras";
  return "identity";
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
 * choice, made in the web), by the method's `assign` mode; null when the method does not allow it:
 * - `keep_per_ability` (II): a total goes only to its own ability and replaces the one kept there;
 * - `arrange` (III to V): one total per ability, each total used once (moved if already placed);
 * - `add_to_base` (VI): each die is added whole to one ability (moved if already placed), never
 *   above the method's `max`; the same die again takes it back off.
 * Method I places nothing: its scores are the server's.
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
      const sum =
        roll.method.base +
        next[ability].reduce((s, i) => s + (roll.rolls[i]?.total ?? 0), 0) +
        total.total;
      if (roll.method.max > 0 && sum > roll.method.max) return null;
      return { ...next, [ability]: [...next[ability], index] };
    }
    default:
      return null;
  }
}

/** True when total `index` already sits on an ability. */
export function isPlaced(placement: Placement, index: number): boolean {
  return ABILITIES.some((a) => placement[a].includes(index));
}

/**
 * The roll column of the arrangement table for an ability: the method's base plus the totals placed
 * there (ADR 0032 step 6 gives both); null when nothing is placed and the method has no base.
 */
export function rolledFor(
  roll: ScoresRoll,
  placement: Placement,
  ability: AbilityCode,
): number | null {
  const placed = placement[ability];
  if (placed.length === 0 && roll.method.base === 0) return null;
  return (
    roll.method.base +
    placed.reduce((s, i) => s + (roll.rolls[i]?.total ?? 0), 0)
  );
}

/**
 * The score an arrangement gives an ability: the roll column plus the race's Table 8 adjustment as
 * the server sent it (ADR 0032 step 6: the stored score is the adjusted one); null with nothing
 * placed. Method I's scores are the server's (`roll.scores`).
 */
export function scoreFor(
  roll: ScoresRoll,
  placement: Placement,
  ability: AbilityCode,
): number | null {
  if (roll.scores) return roll.scores[ability] ?? null;
  const rolled = rolledFor(roll, placement, ability);
  if (rolled === null) return null;
  return rolled + (roll.adjustments?.[ability] ?? 0);
}

/** The entered values a step needs before Suivant; the server checks the ranges and the rules. */
export interface StepInputs {
  cls: string;
  race: string;
  xp: string;
  scores: Record<AbilityCode, string>;
  hpMax: string;
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
