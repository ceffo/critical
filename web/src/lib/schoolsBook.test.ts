// Checks the French school names against the French books' front matter, when the private
// rulebooks clone is present (`just rulebooks`); skipped otherwise. Each spell's French
// `school` list is paired with the catalogue's English `school` by id (same number of
// schools only); the most frequent French spelling of every English name must be the name
// the French catalogue holds, so one-off OCR slips in the books are outvoted.
import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import fr from "@/i18n/locales/fr/common.json";
import { labelKey } from "./lookupLabels";

const root = join(homedir(), "code", "critical-rulebooks");
const catalogue = join(root, "dist", "catalogue.json");
const present = existsSync(catalogue);

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

function englishSchools(node: unknown, out: Map<string, string>) {
  if (Array.isArray(node)) node.forEach((n) => englishSchools(n, out));
  else if (node && typeof node === "object") {
    const o = node as Record<string, unknown>;
    if (typeof o.id === "string" && typeof o.school === "string")
      out.set(o.id, o.school);
    Object.values(o).forEach((n) => englishSchools(n, out));
  }
}

describe.skipIf(!present)("French school names vs the French books", () => {
  it("hold the majority book spelling of every English school", () => {
    const en = new Map<string, string>();
    englishSchools(JSON.parse(readFileSync(catalogue, "utf8")), en);
    const votes = new Map<string, Map<string, number>>();
    let paired = 0;
    for (const dir of ["phb-fr", "tom-fr"]) {
      const spells = join(root, dir, "spells");
      if (!existsSync(spells)) continue;
      for (const file of mdFiles(spells)) {
        const head = readFileSync(file, "utf8").split("---")[1] ?? "";
        const id = /^id: "(.*)"/m.exec(head)?.[1];
        const list = /^school: (\[.*\])/m.exec(head)?.[1];
        const english = id ? en.get(id) : undefined;
        if (!list || !english) continue;
        const frs = JSON.parse(list) as string[];
        const ens = english
          .replace(/^\(/, "")
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean);
        if (frs.length !== ens.length) continue;
        paired++;
        ens.forEach((e, i) => {
          const v = votes.get(e) ?? new Map<string, number>();
          v.set(frs[i], (v.get(frs[i]) ?? 0) + 1);
          votes.set(e, v);
        });
      }
    }
    expect(paired).toBeGreaterThan(500);
    const names = fr.lookup.schoolNames as Record<string, string>;
    for (const [e, v] of votes) {
      const [winner] = [...v.entries()].sort((a, b) => b[1] - a[1])[0];
      const held = names[labelKey(e)];
      // A name no book spells (Tome of Magic's extra schools) has no vote; a name the
      // books spell by halves is held by halves.
      if (held === undefined) continue;
      expect(held, e).toBe(winner);
    }
  });
});
