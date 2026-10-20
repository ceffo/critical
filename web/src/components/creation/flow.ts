import { toBody, type Draft } from "@/lib/characterDetails";
import type { Placement } from "@/lib/creationFlow";
import { classOf, emptyPlacement, xpOfLevel } from "@/lib/creationFlow";
import { parseWhole } from "@/lib/wholeNumber";
import type { PickInput } from "@/components/proficiencies/catalogue";
import {
  ABILITIES,
  type AbilityCode,
  type CharacterPreview,
  type DraftChecks,
  type DraftChecksRequest,
  type CreationOptions,
  type HpRoll,
  type NewCharacter,
  type ScoresArrangement,
  type ScoresRoll,
} from "@/types/character";

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
 * non-weapon picks: each was spent at the old class's cost (ADR 0037 step 4), so they would be
 * refused or mispriced under the new one. */
export function mergeDraft(d: FlowDraft, patch: DraftPatch): FlowDraft {
  const next = { ...d, ...patch, details: { ...d.details, ...patch.details } };
  if (next.cls !== d.cls && patch.picks === undefined) next.picks = [];
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
 * The creation body of a draft (the preview's and Créer's), or the field a value is unreadable in
 * (not a number where one is needed). Only the shape is checked; every range and rule is the server's.
 * A blank alignment or sex is left out (the server says it is missing).
 */
export function buildBody(
  options: CreationOptions,
  draft: FlowDraft,
  xp: string,
  language: string,
): { body: NewCharacter } | { error: FieldError } {
  const fail = (field: string, code: string) => ({
    error: { field, code },
  });
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
    if (v !== null && Number.isNaN(v)) return fail(a, "out_of_range");
    body[a] = v;
  }
  for (const [f, raw] of [
    ["hpMax", draft.hpMax],
    ["xp", xp],
  ] as const) {
    const v = parseWhole(raw);
    if (v !== null && Number.isNaN(v)) return fail(f, "out_of_range");
    body[f] = v;
  }
  const cls = classOf(options, draft.cls);
  if (
    cls?.exceptionalStrengthRaces.includes(draft.race) &&
    body.str === 18 &&
    draft.strExceptional.trim() !== ""
  ) {
    const v = parseWhole(draft.strExceptional);
    if (v === null || Number.isNaN(v))
      return fail("strExceptional", "out_of_range");
    body.strExceptional = v;
  }
  if (cls && cls.skills.length > 0) {
    body.skillPoints = {};
    for (const skill of cls.skills) {
      const raw = draft.skillPoints[skill] ?? "";
      if (raw.trim() === "") continue;
      const v = parseWhole(raw);
      if (v === null || Number.isNaN(v))
        return fail(`skillPoints.${skill}`, "out_of_range");
      body.skillPoints[skill] = v;
    }
  }
  if (draft.picks.length > 0) body.proficiencies = { nonweapon: draft.picks };
  const {
    body: details,
    ageInvalid,
    invalid,
  } = toBody(draft.details, language);
  if (ageInvalid) return fail("details.age", "out_of_range");
  if (invalid.length > 0) return fail(`details.${invalid[0]}`, "not_a_number");
  body.details = details;
  return { body };
}

/**
 * What the server is asked about the draft on the class, skills and proficiency steps (ADR 0034,
 * ADR 0037 step 14): the scores and skill points that read as whole numbers, the XP when it does,
 * and the picks. Only the shape is read; the
 * verdicts (a minimum missed, the points left) are the server's.
 */
export function checksRequest(
  options: CreationOptions,
  draft: FlowDraft,
  xp: string,
): DraftChecksRequest {
  const scores: Record<string, number> = {};
  for (const a of ABILITIES) {
    const v = parseWhole(draft.scores[a]);
    if (v !== null && !Number.isNaN(v)) scores[a] = v;
  }
  const skillPoints: Record<string, number> = {};
  for (const skill of classOf(options, draft.cls)?.skills ?? []) {
    const v = parseWhole(draft.skillPoints[skill] ?? "");
    if (v !== null && !Number.isNaN(v)) skillPoints[skill] = v;
  }
  const x = parseWhole(xp);
  return {
    class: draft.cls,
    scores,
    xp: x === null || Number.isNaN(x) ? null : x,
    skillPoints,
    proficiencies: { nonweapon: draft.picks },
  };
}

/** A refusal shown on its step: the form's field, the server's code, its message as a fallback. */
export interface FieldError {
  field: string;
  code: string;
  message?: string;
}

/**
 * The server's preview of the draft (`POST /api/characters/preview`): its derived figures and
 * warnings, or its refusal; both null before the first answer or when the draft cannot be sent yet.
 */
export interface PreviewState {
  data: CharacterPreview | null;
  error: FieldError | null;
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
  /** The server's preview of the draft (the identity step's warnings, the review's figures). */
  preview: PreviewState;
  /** The server's checks of the draft on the class and skills steps; null before the first answer. */
  checks: DraftChecks | null;
  /** The body Créer would send, null while a value is unreadable. */
  body: NewCharacter | null;
  /** A refusal in words. */
  wording: (e: FieldError) => string;
  /** The UI language (number formatting). */
  language: string;
}
