import type { RollResult } from "@/types/combat";

export async function rollSavingThrow(category: string): Promise<RollResult> {
  const res = await fetch("/api/roll/save", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ category }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json() as Promise<RollResult>;
}
