import { describe, expect, it } from "vitest";
import { clampJournalHeight, JOURNAL_MIN_PX, NOW_MIN_PX } from "./journalSplit";

describe("journal split", () => {
  const stage = 700;
  const handle = 24;

  it("keeps a wanted height that fits", () => {
    expect(clampJournalHeight(300.4, stage, handle)).toBe(300);
  });

  it("never goes below the journal's minimum", () => {
    expect(clampJournalHeight(10, stage, handle)).toBe(JOURNAL_MIN_PX);
  });

  it("leaves the now band its minimum", () => {
    expect(clampJournalHeight(5000, stage, handle)).toBe(
      stage - handle - NOW_MIN_PX,
    );
  });

  it("a stage too short for both keeps the journal's minimum", () => {
    expect(clampJournalHeight(400, 300, handle)).toBe(JOURNAL_MIN_PX);
  });
});
