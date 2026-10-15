// State of the Time passes form's preview. Input parsing only: the server answers what the hours
// would do (GET /api/round/pass-time/preview) and is the authority.
import type { PassTimePreview } from "@/types/buffs";
import { parseDecimal } from "./decimalInput";

export type PreviewAnswer =
  { kind: "error"; message: string } | ({ kind: "ok" } & PassTimePreview);

/** The server's answer and the hours it was asked for. */
export interface KeyedAnswer {
  hours: number;
  answer: PreviewAnswer;
}

export type Preview =
  { kind: "empty" } | { kind: "invalid" } | { kind: "pending" } | PreviewAnswer;

/** The hours to ask the server about for `text`, or null when it is not a positive number. */
export function hoursToAsk(text: string): number | null {
  const v = parseDecimal(text);
  return Number.isNaN(v) || v <= 0 ? null : v;
}

/**
 * What the form shows for `text`: nothing when empty, the hint when not a positive number, the
 * server's answer only when it is for these hours (an answer for older input is never shown: it
 * stays pending until the matching one arrives). Confirm is on only for `kind: "ok"`.
 */
export function previewFor(text: string, latest: KeyedAnswer | null): Preview {
  if (text.trim() === "") return { kind: "empty" };
  const hours = hoursToAsk(text);
  if (hours === null) return { kind: "invalid" };
  return latest?.hours === hours ? latest.answer : { kind: "pending" };
}
