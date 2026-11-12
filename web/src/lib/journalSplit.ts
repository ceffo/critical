// The split between the Act activity's "now" band and the journal (ADR 0022 step 5.4): what height
// a dragged journal may take. Display only.

/** The journal is at least 14rem tall (ADR 0022 step 2.5). */
export const JOURNAL_MIN_PX = 224;
/** The now band keeps room for the act panel's tabs and a first row when the journal is dragged up. */
export const NOW_MIN_PX = 160;
/**
 * The now band's resting height: that of the tallest act tab's content (Turning's: 676 px measured, with its tab header), the
 * same for every tab so the journal's top edge never moves when the tab changes (ADR 0027 step 6.1).
 * It gives way to the journal's minimum in a short stage; a taller tab scrolls inside the panel.
 */
export const NOW_BAND_PX = 680;
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
