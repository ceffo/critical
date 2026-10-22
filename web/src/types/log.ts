// Mirrors api/internal/rest/handler/actionloghdl (LogDTO, EntryDTO, SnapshotDTO, CorrectionDTO).

export type LogKind =
  | "hp_change"
  /** A PSP change entered by hand (ADR 0043 step 5), bounded by the pool; popping it gives them back. */
  | "psp_change"
  /** PSPs recovered over hours of an activity (ADR 0043 step 6), Table 6's hourly rate. */
  | "psp_recovery"
  | "cast"
  | "buff_change"
  | "round_advance"
  | "round_reset"
  | "round_set"
  | "time_passed"
  | "initiative"
  /** The round's initiative corrected by a spell cast or an item used (ADR 0035 item 6.2). */
  | "initiative_correct"
  | "attack"
  | "item_use"
  | "item_restock"
  | "item_add"
  | "item_edit"
  | "item_remove"
  /** Written before the one equipment model (ADR 0033 step 26.2): still shown and popped. */
  | "weapon_add"
  | "weapon_edit"
  | "weapon_remove"
  /** A piece equipped or unequipped (ADR 0033 step 7); popping it puts the pieces back. */
  | "equipment_state"
  /** An XP award added, edited or removed (ADR 0015 step 4); popping it gives the XP total back. */
  | "xp_add"
  | "xp_edit"
  | "xp_remove"
  /** A level-up roll (ADR 0030 step 7.3); popping it gives the max HP back, the level pending again. */
  | "level_up"
  /** A new class taken (ADR 0042 step 3); popping it gives the previous class entries back. */
  | "class_switch"
  /** A chance-to-learn roll (ADR 0030 step 20.2); popping it takes the spell out or lifts the block. */
  | "spell_learn"
  /** A spell written in the spellbook with no roll (ADR 0030 step 20.6); popping it takes it out. */
  | "spell_add"
  /** A cleric's turning undead roll (ADR 0030 step 16); it changes no state. */
  | "turn_undead"
  /** A thief's skill points spent on the sheet (ADR 0030 step 17); popping it gives them back. */
  | "skill_points"
  /** The proficiency picks saved (ADR 0037 steps 12 and 13); popping it gives them back. */
  | "proficiencies"
  | "skill_armor"
  /** The AC override set or cleared (ADR 0033 step 13); popping it gives the previous one back. */
  | "ac_override"
  | "spell_prep"
  | "long_rest"
  | "save"
  | "skill_check"
  /** A plain ability check (ADR 0028), adjusted after the roll like a skill check. */
  | "ability_check"
  | "dice"
  /** One round of exposure to a spell's area (ADR 0008 step 16); on the Bard it carries an HP delta. */
  | "exposure"
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
  /** A void pops the entry, a redo un-pops it (ADR 0019 step 1): the newest of the two decides. */
  kind: "amend" | "void" | "redo";
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
  /** The id of the first entry of this entry's transaction (its own id when it is that entry):
   * everything one request recorded. Undo pops the whole transaction (ADR 0018 steps 1 and 2). */
  transactionId: number;
  characterId: number;
  round: number;
  kind: LogKind;
  /** One of the roll kinds (actionlog.RollKinds, ADR 0018 step 10): its result takes the result
   * card, and only the newest of them takes a correction. Absent from an API older than the mark. */
  roll?: boolean;
  summary: string;
  /** Original HP delta as recorded. */
  hpDelta: number;
  /** After amends; 0 when voided. */
  effectiveHpDelta: number;
  /** PSP delta as recorded (ADR 0043 step 5): any entry may carry one, never amended. Absent from an
   * API older than the PSP ledger. */
  pspDelta?: number;
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

/** Merged by the server into the payload of an entry whose PSP delta it cut (ADR 0043 step 5): a gain
 * by the maximum, a loss at 0. */
export interface PspBreakdown {
  pspRequested: number;
  pspApplied: number;
}

export interface LogResponse {
  entries: LogEntry[];
  /** The newest non-voided entry, the one POST /api/log/undo pops; null when nothing can be popped.
   * It may be older than every listed entry. */
  topId: number | null;
  /** The first entry of the transaction POST /api/log/redo replays (ADR 0019 step 2); its id is the
   * `expectedRedoId`. Null when there is nothing to redo. It may be older than every listed entry. */
  redo: LogEntry | null;
}
