// Checks the French metric conversion (ADR 0021 step 8) against the French books, when the
// private rulebooks clone is present (`just rulebooks`); skipped otherwise. Each spell's French
// range and area of effect (the books' front matter) is paired by id with the catalogue's English
// text; where the French line gives metric lengths, its numbers must be the English numbers as
// `shownUnit` converts them, in order. The French PHB keeps feet and paces, so the pairs are
// nearly all the Tome of Magic's. A pair whose numbers cannot be read one for one (a number in
// words, a dimension written once, dice) is not compared; the lines that disagree are listed
// below with what the book does there, and nothing else may disagree.
import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";
import { beforeAll, describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import { shownUnit, statLocale, type StatLocale } from "./statText";

const root = join(homedir(), "code", "critical-rulebooks");
const catalogue = join(root, "dist", "catalogue.json");
const present = existsSync(catalogue);

/** Book lines whose metric numbers are not the convention's: `<id> <field>` and what they say. */
const SLIPS: Record<string, string> = {
  // The translator read yards as feet.
  "fire_purge area_of_effect": "10-yard square as 3 m",
  "group_mind area_of_effect": "30-yard diameter as 9 m",
  // A number that is not the English one.
  "greater_malison area_of_effect": "30-foot radius as 18 m",
  // The books' two square-foot lines disagree with each other (0.1 and 0.028 m² a square foot);
  // the convention keeps the square of 0.3 m.
  "solipsism area_of_effect": "100 sq ft as 10 m²",
  "estate_transference area_of_effect": "1,000 sq ft/level as 28 m²/level",
  // A cube of feet read as a volume, a cubic foot as a side.
  "metamorphose_liquids area_of_effect": "1-foot cube/level as 27 litres/level",
  "age_object area_of_effect": "1 cubic foot/level as a 30 cm side/level",
};

function mdFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((e) => {
    const p = join(dir, e);
    return statSync(p).isDirectory()
      ? mdFiles(p)
      : p.endsWith(".md")
        ? [p]
        : [];
  });
}

type Length = [number, string];

const NUM = String.raw`(\d{1,3}(?:,\d{3})+|\d+(?:\.\d+)?)`;
const EN_UNIT: [RegExp, string][] = [
  [/^(?:square|sq\.?) ?(?:feet|foot|ft\.?)/i, "sq foot"],
  [/^(?:square|sq\.?) ?(?:yards?|yds?\.?)/i, "sq yard"],
  [/^(?:square|sq\.?) ?(?:miles?|mi\.?)/i, "sq mile"],
  [/^(?:cubic|cu\.?) ?(?:feet|foot|ft\.?)/i, "cu foot"],
  [/^(?:feet|foot|ft\.?|')/i, "foot"],
  [/^(?:yards?|yds?\.?)/i, "yard"],
  [/^(?:miles?|mi\.)/i, "mile"],
  [/^(?:inch(?:es)?|in\.)/i, "inch"],
];

/** The English line's lengths in order; "10 x 20 ft." gives both numbers the unit. */
function englishLengths(text: string): Length[] {
  const out: Length[] = [];
  let pending: number[] = [];
  const re = new RegExp(String.raw`${NUM}(?:[- ]?)`, "g");
  for (let m = re.exec(text); m; m = re.exec(text)) {
    const n = Number(m[1].replace(/,/g, ""));
    const rest = text.slice(re.lastIndex);
    const unit = EN_UNIT.find(([u]) => u.test(rest))?.[1];
    if (unit) {
      out.push(...pending.map((p): Length => [p, unit]), [n, unit]);
      pending = [];
    } else if (/^x\s*/i.test(rest)) pending.push(n);
    else pending = [];
  }
  return out;
}

const FR_UNIT: [RegExp, string][] = [
  [/^(?:mètres? carrés|m2|m²)/, "sq m"],
  [/^(?:kilomètres? carrés|km2|km²)/, "sq km"],
  [/^(?:kilomètres?|km)(?![a-zà-ÿ])/, "km"],
  [/^(?:centimètres?|cm)(?![a-zà-ÿ])/, "cm"],
  [/^(?:m[èê]tres?|m)(?![a-wyzà-ÿ])/, "m"],
  [/^litres?/, "litre"],
];

/** The French line's metric lengths in order; "6 m sur 6", "3 m x 3" give the second the unit. */
function frenchLengths(text: string): Length[] {
  const out: Length[] = [];
  const re = /(\d+(?:,\d+)?) ?/g;
  for (let m = re.exec(text); m; m = re.exec(text)) {
    const n = Number(m[1].replace(",", "."));
    const rest = text.slice(re.lastIndex);
    const unit = FR_UNIT.find(([u]) => u.test(rest))?.[1];
    const before = text.slice(0, m.index);
    const prev = out[out.length - 1];
    if (unit) out.push([n, unit]);
    else if (prev && /(?:sur|x) ?$/.test(before)) out.push([n, prev[1]]);
  }
  return out;
}

/** A length as the French UI shows it: number and unit. */
function shown(loc: StatLocale, [n, unit]: Length): Length {
  const [f, u] = shownUnit(loc, unit, n);
  return [Math.round(n * f * 1000) / 1000, u];
}

let fr: StatLocale;
beforeAll(async () => {
  const i = i18next.createInstance();
  await i.init({
    lng: "fr",
    resources,
    defaultNS: "common",
    interpolation: { escapeValue: false },
  });
  fr = statLocale(i.t, "fr");
});

describe.skipIf(!present)("French metric lengths vs the French books", () => {
  it("are the books' numbers for every stat line they convert", () => {
    const spells = (
      JSON.parse(readFileSync(catalogue, "utf8")) as {
        spells: Record<string, string>[];
      }
    ).spells;
    const en = new Map(spells.map((s) => [s.id, s]));
    let reproduced = 0;
    const skipped: string[] = [];
    const differ: Record<string, string> = {};
    for (const dir of ["phb-fr", "tom-fr"]) {
      const base = join(root, dir, "spells");
      if (!existsSync(base)) continue;
      for (const file of mdFiles(base)) {
        const head = readFileSync(file, "utf8").split("---")[1] ?? "";
        const id = /^id: "(.*)"/m.exec(head)?.[1];
        const spell = id ? en.get(id) : undefined;
        if (!spell) continue;
        for (const field of ["range", "area_of_effect"]) {
          const french = new RegExp(`^${field}: "(.*)"`, "m").exec(head)?.[1];
          const english = spell[field];
          if (!french || !english) continue;
          const want = frenchLengths(french);
          if (!want.length) continue; // imperial (the French PHB) or no length
          const key = `${id} ${field}`;
          const got = englishLengths(english);
          if (/\dd\d/.test(english) || got.length !== want.length) {
            skipped.push(key);
            continue;
          }
          const ok = got.every((g, i) => {
            const [n, u] = shown(fr, g);
            return n === want[i][0] && u === want[i][1];
          });
          if (ok) reproduced++;
          else
            differ[key] = `${english} -> ${got.map((g) => shown(fr, g).join(" ")).join(", ")}`;
        }
      }
    }
    const unexpected = Object.entries(differ).filter(([k]) => !(k in SLIPS));
    expect(unexpected).toEqual([]);
    // 2026-10-15: 159 lines reproduced, 7 slips, 4 not comparable.
    expect(reproduced).toBeGreaterThan(150);
    expect(skipped.length).toBeLessThan(10);
  });
});
