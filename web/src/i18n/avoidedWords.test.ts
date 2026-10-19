import { describe, expect, it } from "vitest";

// ADR 0036 steps 3 and 4: the words the glossary avoids stay out of the locale texts. Only the
// values are read: the keys are identifiers (`buff`, `round`) that keep their domain names.

const files = import.meta.glob("./locales/**/*.json", {
  query: "?raw",
  import: "default",
  eager: true,
}) as Record<string, string>;

/** Every string value of a parsed JSON tree, with its dotted path. */
function texts(node: unknown, path: string[] = []): Array<[string, string]> {
  if (typeof node === "string") return [[path.join("."), node]];
  if (node && typeof node === "object") {
    return Object.entries(node).flatMap(([k, v]) => texts(v, [...path, k]));
  }
  return [];
}

function load(language: string): Array<[string, string]> {
  const entry = Object.entries(files).find(([f]) =>
    f.startsWith(`./locales/${language}/`),
  );
  if (!entry) throw new Error(`no locale files for ${language}`);
  return texts(JSON.parse(entry[1]));
}

/** `{{buff}}` is a placeholder name, not a word of the text. */
function withoutPlaceholders(text: string): string {
  return text.replace(/\{\{[^}]*\}\}/g, "·");
}

interface Avoided {
  word: string;
  pattern: RegExp;
}

// "Afficher" contains "fiche", hence the word boundaries. A magic "bonus au toucher" is a
// bonus of the weapon, not a buff: only the buff wordings are refused.
const french: Avoided[] = [
  { word: "MD (say MJ)", pattern: /\bMD\b/ },
  { word: "fiche (say feuille de personnage)", pattern: /\bfiches?\b/i },
  { word: "round (say tour)", pattern: /\brounds?\b/i },
  { word: "test de (say jet de)", pattern: /\btests? (de|d’|d'|critique)/i },
  { word: "Ajuster ce test (say jet)", pattern: /\bce test\b/i },
  { word: "buff (say effet)", pattern: /\bbuffs?\b/i },
  {
    word: "bonus for an effect (say effet)",
    pattern: /\bbonus (enregistr|expir|·)|nom du bonus/i,
  },
  { word: "livre de sorts (say grimoire)", pattern: /livres? de sorts/i },
  {
    word: "sort lancé sur moi (say sort reçu)",
    pattern: /sorts? lancés? sur moi/i,
  },
  {
    word: "jet de protection (say jet de sauvegarde)",
    pattern: /jets? de protection/i,
  },
];

const english: Avoided[] = [
  { word: "GM (say DM)", pattern: /\bGM\b|\bGM’s\b|\bGM's\b/ },
  { word: "buff (say effect)", pattern: /\bbuffs?\b/i },
  {
    word: "spell cast on me (say received spell)",
    pattern: /spells? cast on me|cast on me/i,
  },
];

describe.each([
  ["fr", french],
  ["en", english],
])("%s locale avoids the glossary's words", (language, avoided) => {
  const entries = load(language);

  it("is read", () => {
    expect(entries.length).toBeGreaterThan(100);
  });

  it.each(avoided.map((a) => [a.word, a.pattern] as const))(
    "has no %s",
    (_word, pattern) => {
      const found = entries
        .filter(([, text]) => pattern.test(withoutPlaceholders(text)))
        .map(([path, text]) => `${language}.${path}: ${text}`);
      expect(found).toEqual([]);
    },
  );
});
