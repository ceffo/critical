import i18next from "i18next";
import { describe, expect, it } from "vitest";
import { resources } from "@/i18n";

// French text with a count must agree in number. i18next picks `_one` / `_other` by the language's
// plural rules; French counts 0 and 1 as singular ("0 tangente", "1 tangente", "2 tangentes").

type Tree = { [k: string]: string | Tree };

function flatten(tree: Tree, prefix = ""): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [k, v] of Object.entries(tree)) {
    const path = prefix ? `${prefix}.${k}` : k;
    if (typeof v === "string") out[path] = v;
    else Object.assign(out, flatten(v, path));
  }
  return out;
}

const en = flatten(resources.en.common as Tree);
const fr = flatten(resources.fr.common as Tree);
const PLURAL_SUFFIX = /_(zero|one|two|few|many|other)$/;

async function translator(lng: "en" | "fr") {
  const inst = i18next.createInstance();
  await inst.init({
    lng,
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  return inst.t.bind(inst);
}

describe("tangent count chip", () => {
  it.each([
    [0, "0 tangente sur 2"],
    [1, "1 tangente sur 2"],
    [2, "2 tangentes sur 2"],
    [5, "5 tangentes sur 2"],
  ])("reads %i in French", async (count, text) => {
    const fr = await translator("fr");
    expect(fr("powers.tab.tangents", { count })).toBe(text);
  });

  it("keeps the English wording", async () => {
    const en = await translator("en");
    expect(en("powers.tab.tangents", { count: 2 })).toBe("2 of 2 tangents");
    expect(en("powers.tab.tangents", { count: 1 })).toBe("1 of 2 tangents");
  });
});

describe("French count keys", () => {
  it("has the plural forms French needs for every plural key", () => {
    const needed = Array.from(
      new Set(new Intl.PluralRules("fr").resolvedOptions().pluralCategories),
    ).filter((c) => c === "one" || c === "other");
    const bases = new Set(
      [...Object.keys(en), ...Object.keys(fr)]
        // `too_many` or `too_other` can be an error code; a plural form holds the count.
        .filter(
          (k) => PLURAL_SUFFIX.test(k) && /\{\{count\}\}/.test(en[k] ?? fr[k]),
        )
        .map((k) => k.replace(PLURAL_SUFFIX, "")),
    );
    const missing: string[] = [];
    for (const base of bases) {
      for (const form of needed) {
        if (!(`${base}_${form}` in fr)) missing.push(`${base}_${form}`);
      }
    }
    expect(missing).toEqual([]);
  });

  it("words a count followed by a noun or an agreeing participle through plural forms", () => {
    // A count in a French sentence that is followed by a word, or by "(s)", is a count-style
    // text. It needs `_one` / `_other` unless it is reviewed here as invariable.
    const invariable: Record<string, string> = {
      "spellSlots.splitCount": "multiplication: 'N × niveau L'",
      "spellSlots.receiveMore": "'N de plus' does not agree",
      "lookup.stat.areaCount": "the unit comes from catalogue data",
      "apiError.devotions_below_twice_sciences":
        "always at least two devotions here",
      "log.item.restocked": "'maintenant' is an adverb",
      "creation.spellbook.countNoMax": "'(sans maximum)' is about the cap",
    };
    const countThenWord = /\{\{count\}\}\s+(\p{L}{3,}|\(s\))/u;
    const flagged = Object.entries(fr)
      .filter(([k, v]) => !PLURAL_SUFFIX.test(k) && countThenWord.test(v))
      .map(([k]) => k)
      .filter((k) => !(k in invariable));
    expect(flagged).toEqual([]);
  });
});
