import type { CritTable } from "./combat";

/** The two racial lift switches (ADR 0041 step 2.2). */
export interface RacialLift {
  multiclassAnyRace: boolean;
  dualclassAnyRace: boolean;
}

/** The buff a successful check of a proficiency puts on (ADR 0041 step 2.4); the UI words the proficiency from its id. */
export interface SuccessEffect {
  proficiencyId: string;
  /** Descending AC: negative betters it. */
  ac: number;
  rounds: number;
}

/** GET /api/ruleset: the default ruleset with its house rules (ADR 0041 step 8). */
export interface Ruleset {
  id: number;
  name: string;
  catalogueVersion: string;
  critTable: CritTable;
  racialLift: RacialLift;
  /** How the racial level limits apply; "warn" is the only mode today. */
  levelLimits: "warn";
  successEffects: SuccessEffect[];
}

/** One line of the ruleset's audit (GET /api/ruleset/changes); before and after are the rule's value as its PUT answers it. */
export interface RulesetChange {
  id: number;
  at: string;
  /** The editing user's id. */
  by: string;
  rule: "crit_table" | "racial_lift" | "success_effects";
  before: unknown;
  after: unknown;
}
