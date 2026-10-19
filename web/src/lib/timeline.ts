// Layout of the round header's effects track: which round columns are shown and where a buff's
// elapsed and remaining parts fall. Display only: the rounds left (`duration`) and the round the
// buff started in (`startRound`) come from the server; nothing here decides when a buff ends.

/**
 * Round columns the track shows, at every width (ADR 0027 step 4.3): the current round, two of history
 * and two ahead. Extra width widens the columns and the effect-name column instead.
 */
export const TRACK_COLUMNS = 5;

/**
 * First round shown: two rounds of history before the current one, never before round 1. Out of
 * combat (round 0) the window starts at 0 so the current column is shown.
 */
export function windowStart(round: number): number {
  return round <= 0 ? 0 : Math.max(1, round - 2);
}

/** A run of columns, 0-based from the window's first column, `to` inclusive. */
export interface ColumnRun {
  from: number;
  to: number;
}

export interface EffectSpan {
  /** Rounds already elapsed, from the buff's start round up to the round before the current one. */
  past: ColumnRun | null;
  /** The current round and what is left: `duration` columns for a timed buff, to the edge otherwise. */
  remaining: ColumnRun | null;
  /** The remaining part runs past the window's right edge (the pill fades out). */
  overflows: boolean;
}

/**
 * Where a buff sits in a window of `size` columns starting at round `start`. A timed buff
 * (`duration` > 0) covers the current round and the `duration - 1` after it; an untimed one runs to
 * the edge. `startRound` above the current round (after End combat or time passing) has no past part.
 */
export function effectSpan(opts: {
  round: number;
  start: number;
  size: number;
  duration: number;
  startRound: number;
}): EffectSpan {
  const { round, start, size, duration, startRound } = opts;
  const last = size - 1;
  const current = round - start;
  const pastFrom = Math.max(startRound - start, 0);
  const past =
    startRound < round && current - 1 >= pastFrom
      ? { from: pastFrom, to: Math.min(current - 1, last) }
      : null;
  if (current > last) return { past, remaining: null, overflows: true };
  const end = duration > 0 ? current + duration - 1 : Number.POSITIVE_INFINITY;
  return {
    past,
    remaining: { from: current, to: Math.min(end, last) },
    overflows: end > last,
  };
}
