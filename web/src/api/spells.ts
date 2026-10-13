import type { PreparationInfo, SpellSlot } from "../types/spells";
import { post, request } from "./http";

export function fetchSpellSlots(): Promise<SpellSlot[]> {
  return request<SpellSlot[]>("/api/spells/slots");
}

export function castSpell(index: number): Promise<SpellSlot[]> {
  return post<SpellSlot[]>(`/api/spells/${index}/cast`);
}

export function resetSpellSlots(): Promise<SpellSlot[]> {
  return post<SpellSlot[]>("/api/spells/reset");
}

export function fetchPreparationInfo(): Promise<PreparationInfo> {
  return request<PreparationInfo>("/api/spells/preparation");
}

/** Replace today's preparation with these spell ids (duplicates allowed). */
export function prepareSpells(spells: string[]): Promise<SpellSlot[]> {
  return request<SpellSlot[]>("/api/spells/prepared", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ spells }),
  });
}
