import type { SpellFilterOptions, SpellName, SpellPage } from "@/types/spells";
import { request } from "./http";

/** Class toggle of the lookup: "mine" is the Bard's spellbook. */
export type SpellClass = "mine" | "wizard" | "priest" | "all";

/** A spell search (GET /api/spells); empty fields do not filter. */
export interface SpellQuery {
  /** Name text, matched by the server against the English and French names. */
  q: string;
  cls: SpellClass;
  /** Spell levels to keep; empty means any. */
  levels: number[];
  /** School group key (GET /api/spells/filters). */
  school: string;
  /** Priest sphere name (GET /api/spells/filters). */
  sphere: string;
}

export const NO_QUERY: SpellQuery = {
  q: "",
  cls: "all",
  levels: [],
  school: "",
  sphere: "",
};

/** The API's largest page. */
export const MAX_PAGE_SIZE = 200;

/**
 * One page (1-based) of the ranked search; `total` counts every match. The server matches the
 * query against the English and French names (case and accents ignored).
 */
export function searchSpells(
  query: SpellQuery,
  page = 1,
  pageSize = MAX_PAGE_SIZE,
  /** The caster level the values, damage and modes are served at (omitted: the character's). */
  casterLevel?: number,
): Promise<SpellPage> {
  const url = new URL("/api/spells", window.location.origin);
  const set = (k: string, v: string) => v && url.searchParams.set(k, v);
  set("q", query.q.trim());
  set("class", query.cls);
  set("level", query.levels.join(","));
  set("school", query.school);
  set("sphere", query.sphere);
  set("page", String(page));
  set("page_size", String(pageSize));
  if (casterLevel !== undefined) set("caster_level", String(casterLevel));
  return request<SpellPage>(url.toString());
}

/** Choices of the school / sphere dropdown. */
export function getSpellFilterOptions(): Promise<SpellFilterOptions> {
  return request<SpellFilterOptions>("/api/spells/filters");
}

/** Every catalogue spell's id and names. */
export function getSpellNames(): Promise<SpellName[]> {
  return request<SpellName[]>("/api/spells/names");
}
