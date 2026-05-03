import type { HpEvent, HpState } from '@/types/hp';

export async function getHpState(): Promise<HpState> {
  const res = await fetch('/api/hp');
  if (!res.ok) throw new Error(await res.text());
  return res.json() as Promise<HpState>;
}

export async function appendHpEvent(delta: number, source: string): Promise<HpEvent> {
  const res = await fetch('/api/hp/event', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ delta, source }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json() as Promise<HpEvent>;
}
