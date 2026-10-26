import type {
  Initiative,
  NextRoundPreview,
  PassTimePreview,
  RoundAdvanceResponse,
  RoundResponse,
} from "../types/buffs";
import { post, put, request } from "./http";

export function fetchRound(): Promise<RoundResponse> {
  return request<RoundResponse>("/api/round");
}

export function advanceRound(): Promise<RoundAdvanceResponse> {
  return post<RoundAdvanceResponse>("/api/round/advance");
}

/** What the next Next Round would charge, row by row and in total (ADR 0043 step 14). */
export function fetchNextRoundPreview(
  init?: RequestInit,
): Promise<NextRoundPreview> {
  return request<NextRoundPreview>("/api/round/advance/preview", init);
}

/** Pop the round's initiative entry and roll again (409 when another entry is newer). */
export function rerollInitiative(): Promise<Initiative> {
  return post<Initiative>("/api/round/initiative/reroll");
}

export function resetRound(): Promise<RoundAdvanceResponse> {
  return post<RoundAdvanceResponse>("/api/round/reset");
}

/** Fast-forward game time; the server converts hours to rounds (and refuses 0 rounds). */
export function passTime(hours: number): Promise<RoundAdvanceResponse> {
  return post<RoundAdvanceResponse>("/api/round/pass-time", { hours });
}

/** What passing `hours` would do (rounds, buffs that would expire); writes nothing. */
export function previewPassTime(
  hours: number,
  init?: RequestInit,
): Promise<PassTimePreview> {
  return request<PassTimePreview>(
    `/api/round/pass-time/preview?hours=${encodeURIComponent(String(hours))}`,
    init,
  );
}

export function setRound(round: number): Promise<RoundAdvanceResponse> {
  return put<RoundAdvanceResponse>("/api/round", { round });
}
