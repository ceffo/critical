import type { KnownPowers, PowerPick, PspPool } from "@/types/character";
import { post, put } from "./http";

/** Saves the whole list of known powers (ADR 0043 step 9), one move the journal's Undo takes back.
 * Answers the known powers as saved; a list over Table 4 or breaking a rule is a 409 naming it. */
export function setKnownPicks(picks: PowerPick[]): Promise<KnownPowers> {
  return put<KnownPowers>("/api/psionics/picks", { picks });
}

/** A labelled modifier to the power check (ADR 0043 step 10): -20 to +20. */
export interface PowerModifier {
  value: number;
  reason: string;
}

/** What a use of a power sends; a field the power does not ask for is left out. */
export interface UsePowerRequest {
  modifiers: PowerModifier[];
  /** The initial cost of a power whose cost is variable. */
  cost?: number;
  /** The maintenance of a power whose maintenance cost is variable. */
  maintenanceCost?: number;
  /** The score of a power that has none of its own (the book gives it in the text). */
  score?: number;
}

/** The power check as the server logged it (the power_check entry's payload, as far as the panel shows it). */
export interface PowerCheckResult {
  power: string;
  target: number;
  d20: number;
  outcome: "success" | "failure" | "power_score" | "natural_20";
  success: boolean;
  cost: number;
  spent: number;
  maintenanceCost?: number;
  maintenancePeriod?: number;
  /** Warning codes: "second_power_this_round". */
  warnings?: string[];
}

/** POST /api/psionics/powers/{id}/use: checks a known power and spends its initial cost (steps 10-13). */
export function invokePower(
  id: string,
  req: UsePowerRequest,
): Promise<{ check: PowerCheckResult; psp: PspPool; logEntryId: number }> {
  return post(`/api/psionics/powers/${encodeURIComponent(id)}/use`, req);
}

/** POST /api/psionics/powers/{id}/stop: stops a maintained power (step 16), free. */
export function stopPower(id: string): Promise<{ powerId: string }> {
  return post(`/api/psionics/powers/${encodeURIComponent(id)}/stop`, {});
}
