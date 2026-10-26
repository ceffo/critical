import type { LocalizedText } from "@/types/localized";

/** What a power takes a slot of (ADR 0043 step 8). */
export type PowerKind = "science" | "devotion" | "defence_mode" | "attack_mode";

/** A cost in PSPs: a number, or the word the book gives where it prints none. */
export type PowerCost = number | "variable" | "contact";

/** A catalogue power (GET /api/powers): every text is a language map (ADR 0023). */
export interface Power {
  id: string;
  name: LocalizedText;
  discipline: string;
  kind: PowerKind;
  /** Absent where the book gives a formula in the text. */
  score?: { ability: string; modifier: number };
  initial_cost: PowerCost;
  /** The rule table that gives the cost (read it with getCostTable); `initial_cost` keeps its word. */
  initial_cost_table?: string;
  /** What a "contact" cost adds once contact is made. */
  initial_cost_plus?: number;
  maintenance?: { cost: PowerCost; period: number };
  /** Ids of the powers to know first. */
  prerequisites: string[];
  min_level?: number;
  range: LocalizedText;
  preparation_time: LocalizedText;
  area: LocalizedText;
  description: LocalizedText;
  power_score_result?: LocalizedText;
  natural_20_result?: LocalizedText;
}

export interface PowerPage {
  items: Power[];
  total: number;
  page: number;
  page_size: number;
}

/** GET /api/powers/filters: the disciplines in book order and the kinds. */
export interface PowerFilterOptions {
  disciplines: { id: string; name: LocalizedText }[];
  kinds: PowerKind[];
}
