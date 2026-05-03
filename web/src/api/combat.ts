import type { RollResult } from "../types/combat";

export async function resolveAttack(
  weaponId: string,
  targetAc: number,
): Promise<RollResult> {
  const res = await fetch("/api/roll/attack", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ weapon_id: weaponId, target_ac: targetAc }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}
