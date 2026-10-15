// Mirrors api/internal/rest/handler/actionloghdl (LogDTO, EntryDTO, SnapshotDTO, CorrectionDTO).

export type LogKind =
  | "hp_change"
  | "cast"
  | "buff_change"
  | "round_advance"
  | "round_reset"
  | "round_set"
  | "time_passed"
  | "initiative"
  | "attack"
  | "item_use"
  | "item_restock"
  | "item_add"
  | "item_edit"
  | "item_remove"
  | "weapon_add"
  | "weapon_edit"
  | "weapon_remove"
  | "spell_prep"
  | "long_rest"
  | "save"
  | "skill_check"
  | "dice"
  /** A correction after a roll (target AC, save adjustment, GM verdict), its own undoable step. */
  | "correction";

export type LogStatus = "active" | "amended" | "voided";

export interface LogSnapshot {
  key: string;
  before: unknown;
  after: unknown;
}

export interface LogCorrection {
  id: number;
  kind: "amend" | "void";
  /** Set on amends. */
  hpDelta?: number;
  /** Set on amends that changed the source. */
  source?: string;
  /** Set on amends that replaced the payload (an adjusted save). */
  payload?: unknown;
  /** Set on note amends. */
  note?: string;
  /** The correction entry a post-roll correction belongs to (ADR 0010 step 6). */
  stepId?: number;
  /** True once that correction entry was popped: the amend no longer counts. */
  undone?: boolean;
  createdAt: string;
}

export interface LogEntry {
  id: number;
  characterId: number;
  round: number;
  kind: LogKind;
  summary: string;
  /** Original HP delta as recorded. */
  hpDelta: number;
  /** After amends; 0 when voided. */
  effectiveHpDelta: number;
  effectiveSummary: string;
  /** The latest note amend's note; "" when none. */
  note: string;
  status: LogStatus;
  payload: unknown;
  /** The payload of the latest amend in effect when one set it (a corrected save or attack), else
   * `payload`. */
  effectivePayload: unknown;
  snapshots: LogSnapshot[];
  corrections: LogCorrection[];
  createdAt: string;
}

/** Merged by the server into a heal's payload: the heal asked for vs the part
 * logged after the max-HP cap. */
export interface HpBreakdown {
  hpRequested: number;
  hpApplied: number;
}

export interface LogResponse {
  entries: LogEntry[];
  /** The newest non-voided entry, the one POST /api/log/undo pops; null when nothing can be popped.
   * It may be older than every listed entry. */
  topId: number | null;
}
