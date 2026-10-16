// The per-field coverage report of `just stat-coverage` (ADR 0021 step 8): how the French UI words
// the six stats of every spell of a catalogue, from the typed stats `api/cmd/spellstats` prints.
// Skipped unless STAT_COVERAGE_JSON names that output, so `just validate` never runs it.
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import {
  statLocale,
  statText,
  type StatField,
  type StatSource,
} from "./statText";
import { FALLBACK_LANG } from "@/i18n/localized";
import type { Spell } from "@/types/spells";

const FIELDS: StatField[] = [
  "casting_time",
  "range",
  "duration",
  "area_of_effect",
  "components",
  "saving_throw",
];
/** One spell of `api/cmd/spellstats`: the books' (English) stat texts and their typed form. */
type CoverageSpell = Pick<Spell, "stats"> & Record<StatField, string>;

/** The spell as the API serves it: each stat text a language map in the books' language. */
function asSpell(s: CoverageSpell): Pick<Spell, StatField | "stats"> {
  const text = (body: string) => ({ [FALLBACK_LANG]: body });
  return {
    stats: s.stats,
    casting_time: text(s.casting_time),
    range: text(s.range),
    duration: text(s.duration),
    area_of_effect: text(s.area_of_effect),
    components: text(s.components),
    saving_throw: text(s.saving_throw),
  };
}

const file = process.env.STAT_COVERAGE_JSON;
/** STAT_COVERAGE_LIST=<field>: also print every distinct English text of that field and its wording. */
const list = process.env.STAT_COVERAGE_LIST as StatField | undefined;

describe.skipIf(!file)("stat coverage", () => {
  it("reports, per field, the typed, labelled and English texts of the French UI", async () => {
    const spells = JSON.parse(readFileSync(file!, "utf8")) as CoverageSpell[];
    const i = i18next.createInstance();
    await i.init({
      lng: "fr",
      resources,
      defaultNS: "common",
      interpolation: { escapeValue: false },
    });
    const loc = statLocale(i.t, "fr");
    const lines = [`${spells.length} spells`, "field: typed / label / book"];
    for (const f of FIELDS) {
      const n: Record<StatSource, number> = { typed: 0, label: 0, book: 0 };
      const left = new Map<string, number>();
      for (const s of spells) {
        const { source } = statText(loc, asSpell(s), f);
        n[source]++;
        if (source === "book") left.set(s[f], (left.get(s[f]) ?? 0) + 1);
      }
      const top = [...left]
        .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
        .slice(0, 8)
        .map(([raw, c]) => `${JSON.stringify(raw)} x${c}`)
        .join(", ");
      lines.push(
        `${f}: ${n.typed} / ${n.label} / ${n.book}${top ? `  (book: ${top})` : ""}`,
      );
      expect(n.typed + n.label + n.book).toBe(spells.length);
    }
    if (list && FIELDS.includes(list)) {
      const seen = new Map<string, string>();
      for (const s of spells)
        seen.set(s[list], statText(loc, asSpell(s), list).text);
      for (const [raw, text] of [...seen].sort())
        lines.push(`${JSON.stringify(raw)} -> ${JSON.stringify(text)}`);
    }
    process.stdout.write(`${lines.join("\n")}\n`);
  });
});
