import type { PspPool } from "@/types/character";
import { post } from "./http";

/** Table 6's activities (ADR 0043 step 6), the rows a recovery names. */
export const PSP_ACTIVITIES = [
  "exertion",
  "walking",
  "resting",
  "sleeping",
] as const;
export type PspActivity = (typeof PSP_ACTIVITIES)[number];

/** Append a PSP change (negative = spent) to the action log (ADR 0043 step 5). */
export function appendPspEvent(
  delta: number,
  source: string,
): Promise<PspPool> {
  return post<PspPool>("/api/psp/event", { delta, source });
}

/** Recover PSPs over hours of an activity (ADR 0043 step 6): the server reads Table 6's rate. */
export function recoverPsp(
  hours: number,
  activity: PspActivity,
): Promise<PspPool> {
  return post<PspPool>("/api/psp/recover", { hours, activity });
}
