/**
 * Parses the XP amount field: a whole number, spaces (thousands separators) ignored. Null when the
 * field is empty or not a whole number; the server checks the range.
 */
export function parseXpAmount(text: string): number | null {
  const s = text.replace(/[\s\u00a0\u202f]/g, "");
  if (!/^\d+$/.test(s)) return null;
  const n = Number(s);
  return Number.isSafeInteger(n) ? n : null;
}
