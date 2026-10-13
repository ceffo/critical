// Display-only French labels of the lookup: classes, Priest spheres and the common scraped
// stat values. The stored data and the API values are never changed; text with no entry
// in the catalogue shows exactly as scraped.
import type { TFunction } from "i18next";
import type { StatKey } from "@/lib/spellValues";

/** Catalogue key of a raw text: trimmed, lowercased, every run of non-alphanumerics -> "_". */
export function labelKey(raw: string): string {
  return raw
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_");
}

/** The catalogue entry at `lookup.<group>.<key of raw>`, or `raw` itself when there is none. */
function lookupLabel(t: TFunction, group: string, raw: string): string {
  const key = labelKey(raw);
  if (!key) return raw;
  return t(`lookup.${group}.${key}`, { defaultValue: raw });
}

/** A scraped stat value (exact whole-value match only; "210 yd" etc. are left alone). */
export function translateValue(t: TFunction, stat: StatKey, raw: string): string {
  return lookupLabel(t, `values.${stat}`, raw);
}

/** A Priest sphere name ("Elemental Fire"). */
export function sphereLabel(t: TFunction, name: string): string {
  return lookupLabel(t, "spheres", name);
}

/** A spell's sphere field: comma-separated names, each translated on its own. */
export function spheresLabel(t: TFunction, raw: string): string {
  return raw
    .split(", ")
    .map((s) => sphereLabel(t, s))
    .join(", ");
}

/** "Wizard" / "Priest". */
export function classLabel(t: TFunction, cls: string): string {
  return lookupLabel(t, "classes", cls);
}
