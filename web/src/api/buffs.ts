import type { BuffEffect } from "../types/buffs";

export async function fetchActiveBuffs(): Promise<BuffEffect[]> {
  const res = await fetch('/api/buffs');
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function activateBuff(buff: Partial<BuffEffect>): Promise<BuffEffect[]> {
  const res = await fetch('/api/buffs', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(buff),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function dismissBuff(id: string): Promise<BuffEffect[]> {
  const res = await fetch(`/api/buffs/${id}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function triggerBuff(id: string): Promise<BuffEffect[]> {
  const res = await fetch(`/api/buffs/${id}/trigger`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}
