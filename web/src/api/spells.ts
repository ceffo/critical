import type { SpellSlot } from '@/types/spells';

export async function getSpellSlots(): Promise<SpellSlot[]> {
  const res = await fetch('/api/spells/slots');
  if (!res.ok) throw new Error(await res.text());
  return res.json() as Promise<SpellSlot[]>;
}

export async function useSpellSlot(id: number): Promise<SpellSlot> {
  const res = await fetch(`/api/spells/slots/${id}/use`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json() as Promise<SpellSlot>;
}
