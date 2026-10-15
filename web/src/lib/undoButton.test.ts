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
    expect(
      undoButtonState(
        null,
        [{ ...initiative, status: "voided" }],
        false,
        await makeT("fr"),
      ),
    ).toEqual({
      disabled: true,
      label: "Rien à annuler",
    });
  });

  it("names the action on top, worded from its payload, en and fr", async () => {
    expect(undoButtonState(7, [initiative], false, await makeT("en"))).toEqual({
      disabled: false,
      label: "Undo: Initiative 4 (d10) for turn 2 (Ctrl+Z / ⌘Z)",
    });
    expect(undoButtonState(7, [initiative], false, await makeT("fr"))).toEqual({
      disabled: false,
      label: "Annuler : Initiative 4 (d10) pour le tour 2 (Ctrl+Z / ⌘Z)",
    });
  });

  it("disabled while an action runs", async () => {
    expect(
      undoButtonState(7, [initiative], true, await makeT("en")).disabled,
    ).toBe(true);
  });

  it("a top older than the listed entries is named by its id", async () => {
    expect(
      undoButtonState(3, [initiative], false, await makeT("en")).label,
    ).toBe("Undo: #3 (Ctrl+Z / ⌘Z)");
  });

  // ADR 0010 step 6: a correction on top is named from its payload (the change, before → after).
  it("names a correction on top, en and fr", async () => {
    const correction = (payload: object): LogEntry => ({
      ...initiative,
      id: 9,
      kind: "correction",
      payload: { entryId: 7, entryKind: "attack", ...payload },
    });
    const label = async (lng: string, payload: object) =>
      undoButtonState(9, [correction(payload)], false, await makeT(lng)).label;
    const ac = { field: "targetAc", from: 5, to: 7 };
    expect(await label("fr", ac)).toBe(
      "Annuler : Correction CA 5 → 7 (Ctrl+Z / ⌘Z)",
    );
    expect(await label("en", ac)).toBe(
      "Undo: Correction AC 5 → 7 (Ctrl+Z / ⌘Z)",
    );
    expect(await label("fr", { field: "targetAc", from: null, to: 4 })).toBe(
      "Annuler : Correction CA ? → 4 (Ctrl+Z / ⌘Z)",
    );
    const save = {
      entryKind: "save",
      field: "saveAdjustment",
      from: 0,
      to: -3,
    };
    expect(await label("fr", save)).toBe(
      "Annuler : Correction ajustement de sauvegarde 0 → −3 (Ctrl+Z / ⌘Z)",
    );
    expect(await label("en", save)).toBe(
      "Undo: Correction save adjustment 0 → −3 (Ctrl+Z / ⌘Z)",
    );
    const verdict = { field: "verdict", from: null, to: "hit" };
    expect(await label("fr", verdict)).toBe(
      "Annuler : Correction verdict du MJ aucun → Touché (Ctrl+Z / ⌘Z)",
    );
    expect(
      await label("en", { field: "verdict", from: "hit", to: "miss" }),
    ).toBe("Undo: Correction GM verdict Hit → Miss (Ctrl+Z / ⌘Z)");
  });
});
