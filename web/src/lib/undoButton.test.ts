import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import type { LogEntry } from "@/types/log";
import { undoButtonState } from "./undoButton";

async function makeT(lng: string) {
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

const initiative: LogEntry = {
  id: 7,
  characterId: 1,
  round: 2,
  kind: "initiative",
  summary: "",
  hpDelta: 0,
  effectiveHpDelta: 0,
  effectiveSummary: "",
  note: "",
  status: "active",
  payload: { round: 2, d10: 4 },
  effectivePayload: { round: 2, d10: 4 },
  snapshots: [],
  corrections: [],
  createdAt: "2026-10-15T00:00:00Z",
};

describe("undoButtonState", () => {
  // Checklist: an empty log (fresh, or after popping everything) disables the button.
  it("nothing to pop: disabled, en and fr", async () => {
    expect(undoButtonState(null, [], false, await makeT("en"))).toEqual({
      disabled: true,
      label: "Nothing to undo",
    });
    expect(undoButtonState(null, [{ ...initiative, status: "voided" }], false, await makeT("fr"))).toEqual({
      disabled: true,
      label: "Rien à annuler",
    });
  });

  it("names the action on top, worded from its payload, en and fr", async () => {
    expect(undoButtonState(7, [initiative], false, await makeT("en"))).toEqual({
      disabled: false,
      label: "Undo: Initiative 4 (d10) for round 2 (Ctrl+Z / ⌘Z)",
    });
    expect(undoButtonState(7, [initiative], false, await makeT("fr"))).toEqual({
      disabled: false,
      label: "Annuler : Initiative 4 (d10) pour le tour 2 (Ctrl+Z / ⌘Z)",
    });
  });

  it("disabled while an action runs", async () => {
    expect(undoButtonState(7, [initiative], true, await makeT("en")).disabled).toBe(true);
  });

  it("a top older than the listed entries is named by its id", async () => {
    expect(undoButtonState(3, [initiative], false, await makeT("en")).label).toBe(
      "Undo: #3 (Ctrl+Z / ⌘Z)",
    );
  });
});
