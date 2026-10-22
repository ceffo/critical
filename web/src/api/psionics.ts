import type { KnownPowers, PowerPick } from "@/types/character";
import { put } from "./http";

/** Saves the whole list of known powers (ADR 0043 step 9), one move the journal's Undo takes back.
 * Answers the known powers as saved; a list over Table 4 or breaking a rule is a 409 naming it. */
export function setKnownPicks(picks: PowerPick[]): Promise<KnownPowers> {
  return put<KnownPowers>("/api/psionics/picks", { picks });
}
