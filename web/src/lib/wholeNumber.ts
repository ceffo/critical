/**
 * A whole number as typed in a form field: null when blank (the server then names the missing
 * field), NaN when it is not a whole number. Parsing only: the server checks the ranges.
 */
export function parseWhole(raw: string): number | null {
  const v = raw.trim();
  if (v === "") return null;
  return /^-?\d+$/.test(v) ? Number(v) : Number.NaN;
}
