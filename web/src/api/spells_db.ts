import type { Spell } from '@/types/spells';

export async function searchSpells(q: string, cls: string = ''): Promise<Spell[]> {
  const url = new URL('/api/spells', window.location.origin);
  if (q) url.searchParams.set('q', q);
  if (cls) url.searchParams.set('class', cls);

  const res = await fetch(url.toString());
  if (!res.ok) throw new Error(await res.text());
  return res.json() as Promise<Spell[]>;
}
