import type { RollResult } from '@/types/combat';

export async function rollAttack(targetAC: number): Promise<RollResult> {
  const res = await fetch('/api/roll/attack', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ targetAC }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json() as Promise<RollResult>;
}
