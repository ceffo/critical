// The per-kind test harness: each kind's test file runs its fixtures (entries as GET /api/log
// serves them, in fixtures/<kind>.json) through the registry, in English and in French. Imported by
// tests only.
import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { I18nextProvider } from "react-i18next";
import i18next, { type i18n } from "i18next";
import { resources } from "@/i18n";
import { englishSpellName } from "@/i18n/spellName";
import { inventoryNamer } from "@/i18n/inventoryName";
import type { LogEntry, LogKind } from "@/types/log";
import { cardContent } from "./card";
import { isRolledEntry, rowValue, shortResult } from "./entry";
import type { RowValue } from "./kindView";
import { KIND_VIEWS } from "./registry";
import type { Lang } from "./testing";

/** A fixture: the entry's fields as the server serves them; the rest is filled in. */
export type Fixture = Partial<LogEntry> & { payload: unknown };

/** What one language shows of an entry. */
export interface Words {
  /** The journal's text. */
  text: string;
  /** The header's last result, "what · verdict" (absent: the kind's label alone). */
  short?: string;
  /** The result card's subtitle (absent: none). */
  subtitle?: string;
  /** Pieces of the result card's body, rendered (absent: not checked). */
  card?: string[];
}

/** What a fixture shows, in both languages, and its row value and Rolls filter flag. */
export interface Want {
  en: Words;
  fr: Words;
  value?: RowValue | null;
  rolled?: boolean;
}

async function i18nFor(lng: Lang): Promise<i18n> {
  const inst = i18next.createInstance();
  await inst.init({
    lng,
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  return inst;
}

function served(kind: LogKind, f: Fixture): LogEntry {
  return {
    id: 7,
    transactionId: 7,
    characterId: 1,
    round: 3,
    kind,
    summary: "",
    hpDelta: 0,
    effectiveHpDelta: 0,
    note: "",
    effectiveSummary: "",
    status: "active",
    effectivePayload: f.payload,
    snapshots: [],
    corrections: [],
    createdAt: "2026-10-15T21:31:00Z",
    ...f,
  };
}

/**
 * Runs every fixture of a kind through its view: the journal's text, the last result's line, the
 * card's subtitle and body in English and French, the row value and the Rolls filter. Every
 * fixture needs its expectation, and every expectation its fixture.
 */
export function describeKind(
  kind: LogKind,
  /** fixtures/<kind>.json: a name per entry. */
  json: Record<string, unknown>,
  wants: Record<string, Want>,
) {
  const fixtures = json as Record<string, Fixture>;
  describe(`${kind} entries`, () => {
    it("every fixture has its expectation", () => {
      expect(Object.keys(wants).sort()).toEqual(Object.keys(fixtures).sort());
    });

    for (const [name, fixture] of Object.entries(fixtures)) {
      const want = wants[name];
      if (!want) continue;
      it(`${name}: en and fr`, async () => {
        const e = served(kind, fixture);
        const view = KIND_VIEWS[kind];
        expect(view.decode(e), "the fixture decodes").not.toBeNull();
        if ("value" in want) expect(rowValue(e)).toEqual(want.value);
        if (want.rolled !== undefined)
          expect(isRolledEntry(e)).toBe(want.rolled);
        for (const lang of ["en", "fr"] as const) {
          const inst = await i18nFor(lang);
          const w = want[lang];
          // No inventory: item and weapon names as recorded, recorded name maps in `lang`.
          const names = inventoryNamer([], [], lang);
          const c = cardContent(
            { kind: "entry", entry: e, latest: true },
            inst.t,
            lang,
            englishSpellName,
            names,
          );
          expect(c.text, lang).toBe(w.text);
          expect(c.subtitle, lang).toBe(w.subtitle ?? "");
          const s = shortResult(e, inst.t, englishSpellName, names);
          const line = s.verdict ? `${s.what} · ${s.verdict.text}` : s.what;
          expect(line, lang).toBe(w.short ?? c.tag);
          if (w.card) {
            expect(view.card, "the kind has a card body").toBeDefined();
            const html = renderToStaticMarkup(
              <I18nextProvider i18n={inst}>
                {view.card?.({
                  payload: c.payload,
                  entry: e,
                  text: c.text,
                  interactive: true,
                })}
              </I18nextProvider>,
            );
            for (const piece of w.card) expect(html, lang).toContain(piece);
          }
        }
      });
    }
  });
}
