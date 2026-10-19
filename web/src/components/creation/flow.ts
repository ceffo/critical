import type { Draft } from "@/lib/characterDetails";
import type { Placement } from "@/lib/creationFlow";
import { emptyPlacement, xpOfLevel } from "@/lib/creationFlow";
import { parseWhole } from "@/lib/wholeNumber";
import type {
  AbilityCode,
  CreationOptions,
  HpRoll,
  ScoresArrangement,
  ScoresRoll,
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
 * detail (a rolled height) keeps what was typed since the request left. */
export function mergeDraft(d: FlowDraft, patch: DraftPatch): FlowDraft {
  return { ...d, ...patch, details: { ...d.details, ...patch.details } };
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
  skillArmor: string;
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
  skillArmor: "leather",
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

/** A refusal shown on its step: the form's field, the server's code, its message as a fallback. */
export interface FieldError {
  field: string;
  code: string;
  message?: string;
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
}
