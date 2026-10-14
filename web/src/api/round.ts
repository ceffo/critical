import type { RoundAdvanceResponse, RoundResponse } from "../types/buffs";
import { post, put, request } from "./http";

export function fetchRound(): Promise<RoundResponse> {
  return request<RoundResponse>("/api/round");
}

export function advanceRound(): Promise<RoundAdvanceResponse> {
  return post<RoundAdvanceResponse>("/api/round/advance");
}

export function resetRound(): Promise<RoundAdvanceResponse> {
  return post<RoundAdvanceResponse>("/api/round/reset");
}

/** Fast-forward game time; the server converts hours to rounds (and refuses 0 rounds). */
export function passTime(hours: number): Promise<RoundAdvanceResponse> {
  return post<RoundAdvanceResponse>("/api/round/pass-time", { hours });
}

export function setRound(round: number): Promise<RoundAdvanceResponse> {
  return put<RoundAdvanceResponse>("/api/round", { round });
}
