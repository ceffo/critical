import type { Draft } from "@/lib/characterDetails";
import type { Placement } from "@/lib/creationFlow";
import { emptyPlacement } from "@/lib/creationFlow";
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
  update: (patch: Partial<FlowDraft>) => void;
  error: FieldError | null;
  /** The worded refusal for a field, or null when the error is another field's. */
  errorFor: (field: string) => string | null;
  /** The XP the request carries: the picked level's, or the typed value. */
  xp: string;
}
