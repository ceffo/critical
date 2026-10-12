import type { RoundAdvanceResponse, RoundResponse } from "../types/buffs";
import { post, request } from "./http";

export function fetchRound(): Promise<RoundResponse> {
  return request<RoundResponse>("/api/round");
}

export function advanceRound(): Promise<RoundAdvanceResponse> {
  return post<RoundAdvanceResponse>("/api/round/advance");
}
