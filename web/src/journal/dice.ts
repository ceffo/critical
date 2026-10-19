// Dice as the result card shows them: one chip per die, its maximum in the accent, a 1 dimmed.

/** One die as a chip: its value and emphasis. */
export interface DieChipView {
  value: number;
  tone: "max" | "min" | "plain";
}

/**
 * The faces of a single dice group ("6d6", "1d8+2" → 6, 8), to mark a die at its maximum or at 1;
 * null for anything else ("2d6+1d4").
 */
export function dieFaces(dice: string): number | null {
  const m = /^\s*\d*\s*d\s*(\d+)\s*([+-]\s*\d+\s*)?$/i.exec(dice);
  return m ? Number(m[1]) : null;
}

/** A die's emphasis: its maximum in the accent, a 1 dimmed. */
export function dieTone(
  value: number,
  faces: number | null,
): "max" | "min" | "plain" {
  if (faces !== null && value === faces) return "max";
  if (value === 1) return "min";
  return "plain";
}

/** Recorded dice ({sides, value}) as chips. */
export function chips(
  dice: readonly { sides: number; value: number }[],
): DieChipView[] {
  return dice.map((d) => ({ value: d.value, tone: dieTone(d.value, d.sides) }));
}
