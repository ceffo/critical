// Display-only French labels of the lookup: classes and Priest spheres. The stored data and the API values are never changed; text with no entry
// in the catalogue shows exactly as scraped.
import type { TFunction } from "i18next";

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

/** One school name: the whole text, else its "/" halves, else its words; unknown stays as scraped. */
function schoolPart(t: TFunction, raw: string): string {
  const known = (r: string) =>
    t(`lookup.schoolNames.${labelKey(r)}`, { defaultValue: "" });
  const whole = known(raw);
  if (whole) return whole;
  const halves = raw.split("/").map((p) => known(p.trim()));
  if (halves.length > 1 && halves.every(Boolean)) return halves.join("/");
  const words = raw.split(/\s+/);
  if (words.length > 1) return words.map((w) => schoolPart(t, w)).join(" ");
  return raw;
}

/** A spell's school field: comma-separated names ("Alteration, Elemental Water"), each translated on its own. */
export function schoolLabel(t: TFunction, raw: string): string {
  return raw
    .split(",")
    .map((p) => p.trim().replace(/^\(/, ""))
    .filter(Boolean)
    .map((p) => schoolPart(t, p))
    .join(", ");
}
