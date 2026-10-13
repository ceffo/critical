// Input parsing only: the server validates the value (>= 1) and says which row is wrong.

/**
 * A typed decimal: "1.5" and the French "1,5" both work. Anything that is not a plain
 * non-negative number (empty, "abc", "1.5.2", "1e3") becomes NaN, which the server refuses with a
 * message naming the row.
 */
export function parseDecimal(s: string): number {
  const text = s.trim().replace(",", ".");
  return /^\d+(\.\d+)?$/.test(text) ? Number(text) : Number.NaN;
}
