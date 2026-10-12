import type { SpellSlot } from "../types/spells";
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
