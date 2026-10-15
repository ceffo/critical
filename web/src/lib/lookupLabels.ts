// Display-only French labels of the lookup: classes, Priest spheres and the common scraped
// stat values. The stored data and the API values are never changed; text with no entry
// in the catalogue shows exactly as scraped.
import type { TFunction } from "i18next";
import { frenchStat } from "@/i18n/spellName";
import { statField, statRaw, type StatKey } from "@/lib/spellValues";
import type { Spell } from "@/types/spells";

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
export function translateValue(
  t: TFunction,
  stat: StatKey,
  raw: string,
): string {
  return lookupLabel(t, `values.${stat}`, raw);
}

/**
 * A stat's text in the detail grid: the catalogue's French text in French (ADR 0021), else the
 * English text, its common values labelled (`translateValue`). Per field: a spell may have a
 * French range and an English casting time.
 */
export function statText(
  t: TFunction,
  lang: string,
  spell: Spell,
  key: StatKey,
): string {
  return (
    frenchStat(lang, spell, statField(key)) ??
    translateValue(t, key, statRaw(spell, key))
  );
}

/** The components of the spell's meta line: French in French when the catalogue has them. */
export function componentsText(lang: string, spell: Spell): string {
  return frenchStat(lang, spell, "components") ?? spell.components;
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
