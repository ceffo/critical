// Test helpers of the journal registry: a translator per language and entries as GET /api/log
// serves them. Imported by tests only.
import i18next, { type TFunction } from "i18next";
import { resources } from "@/i18n";
import { englishSpellName, type SpellNamer } from "@/i18n/spellName";
import { recordedNames, type InventoryNamer } from "@/i18n/inventoryName";
import type { LogEntry, LogKind } from "@/types/log";
import { cardContent, type CardContent } from "./card";

export type Lang = "en" | "fr";

export async function makeT(lng: Lang | string): Promise<TFunction> {
  const inst = i18next.createInstance();
  await inst.init({
    lng,
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  return inst.t;
}

/** An entry as GET /api/log serves it, with the payload logged. A roll kind's entry passes
 * `{ roll: true }`, as the server marks it (the web keeps no list of the roll kinds). */
export function logged(
  kind: LogKind,
  payload: unknown,
  over: Partial<LogEntry> = {},
): LogEntry {
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
    payload,
    effectivePayload: payload,
    snapshots: [],
    corrections: [],
    createdAt: "2026-10-15T21:31:00Z",
    ...over,
  };
}

/** The result card's content for a journal entry (the latest result). */
export function cardOf(
  e: LogEntry,
  t: TFunction,
  lang = "en",
  spellName: SpellNamer = englishSpellName,
  names: InventoryNamer = recordedNames,
): CardContent {
  return cardContent(
    { kind: "entry", entry: e, latest: true },
    t,
    lang,
    spellName,
    names,
  );
}
