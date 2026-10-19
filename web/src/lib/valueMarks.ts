// Gameplay values in a text (ADR 0021 step 15). The API recognises them in the descriptions and
// the stat texts it serves, as spans (`Spell.marked`); the client wording the typed stats builds
// the values it composes. Nothing here recognises a value: it only carries marks.
//
// A marked text is a string with each value wrapped in private-use characters, so that it passes
// through the description parser, i18n interpolation and string joins unchanged, and is split into
// spans only where it is rendered.
import type { LocalizedText } from "@/types/localized";
import { localizedPick } from "@/i18n/localized";

/** The kinds of gameplay value; the style is one for all of them (the kind stays for a later style per kind). */
export type ValueKind =
  "dice" | "time" | "level" | "distance" | "percent" | "modifier";

/** A piece of a text; the spans of a text concatenate to it. */
export interface ValueSpan {
  text: string;
  kind?: ValueKind;
}

/** Spans by field name, then by language (the wire's `marked`). */
export type MarkedTexts = Record<string, Record<string, ValueSpan[]>>;

const OPEN = "";
const SEP = "";
const CLOSE = "";
const MARKS = /[-]/g;

/** `text` as a value of `kind`. */
export function mark(kind: ValueKind, text: string): string {
  return `${OPEN}${kind}${SEP}${text.replace(MARKS, "")}${CLOSE}`;
}

/** A text's spans as one marked string. */
export function markedString(spans: ValueSpan[]): string {
  return spans
    .map((s) => (s.kind ? mark(s.kind, s.text) : s.text.replace(MARKS, "")))
    .join("");
}

/** The text without its marks. */
export function plainText(s: string): string {
  return s
    .replace(new RegExp(`${OPEN}[a-z]+${SEP}`, "g"), "")
    .replace(MARKS, "");
}

/** A marked string as spans. */
export function valueSpans(s: string): ValueSpan[] {
  const out: ValueSpan[] = [];
  const re = new RegExp(`${OPEN}([a-z]+)${SEP}([^${CLOSE}]*)${CLOSE}`, "g");
  let at = 0;
  for (const m of s.matchAll(re)) {
    if (m.index > at) out.push({ text: s.slice(at, m.index) });
    out.push({ text: m[2], kind: m[1] as ValueKind });
    at = m.index + m[0].length;
  }
  if (at < s.length) out.push({ text: s.slice(at) });
  return out;
}

/**
 * The field of a spell in the UI language, marked: the API's spans of the language the text was
 * picked in (`localized`'s fallback), or the plain text when the API marked none or the spans do
 * not make that text.
 */
export function markedField(
  lang: string,
  text: LocalizedText | undefined,
  marked: MarkedTexts | undefined,
  field: string,
): string {
  const { lang: picked, body } = localizedPick(lang, text);
  const spans = marked?.[field]?.[picked];
  return spans && spans.map((s) => s.text).join("") === body
    ? markedString(spans)
    : body;
}
