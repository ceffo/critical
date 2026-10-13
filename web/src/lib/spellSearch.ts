// The one spell name search shared by the Spell Lookup and "Spell cast on me" (Receive).
// The server knows only English names; names in the other catalogues (French) are matched
// here and sent as catalogue keys with their tier. The server ORs them with its English
// match and ranks across both (exact, then prefix, then contains; level then name).
import { LANGUAGES, resources } from "@/i18n";

/** Lowercase without accents, for matching typed text against names. */
export function fold(s: string): string {
  return s
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "") // combining accents
    .toLowerCase();
}

/** Catalogue keys whose non-English name matches the query, by tier (GET /api/spells match_*). */
export interface NameMatches {
  exact: string[];
  prefix: string[];
  contains: string[];
}

/** Matches of `q` against every non-English spell catalogue (accents and case ignored). */
export function nameMatches(q: string): NameMatches {
  const out: NameMatches = { exact: [], prefix: [], contains: [] };
  const needle = fold(q.trim());
  if (!needle) return out;
  const best = new Map<string, keyof NameMatches>();
  const rank = { exact: 0, prefix: 1, contains: 2 } as const;
  for (const lang of LANGUAGES) {
    if (lang === "en") continue;
    const names = resources[lang].spells as Record<string, string>;
    for (const [key, name] of Object.entries(names)) {
      const n = fold(name);
      const tier: keyof NameMatches | null =
        n === needle
          ? "exact"
          : n.startsWith(needle)
            ? "prefix"
            : n.includes(needle)
              ? "contains"
              : null;
      if (!tier) continue;
      const had = best.get(key);
      if (!had || rank[tier] < rank[had]) best.set(key, tier);
    }
  }
  for (const [key, tier] of best) out[tier].push(key);
  return out;
}

/** The base catalogue key of a spell id ("hold_person_priest", "fortify_l4" -> "hold_person", "fortify"). */
export function baseKey(id: string): string {
  return id.replace(/(_priest)?(_l\d+)?$/, "") || id;
}
