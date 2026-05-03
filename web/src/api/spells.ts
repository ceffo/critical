import type { SpellSlot } from "../types/spells";

export const fetchSpellSlots = async (): Promise<SpellSlot[]> => {
  const res = await fetch("/api/spells/slots");
  if (!res.ok) throw new Error("Failed to fetch spell slots");
  return res.json();
};

export const castSpell = async (index: number): Promise<SpellSlot[]> => {
  const res = await fetch(`/api/spells/${index}/cast`, { method: "POST" });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.error || "Failed to cast spell");
  }
  return res.json();
};

export const resetSpellSlots = async (): Promise<SpellSlot[]> => {
  const res = await fetch("/api/spells/reset", { method: "POST" });
  if (!res.ok) throw new Error("Failed to reset spell slots");
  return res.json();
};
