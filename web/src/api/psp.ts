import type { LocalizedText } from "@/types/localized";
import type { PspPool } from "@/types/character";
import { post } from "./http";

/** Append a PSP change (negative = spent) to the action log (ADR 0043 step 5). */
export function appendPspEvent(
  delta: number,
  source: string,
): Promise<PspPool> {
  return post<PspPool>("/api/psp/event", { delta, source });
}

/** Recover PSPs over hours of a Table 6 row (ADR 0043 step 6): the server reads its rate. */
export function recoverPsp(hours: number, activity: string): Promise<PspPool> {
  return post<PspPool>("/api/psp/recover", { hours, activity });
}

/** The answer of `POST /api/psionics/stop-and-recover`: the pool after it and the rows it ended. */
export interface StopRecoverResult {
  psp: PspPool;
  /** Catalogue power ids whose maintained row ended. */
  powersEnded: string[];
  powerNames?: Record<string, LocalizedText>;
  /** Minds whose tangent or contact row ended. */
  targetsEnded: string[];
}

/** Stop every power that pays PSPs and recover, as one undoable entry (ADR 0047 item 4). */
export function stopAndRecoverPsp(
  hours: number,
  activity: string,
): Promise<StopRecoverResult> {
  return post<StopRecoverResult>("/api/psionics/stop-and-recover", {
    hours,
    activity,
  });
}
