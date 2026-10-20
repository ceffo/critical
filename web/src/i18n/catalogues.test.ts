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

  it("every skill id of seeds/skills.json and seeds/proficiencies.json has an en and a fr name", () => {
    const read = (file: string) =>
      JSON.parse(
        readFileSync(join(__dirname, "..", "..", "..", "seeds", file), "utf8"),
      ) as unknown;
    const skills = read("skills.json") as { bard_skills: { id: string }[] };
    const catalogue = read("proficiencies.json") as {
      nonweapon: { id: string }[];
    };
    const ids = [...catalogue.nonweapon, ...skills.bard_skills].map(
      (s) => s.id,
    );
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
