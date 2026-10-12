import type { HpState } from "@/types/hp";
import { post, request } from "./http";

export function getHpState(): Promise<HpState> {
  return request<HpState>("/api/hp");
}

/** Append an HP change (negative = damage) to the action log. */
export function appendHpEvent(delta: number, source: string): Promise<HpState> {
  return post<HpState>("/api/hp/event", { delta, source });
}

/** Void the newest non-voided HP change. */
export function undoLastEvent(): Promise<HpState> {
  return post<HpState>("/api/hp/undo");
}
