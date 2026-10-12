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
