import type { Power, PowerFilterOptions, PowerPage } from "@/types/powers";
import { request } from "./http";

/** A power search (GET /api/powers); empty fields do not filter. */
export interface PowerQuery {
  /** Name text, matched by the server against the name in every language. */
  q: string;
  /** Discipline id (GET /api/powers/filters). */
  discipline: string;
  /** Power kind. */
  kind: string;
}

export const NO_POWER_QUERY: PowerQuery = { q: "", discipline: "", kind: "" };

/** The API's largest page. */
export const MAX_POWER_PAGE_SIZE = 200;

/** One page (1-based) of the ranked search; `total` counts every match. */
export function searchPowers(query: PowerQuery, page = 1): Promise<PowerPage> {
  const url = new URL("/api/powers", window.location.origin);
  const set = (k: string, v: string) => v && url.searchParams.set(k, v);
  set("q", query.q.trim());
  set("discipline", query.discipline);
  set("kind", query.kind);
  set("page", String(page));
  set("page_size", String(MAX_POWER_PAGE_SIZE));
  return request<PowerPage>(url.toString());
}

/** Choices of the discipline and kind filters. */
export function getPowerFilterOptions(): Promise<PowerFilterOptions> {
  return request<PowerFilterOptions>("/api/powers/filters");
}

/** The catalogue power `id`; a 404 when the catalogue has none. */
export function getPower(id: string): Promise<Power> {
  return request<Power>(`/api/powers/${encodeURIComponent(id)}`);
}
