import { toBody, type Draft } from "@/lib/characterDetails";
import { codeMessage, OVER_BUDGET } from "@/api/http";
import { parseWhole } from "@/lib/wholeNumber";
import type { PickInput } from "@/components/proficiencies/catalogue";
import {
  ABILITIES,
  type AbilityCode,
  type CreationDraft,
  type DraftRefusal,
  type WeaponPickInput,
  type CreationOptions,
  type HpRoll,
  type NewCharacter,
  type ScoresArrangement,
  type ScoresRoll,
} from "@/types/character";

/*
 * The creation flow (ADR 0034 steps 3 to 6): its steps and what blocks Suivant, the draft the
 * player fills in, the one request it sends (`buildBody`, the draft's and Créer's) and the one
 * answer it reads (`POST /api/characters/draft`), and the arrangement of a score roll. Rules are the
 * server's: nothing here computes a score, a level or a verdict.
 */

/** The creation route's steps (ADR 0034 step 3), in order; Créer is on the review. */
export const STEPS = [
  "class",
  "race",
  "scores",
  "extras",
  "proficiencies",
  "spellbook",
  "identity",
  "review",
] as const;
export type StepId = (typeof STEPS)[number];

/** True when the class enters a starting spellbook (ADR 0030 step 20.7: a mage or a bard), as the
 * server's options say (the class's `spellbook`). */
export function hasSpellbook(
  options: CreationOptions | null,
  cls: string,
): boolean {
  return Boolean(classOf(options, cls)?.spellbook);
}

/** The steps the class goes through, in order: the spellbook step only for a class with a
 * spellbook (ADR 0030 step 20.7); before a class is chosen, the steps of a class without one. */
export function stepsFor(
  options: CreationOptions | null,
  cls: string,
): StepId[] {
  return STEPS.filter((s) => s !== "spellbook" || hasSpellbook(options, cls));
}

/**
 * The starting spellbook the draft sends (ADR 0030 step 20.7): the spells the player ticked, or,
 * before he touched the list, the ones the server offers pre-ticked for the class (a mage's Read
 * Magic and Detect Magic); empty for a class without a spellbook.
 */
export function draftSpellbook(
  options: CreationOptions | null,
  d: Pick<FlowDraft, "cls" | "spellbook">,
): string[] {
  const book = classOf(options, d.cls)?.spellbook;
  if (!book) return [];
  return d.spellbook ?? book.startingSpells;
}

/**
 * What the spellbook step shows (ADR 0030 step 20.7), from the draft's answer: `wait` before the
 * server's answer for the class (or while the scores do not derive), `none` when the character has
 * no spell level to enter at its starting level (a bard at level 1), else `levels`.
 */
export function spellbookState(
  answer: CreationDraft | null,
): "wait" | "none" | "levels" {
  const book = answer?.spellbook;
  if (!book) return "wait";
  return book.levels.length === 0 ? "none" : "levels";
}

/**
 * The step a refusal's field belongs to, so the flow opens that step and shows the refusal there.
 * The fields are the server's, as the creation and the draft name them (`class`, `race`, the six
 * scores, `skillPoints.x`, `nonweapon.x`, `weapon.x`, `details.x`; `details.languages` is the
 * proficiency step's); `buildBody` names an unreadable value with the same names. An unknown field
 * stays on the review, where Créer is.
 */
