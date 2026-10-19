import { describe, expect, it } from "vitest";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import ts from "typescript";
import { resources } from "@/i18n";

/*
 * The shared pieces (ADR 0026 step 5, ADR 0027 step 3) hold only if no component brings back its own
 * variant: these tests read every component's source.
 */

function tsxFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((f) => {
    const p = join(dir, f);
    if (statSync(p).isDirectory()) return f === "ui" ? [] : tsxFiles(p);
    return p.endsWith(".tsx") ? [p] : [];
  });
}

const root = join(__dirname, "..");
const files = [join(root, "App.tsx"), ...tsxFiles(join(root, "components"))];
const rel = (file: string) => file.slice(root.length + 1);

/** Class patterns each replaced by one shared piece, with the piece to use instead. */
const RETIRED: [RegExp, string][] = [
  [/\bbg-neu-\d+/, "bg-raised (hover: bg-raised-hover)"],
  [/\bbg-amb-(800|900)\b/, "selected, chip-chosen or bg-selected"],
  [/\bbg-(muted|secondary|accent)\b/, "bg-raised"],
  [/\bbg-background\b/, "fields sit on the card: fieldClass / bg-surface"],
  [/\bring-amb-\d+/, "chip-chosen"],
  [/\bborder-amb-\d+/, "latest, or border-tint"],
  [/\bdisabled:opacity-/, "the standard disabled look of ui/button"],
  [/\bvariant="(outline|secondary|default)"/, 'variant="main" or quiet'],
];

describe("one of each shared piece", () => {
  for (const file of files) {
    if (file.endsWith("App.tsx")) continue; // the page itself is the ground
    it(rel(file), () => {
      const src = readFileSync(file, "utf8");
      const found = RETIRED.flatMap(([re, use]) => {
        const m = src.match(re);
        return m ? [`${m[0]} -> ${use}`] : [];
      });
      expect(found).toEqual([]);
    });
  }

  it("headings use the section label, not text-title (step 5.6)", () => {
    const offenders = files.flatMap((file) =>
      [...readFileSync(file, "utf8").matchAll(/<h[1-6][^>]*text-title/g)].map(
        (m) => `${rel(file)}: ${m[0]}`,
      ),
    );
    expect(offenders).toEqual([]);
  });
});

function lookup(lang: "en" | "fr", key: string): string | undefined {
  let v: unknown = resources[lang].common;
  for (const part of key.split(".")) {
    if (typeof v !== "object" || v === null) return undefined;
    v = (v as Record<string, unknown>)[part];
  }
  return typeof v === "string" ? v : undefined;
}

/** Short fixed terms ADR 0027 step 3.2 allows where no single word names the action. */
const FIXED_TERMS = new Set([
  "round.next",
  "spellSlots.longRest",
  // "Log in" / "Log out" (the user's caption for the cog menu's entry): no single word names them.
  "login.action",
  "settings.logout",
]);

/** The i18n keys a button shows as its caption: `t("…")` calls among its children. */
function captionKeys(el: ts.JsxElement): string[] {
  const keys: string[] = [];
  const visit = (n: ts.Node) => {
    if (ts.isJsxAttributes(n)) return; // aria-label, title: the detail goes there
    if (
      ts.isCallExpression(n) &&
      ts.isIdentifier(n.expression) &&
      n.expression.text === "t"
    ) {
      for (const arg of n.arguments)
        if (ts.isStringLiteral(arg)) keys.push(arg.text);
    }
    ts.forEachChild(n, visit);
  };
  el.children.forEach(visit);
  return keys;
}

describe("one action word per button caption (ADR 0027 step 3)", () => {
  for (const file of files) {
    it(rel(file), () => {
      const sf = ts.createSourceFile(
        file,
        readFileSync(file, "utf8"),
        ts.ScriptTarget.Latest,
        true,
        ts.ScriptKind.TSX,
      );
      const offenders: string[] = [];
      const visit = (n: ts.Node) => {
        if (ts.isJsxElement(n)) {
          const open = n.openingElement;
          const tag = open.tagName.getText();
          // A button with a role is a choice (radio, tab, option) or a menu entry, not an action.
          const hasRole = open.attributes.properties.some(
            (a) => ts.isJsxAttribute(a) && a.name.getText() === "role",
          );
          if ((tag === "Button" || tag === "button") && !hasRole) {
            for (const key of captionKeys(n)) {
              if (FIXED_TERMS.has(key)) continue;
              for (const lang of ["en", "fr"] as const) {
                const text = lookup(lang, key);
                if (text === undefined) continue;
                const words = text
                  .replace(/\{\{[^}]*\}\}/g, " ")
                  .split(/\s+/)
                  .filter((w) => /\p{L}/u.test(w));
                if (words.length > 1)
                  offenders.push(`${key} (${lang}): "${text}"`);
              }
            }
          }
        }
        ts.forEachChild(n, visit);
      };
      visit(sf);
      expect(offenders).toEqual([]);
    });
  }
});
