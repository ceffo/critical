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

export function setRound(round: number): Promise<RoundAdvanceResponse> {
  return put<RoundAdvanceResponse>("/api/round", { round });
}
