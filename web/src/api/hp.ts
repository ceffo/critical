import type { HpState } from "@/types/hp";

export async function getHpState(): Promise<HpState> {
  const res = await fetch("/api/hp");
  if (!res.ok) throw new Error(await res.text());
  return res.json() as Promise<HpState>;
}

export async function appendHpEvent(
  delta: number,
  source: string,
): Promise<HpState> {
  const res = await fetch("/api/hp/event", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ delta, source }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json() as Promise<HpState>;
}

export async function undoLastEvent(): Promise<HpState> {
  const res = await fetch("/api/hp/undo", {
    method: "POST",
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json() as Promise<HpState>;
}
