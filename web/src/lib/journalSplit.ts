// The split between the Act activity's "now" band and the journal (ADR 0022 step 5.4): what height
// a dragged journal may take. Display only.

/** The journal is at least 14rem tall (ADR 0022 step 2.5). */
export const JOURNAL_MIN_PX = 224;
/** The now band keeps room for the act panel's tabs and a first row when the journal is dragged up. */
export const NOW_MIN_PX = 160;
/** One arrow-key press on the handle moves it by this much. */
export const SPLIT_STEP_PX = 16;

/**
 * The journal height to use for a wanted one, in a stage `stage` px tall whose handle takes
 * `handle` px: at least JOURNAL_MIN_PX, at most what leaves NOW_MIN_PX to the now band. When the
 * stage is too short for both minimums the journal keeps its own.
 */
export function clampJournalHeight(
  wanted: number,
  stage: number,
  handle: number,
): number {
  const max = Math.max(JOURNAL_MIN_PX, stage - handle - NOW_MIN_PX);
  return Math.round(Math.min(Math.max(wanted, JOURNAL_MIN_PX), max));
}
