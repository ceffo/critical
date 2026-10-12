import type { Spell } from "@/types/spells";
import { request } from "./http";

export function searchSpells(q: string, cls: string = ""): Promise<Spell[]> {
  const url = new URL("/api/spells", window.location.origin);
  if (q) url.searchParams.set("q", q);
  if (cls) url.searchParams.set("class", cls);
  return request<Spell[]>(url.toString());
}
