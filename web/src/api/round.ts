import type { RoundAdvanceResponse } from "../types/buffs";

export async function fetchRound(): Promise<{ roundNumber: number }> {
  const res = await fetch("/api/round");
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function advanceRound(): Promise<RoundAdvanceResponse> {
  const res = await fetch("/api/round/advance", {
    method: "POST",
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}
