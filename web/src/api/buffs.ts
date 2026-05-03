import type { Buff } from '@/types/buffs';

export async function getBuffs(): Promise<Buff[]> {
  const res = await fetch('/api/buffs');
  if (!res.ok) throw new Error(await res.text());
  return res.json() as Promise<Buff[]>;
}

export async function addBuff(name: string, durationType: 'rounds' | 'charges', amount: number): Promise<Buff> {
  const res = await fetch('/api/buffs', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, durationType, amount }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json() as Promise<Buff>;
}
