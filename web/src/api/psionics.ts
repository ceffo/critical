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

/** POST /api/psionics/powers/{id}/raise: raises a known defence mode, no roll (ADR 0044 steps 4 and 5). */
export function raisePower(id: string): Promise<{
  raise: { power: string; defence: "raised" | "standing"; cost: number };
  psp: PspPool;
  logEntryId: number;
  buffId: string;
}> {
  return post(`/api/psionics/powers/${encodeURIComponent(id)}/raise`, {});
}

/** One roll of the DM's, as sent and as the server decides it. */
export interface IncomingRoll {
  roll: number;
  success: boolean;
}

/** One contest of a defend: the DM's roll, the defence's check (none without a defence), the winner. */
export interface ContestResult {
  attack: IncomingRoll;
  check?: { target: number; d20: number; outcome: string; kept?: boolean };
  fromKept?: boolean;
  winner: "attacker" | "defender";
}

/** The answer of a defend (the psionic_defence entry's payload, as far as the form shows it). */
export interface DefendResult {
  attack: string;
  defence?: string;
  defenceKind?: "raised" | "standing";
  score?: number;
  kept?: boolean;
  /** Table 14's cell for the attack against the defence, for the DM's own score. */
  modifier?: number;
  contests: ContestResult[];
  /** The check the round's next contests of this defence take with no die. */
  keptCheck?: { target: number; d20: number; outcome: string; kept?: boolean };
}

/** POST /api/psionics/defend: decides the DM's one or two rolls of an attack mode against the current defence (step 8). */
export function defend(req: {
  attack: string;
  rolls: IncomingRoll[];
  /** Index of the roll whose check the round's next contests keep. */
  keep?: number;
}): Promise<{ defence: DefendResult; logEntryId: number }> {
  return post("/api/psionics/defend", req);
}

/** One check of an attack as the server logged it: the die, the defence's roll if any, the winner. */
export interface AttackCheckResult {
  check: { target: number; d20: number; outcome: string; kept?: boolean };
  defence?: IncomingRoll;
  winner: "attacker" | "defender";
  /** A won contest against an open mind: the mode's effect, for the DM to apply. */
  effect?: boolean;
}

/** The answer of an attack (the psionic_attack entry's payload, as far as the form shows it). */
export interface AttackResult {
  power: string;
  score: number;
  target: string;
  mind: "open" | "closed";
  defence?: string;
  /** Table 14's cell for the mode against the defence. */
  attackVersusDefence?: number;
  checks: AttackCheckResult[];
  cost: number;
  spent: number;
  tangentsBefore: number;
  tangentsAfter: number;
  contact?: boolean;
  /** The opponent whose tangents the new ones replaced. */
  broken?: string;
  warnings?: string[];
}

/** What an attack sends: the opponent, the DM's rolls against its defence, the labelled modifiers. */
export interface AttackRequest {
  target: string;
  mind: "open" | "closed";
  level: number;
  defence?: string;
  rolls?: IncomingRoll[];
  modifiers: PowerModifier[];
}

/** POST /api/psionics/powers/{id}/attack: uses a known attack mode on an opponent (ADR 0044 step 7). */
export function attackPower(
  id: string,
  req: AttackRequest,
): Promise<{ attack: AttackResult; psp: PspPool; logEntryId: number }> {
  return post(`/api/psionics/powers/${encodeURIComponent(id)}/attack`, req);
}