export function stepOfField(field: string): StepId {
  if (field === "class" || field === "xp") return "class";
  if (field === "race") return "race";
  if ((ABILITIES as readonly string[]).includes(field)) return "scores";
  if (
    field === "hpMax" ||
    field === "strExceptional" ||
    field === "skillPoints" ||
    field.startsWith("skillPoints.")
  )
    return "extras";
  // The languages are the proficiency step's (ADR 0037 step 18), though the body still carries them
  // under the details.
  if (
    field === "nonweapon" ||
    field.startsWith("nonweapon.") ||
    field === "weapon" ||
    field.startsWith("weapon.") ||
    field === "details.languages"
  )
    return "proficiencies";
  if (field === "spellbook" || field.startsWith("spellbook."))
    return "spellbook";
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

/** The races in the server's order (the catalogue's), else the classes' union. */
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

/**
 * Everything the player entered in the creation route, kept across the steps (back and forward keep
 * the values). Numbers stay as typed; the server checks them all at Créer.
 */
/** A change to the draft: `details` names only the fields that change. */
export type DraftPatch = Partial<Omit<FlowDraft, "details">> & {
  details?: Partial<FlowDraft["details"]>;
};

/** The draft with the patch applied; `details` is merged field by field, so a reply carrying one
 * detail (a rolled height) keeps what was typed since the request left. Another class clears the
 * picks: each non-weapon one was spent at the old class's cost (ADR 0037 step 4), and a weapon
 * type or a specialization may not be the new class's (steps 7 and 9), so they would be
 * refused or mispriced under the new one. The starting spellbook goes back to the new class's
 * pre-ticked spells (ADR 0030 step 20.7). */
export function mergeDraft(d: FlowDraft, patch: DraftPatch): FlowDraft {
  const next = { ...d, ...patch, details: { ...d.details, ...patch.details } };
  if (next.cls !== d.cls && patch.picks === undefined) next.picks = [];
  if (next.cls !== d.cls && patch.weaponPicks === undefined)
    next.weaponPicks = [];
  if (next.cls !== d.cls && patch.spellbook === undefined)
    next.spellbook = null;
  return next;
}

export interface FlowDraft {
  cls: string;
  race: string;
  /** ADR 0034 step 4: a level (sent as the class's lowest XP for it) or the exact XP. */
  levelMode: "level" | "xp";
  level: number;
  xp: string;
  scoreMode: "roll" | "type";
  method: string;
  roll: ScoresRoll | null;
  placement: Placement;
  /** The total picked in the roll, waiting for an ability to be placed on. */
  picked: number | null;
  /** The server's scores for the placement (ADR 0032 step 6), null before any or on an older server. */
  arranged: ScoresArrangement | null;
  scores: Record<AbilityCode, string>;
  hpRoll: HpRoll | null;
  /** What the HP roll was made for (the class, the Constitution and the XP sent). */
  hpRollFor: HpRollInputs | null;
  /** True once a roll was dropped because its class, Constitution or XP changed since. */
  hpOutdated: boolean;
  hpMax: string;
  strExceptional: string;
  skillPoints: Record<string, string>;
  /** The non-weapon picks (ADR 0037 step 14), each at the slots the player gave it. */
  picks: PickInput[];
  /** The weapon picks and the specialization (ADR 0037 steps 7, 9 and 14), each type at its
   * slots. */
  weaponPicks: WeaponPickInput[];
  /** The starting spellbook's spells (ADR 0030 step 20.7) as the player ticked them, in order; null
   * before he touched the list (the class's pre-ticked spells are sent, `draftSpellbook`). */
  spellbook: string[] | null;
  name: string;
  details: Draft;
}

export const EMPTY_DRAFT: FlowDraft = {
  cls: "",
  race: "",
  levelMode: "level",
  level: 1,
  xp: "0",
  scoreMode: "roll",
  method: "",
  roll: null,
  placement: emptyPlacement(),
  picked: null,
  arranged: null,
  scores: { str: "", dex: "", con: "", int: "", wis: "", cha: "" },
  hpRoll: null,
  hpRollFor: null,
  hpOutdated: false,
  hpMax: "",
  strExceptional: "",
  skillPoints: {},
  picks: [],
  weaponPicks: [],
  spellbook: null,
  name: "",
  details: {
    alignment: "",
    age: "",
    sex: "",
    height: "",
    weight: "",
    hair: "",
    eyes: "",
    languages: [],
  },
};

/** The inputs an HP roll is made from (`POST /api/characters/rolls/hp`). */
export interface HpRollInputs {
  cls: string;
  con: number;
  xp: number;
}

/** The XP the request carries: the picked level's (the class's lowest XP for it), or the typed value. */
export function draftXp(
  options: CreationOptions | null,
  d: Pick<FlowDraft, "cls" | "level" | "levelMode" | "xp">,
): string {
  const levelXp = xpOfLevel(options, d.cls, d.level);
  return d.levelMode === "level" && levelXp !== undefined
    ? String(levelXp)
    : d.xp;
}

/**
 * A draft after a change: an HP roll whose class, Constitution or XP no longer match the draft
 * (ADR 0032 step 7: the hit points are rolled for one class, Constitution and XP) is dropped instead of
 * passing as current. Its total goes with it unless the player typed another value in the box.
 */
export function settle(d: FlowDraft, xp: string): FlowDraft {
  const roll = d.hpRoll;
  const f = d.hpRollFor;
  if (!roll || !f) return d;
  if (
    f.cls === d.cls &&
    f.con === parseWhole(d.scores.con) &&
    f.xp === parseWhole(xp)
  )
    return d;
  return {
    ...d,
    hpRoll: null,
    hpRollFor: null,
    hpOutdated: true,
    hpMax: d.hpMax.trim() === String(roll.total) ? "" : d.hpMax,
  };
}

/**
 * The one request the flow sends: the creation body of the draft as far as it is entered, asked at
 * `POST /api/characters/draft` on every change and sent by Créer to `POST /api/characters`. A value
 * left blank or unreadable (not a number where one is needed) is left out (a null number), so the
 * draft's answer lists it missing; `error` names the first unreadable one, which Créer does not
 * send. Only the shape is read: every range and rule is the server's. A blank alignment or sex is
 * left out (the server says it is missing). The 18/xx is sent only when `offersExceptional`, the
 * server's last draft answer for the class (its `exceptionalStrength`).
 */
export function buildBody(
  options: CreationOptions,
  draft: FlowDraft,
  xp: string,
  language: string,
  offersExceptional = false,
): { body: NewCharacter; error: FieldError | null } {
  let error: FieldError | null = null;
  const fail = (field: string, code: string) => {
    error ??= { field, code };
  };
  const body: NewCharacter = {
    name: draft.name,
    class: draft.cls,
    raceCode: draft.race,
    str: null,
    dex: null,
    con: null,
    int: null,
    wis: null,
    cha: null,
    hpMax: null,
    xp: null,
    strExceptional: 0,
  };
  for (const a of ABILITIES) {
    const v = parseWhole(draft.scores[a]);
    if (v !== null && Number.isNaN(v)) fail(a, "out_of_range");
    else body[a] = v;
  }
  for (const [f, raw] of [
    ["hpMax", draft.hpMax],
    ["xp", xp],
  ] as const) {
    const v = parseWhole(raw);
    if (v !== null && Number.isNaN(v)) fail(f, "out_of_range");
    else body[f] = v;
  }
  const cls = classOf(options, draft.cls);
  if (offersExceptional && draft.strExceptional.trim() !== "") {
    const v = parseWhole(draft.strExceptional);
    if (v === null || Number.isNaN(v)) fail("strExceptional", "out_of_range");
    else body.strExceptional = v;
  }
  if (cls && cls.skills.length > 0) {
    body.skillPoints = {};
    for (const skill of cls.skills) {
      const raw = draft.skillPoints[skill] ?? "";
      if (raw.trim() === "") continue;
      const v = parseWhole(raw);
      if (v === null || Number.isNaN(v))
        fail(`skillPoints.${skill}`, "out_of_range");
      else body.skillPoints[skill] = v;
    }
  }
  if (draft.picks.length > 0 || draft.weaponPicks.length > 0)
    body.proficiencies = {
      ...(draft.picks.length > 0 && { nonweapon: draft.picks }),
      ...(draft.weaponPicks.length > 0 && { weapon: draft.weaponPicks }),
    };
  const spells = draftSpellbook(options, draft);
  if (spells.length > 0) body.spellbook = spells;
  const {
    body: details,
    ageInvalid,
    invalid,
  } = toBody(draft.details, language);
  if (ageInvalid) fail("details.age", "out_of_range");
  if (invalid.length > 0) fail(`details.${invalid[0]}`, "not_a_number");
  body.details = details;
  return { body, error };
}

/** A refusal shown on its step: the form's field, the server's code, its message as a fallback. */
export interface FieldError {
  field: string;
  code: string;
  message?: string;
}

/** A refusal of the draft's answer as the flow's field error: an over_budget worded with the server's
 * figures (ADR 0037 step 10), any other code worded by the step. */
export function refusalError(r: DraftRefusal): FieldError {
  if (r.code === OVER_BUDGET)
    return {
      field: r.field,
      code: r.code,
      message:
        codeMessage(r.code, { spent: r.spent ?? 0, limit: r.limit ?? 0 }) ??
        r.code,
    };
  return { field: r.field, code: r.code, message: r.code };
}

/**
 * The server's answer to the draft (`POST /api/characters/draft`), in two readings. `current`
 * answers the draft as it stands, null before the answer or while a newer one is asked (the
 * warnings, the review's figures and the refusals: a verdict on an older draft is not shown).
 * `latest` is the last answer for the draft's class, kept while a newer one is asked so the figures
 * do not blink as the player types (the class step's minimums, the extras step's budget, the
 * proficiency step's pools). `failed` is a request that got no answer (a network error).
 */
export interface DraftAnswer {
  current: CreationDraft | null;
  latest: CreationDraft | null;
  failed: FieldError | null;
}

/** What every step receives. */
export interface StepProps {
  options: CreationOptions;
  draft: FlowDraft;
  /** Applies a change to the latest draft; `details` is merged field by field, so a late reply
   * (a rolled figure) never rebuilds the details from an older copy. */
  update: (patch: DraftPatch) => void;
  error: FieldError | null;
  /** The worded refusal for a field, or null when the error is another field's. */
  errorFor: (field: string) => string | null;
  /** The XP the request carries: the picked level's, or the typed value. */
  xp: string;
  /**
   * Called before a request: returns a check that stays true until the draft next changes, so a
   * reply that lands after a newer choice (or a newer edit) is dropped instead of overwriting it.
   */
  watch: () => () => boolean;
  /** The server's answer to the draft. */
  answer: DraftAnswer;
  /** The draft's refusals as field errors, on the steps that show them (the proficiency and identity
   * steps and the review), else none. */
  refusals: FieldError[];
  /** The body Créer would send, null while a value is unreadable. */
  body: NewCharacter | null;
  /** A refusal in words. */
  wording: (e: FieldError) => string;
  /** The UI language (number formatting). */
  language: string;
}
