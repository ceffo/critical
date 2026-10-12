// Mirrors api/internal/rest/handler/actionloghdl (LogDTO, EntryDTO, SnapshotDTO, CorrectionDTO).

export type LogKind =
  | "hp_change"
  | "cast"
  | "buff_change"
  | "round_advance"
  | "attack"
  | "item_use"
  | "spell_prep";

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
  status: LogStatus;
  payload: unknown;
  snapshots: LogSnapshot[];
  corrections: LogCorrection[];
  createdAt: string;
}

export interface LogResponse {
  entries: LogEntry[];
}
