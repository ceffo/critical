import { describe, expect, it } from "vitest";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import ts from "typescript";
import { resources } from "./index";

function flatten(obj: Record<string, unknown>, prefix = ""): string[] {
  return Object.entries(obj).flatMap(([k, v]) =>
    typeof v === "object" && v !== null
      ? flatten(v as Record<string, unknown>, `${prefix}${k}.`)
      : [`${prefix}${k}`],
  );
}

describe("catalogues", () => {
  it("fr common has exactly the keys of en common", () => {
    expect(flatten(resources.fr.common).sort()).toEqual(
      flatten(resources.en.common).sort(),
    );
  });

  it("every PHB spell id has a fr name, bar the known gaps", () => {
    // Ids the French Manuel des Joueurs gives no name for.
    const knownGaps: string[] = [];
    const phbIds = JSON.parse(
      readFileSync(join(__dirname, "phbSpellIds.json"), "utf8"),
    ) as string[];
    const seed = JSON.parse(
      readFileSync(
        join(__dirname, "..", "..", "..", "seeds", "spells.json"),
        "utf8",
      ),
    ) as { name: string }[];
    const slug = (n: string) =>
      n
        .toLowerCase()
        .replace(/['’*]/g, "")
        .replace(/[^a-z0-9]+/g, "_")
        .replace(/^_+|_+$/g, "");
    const known = new Set(seed.map((s) => slug(s.name)));
    const fr = resources.fr.spells as Record<string, string>;
    expect(phbIds.length).toBeGreaterThan(400);
    for (const id of phbIds) {
      expect(known.has(id), `${id} is not a catalogue spell`).toBe(true);
      if (knownGaps.includes(id)) continue;
      expect(fr[id], `fr name of ${id}`).toBeTruthy();
    }
  });

  it("every skill id of seeds/skills.json has an en and a fr name", () => {
    const seed = JSON.parse(
      readFileSync(
        join(__dirname, "..", "..", "..", "seeds", "skills.json"),
        "utf8",
      ),
    ) as { proficiencies: { id: string }[]; bard_skills: { id: string }[] };
    const ids = [...seed.proficiencies, ...seed.bard_skills].map((s) => s.id);
    expect(ids.length).toBeGreaterThan(0);
    for (const lang of ["en", "fr"] as const) {
      const names = (
        resources[lang].common as { skills: { names: Record<string, string> } }
      ).skills.names;
      for (const id of ids) expect(names[id], `${lang} ${id}`).toBeTruthy();
    }
  });

  it("every damage type of seeds/spell_effects.json has an en and a fr word", () => {
    const seed = JSON.parse(
      readFileSync(
        join(__dirname, "..", "..", "..", "seeds", "spell_effects.json"),
        "utf8",
      ),
    ) as { damage?: { type?: string } }[];
    const types = [
      ...new Set(seed.flatMap((e) => (e.damage?.type ? [e.damage.type] : []))),
    ];
    expect(types.length).toBeGreaterThan(0);
    for (const lang of ["en", "fr"] as const) {
      const words = (
        resources[lang].common as {
          spellDice: { types: Record<string, string> };
        }
      ).spellDice.types;
      for (const type of types)
        expect(words[type], `${lang} ${type}`).toBeTruthy();
    }
  });

  it("every fr spell key exists in en (en is the fallback)", () => {
    const en = new Set(flatten(resources.en.spells));
    for (const k of flatten(resources.fr.spells)) expect(en.has(k)).toBe(true);
  });
});

// Guard against hard-coded user-visible strings in components.
function tsxFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((f) => {
    const p = join(dir, f);
    if (statSync(p).isDirectory()) return f === "ui" ? [] : tsxFiles(p);
    return p.endsWith(".tsx") ? [p] : [];
  });
}

describe("no hard-coded UI literals", () => {
  const root = join(__dirname, "..");
  const files = [join(root, "App.tsx"), ...tsxFiles(join(root, "components"))];
  const attrs = new Set(["placeholder", "title", "aria-label", "alt"]);
  for (const file of files) {
    it(file.slice(root.length + 1), () => {
      const sf = ts.createSourceFile(
        file,
        readFileSync(file, "utf8"),
        ts.ScriptTarget.Latest,
        true,
        ts.ScriptKind.TSX,
      );
      const offenders: string[] = [];
      const visit = (n: ts.Node) => {
        if (ts.isJsxText(n) && /[A-Za-z]{2,}/.test(n.text))
          offenders.push(n.text.trim());
        if (
          ts.isJsxAttribute(n) &&
          attrs.has(n.name.getText()) &&
          n.initializer &&
          ts.isStringLiteral(n.initializer)
        )
          offenders.push(n.getText());
        ts.forEachChild(n, visit);
      };
      visit(sf);
      expect(offenders).toEqual([]);
    });
  }
});
