/** One XP award (GET /api/xp `entries`); bonusAmount and total are the server's (ADR 0015 step 1). */
export interface XpEntry {
  id: number;
  /** Day it was recorded, YYYY-MM-DD. */
  date: string;
  session: string;
  reason: string;
  base: number;
  /** Whether the +10% bonus applies to this entry. */
  bonus: boolean;
  bonusAmount: number;
  total: number;
  /** ADR 0042 step 5.1: the total shared between a multi-class character's classes; absent for one. */
  split?: ClassAmount[];
}

/** One class entry's share of an award. */
export interface ClassAmount {
  class: string;
  amount: number;
}

export type XpLevelStatus = "past" | "current" | "next" | "ahead";

/** One level of PHB Table 25 with the XP that reaches it. */
export interface XpLevel {
  level: number;
  threshold: number;
  status: XpLevelStatus;
}

/** GET /api/xp and every XP write: totals and progress computed on the server (ADR 0015). */
export interface XpLedger {
  opening: number;
  total: number;
  level: number;
  currentFloor: number;
  /** Null at the last level of the table. */
  nextLevel: number | null;
  nextThreshold: number | null;
  toNext: number;
  /** 0..1, for the bar width. */
  progress: number;
  progressPercent: number;
  /** Default of the +10% checkbox (ADR 0015 step 2). */
  bonusEligible: boolean;
  levels: XpLevel[];
  entries: XpEntry[];
}

/** GET /api/xp/preview: what an entry would add. */
export interface XpAmount {
  base: number;
  bonusAmount: number;
  total: number;
  /** ADR 0042 step 5.1: the total shared between the classes; absent for one class. */
  split?: ClassAmount[];
}

/** POST /api/xp body; on PATCH only the fields present change. */
export interface XpEntryBody {
  base?: number;
  bonus?: boolean;
  reason?: string;
  session?: string;
}

/** Payload of xp_add, xp_edit and xp_remove log entries (xpsvc changeRecord): the award as it is
 * now (as it was, for a remove) and, for an edit, the award before it. */
export interface XpChangeRecord {
  entry: XpEntry;
  /** Only in xp_edit. */
  before?: XpEntry;
}
