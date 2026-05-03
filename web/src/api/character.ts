import type { Character } from '@/types/character';

export async function fetchCharacter(): Promise<Character> {
  const res = await fetch('/api/character');
  if (!res.ok) throw new Error(await res.text());
  return res.json() as Promise<Character>;
}
