import type { SpellPage } from "@/types/spells";
import { request } from "./http";

/** Filters of GET /api/spells; empty/undefined ones are not sent. */
export interface SpellFilters {
  q: string;
  /** "wizard" | "priest"; "" means both. */
  cls: string;
  level: string;
  school: string;
  /** Subset of V, S, M the spell must need. */
  components: string[];
  /** "none" | "neg" | "1/2" | "special" */
  save: string;
  /** "segment" | "round" | "turn" | "hour" | "day" | "week" | "special" */
  castingTime: string;
  /** Empty: the character's level (the server's default). */
  casterLevel: string;
}

export const NO_FILTERS: SpellFilters = {
  q: "",
  cls: "",
  level: "",
  school: "",
  components: [],
  save: "",
  castingTime: "",
  casterLevel: "",
};

/** Rows per lookup page (the server's default). */
export const LOOKUP_PAGE_SIZE = 50;

/** One page (1-based) of GET /api/spells; `total` counts every match. */
export function searchSpells(
  f: SpellFilters,
  page = 1,
  pageSize = LOOKUP_PAGE_SIZE,
): Promise<SpellPage> {
  const url = new URL("/api/spells", window.location.origin);
  const set = (k: string, v: string) => v && url.searchParams.set(k, v);
  set("q", f.q);
  set("class", f.cls);
  set("level", f.level);
  set("school", f.school.trim());
  set("components", f.components.join(","));
  set("save", f.save);
  set("casting_time", f.castingTime);
  set("caster_level", f.casterLevel);
  set("page", String(page));
  set("page_size", String(pageSize));
  return request<SpellPage>(url.toString());
}
