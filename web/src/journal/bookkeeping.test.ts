import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import type { LogEntry, LogKind } from "@/types/log";
import { undoButtonState } from "@/lib/undoButton";
import { filterEntries, isBookkeeping } from "./entry";
import { KIND_VIEWS } from "./registry";

function entry(kind: LogKind, id: number, payload: unknown = null): LogEntry {
  return {
    id,
    transactionId: id,
    characterId: 1,
    round: 0,
    kind,
    summary: "",
    hpDelta: 0,
    effectiveHpDelta: 0,
    note: "",
    effectiveSummary: "",
    status: "active",
    payload,
    effectivePayload: payload,
    snapshots: [],
    corrections: [],
    createdAt: "2026-11-11T00:00:00Z",
  };
}

const spellAdd = (id: number) =>
  entry("spell_add", id, { spellId: "alarm", origin: "given" });

describe("journal bookkeeping filter", () => {
  // The decided list: setup and bookkeeping, hidden by the default view; read from the registry.
  it("flags exactly the setup kinds", () => {
    const flagged = (Object.keys(KIND_VIEWS) as LogKind[]).filter(
      (k) => KIND_VIEWS[k].bookkeeping,
    );
    expect(flagged.sort()).toEqual(
      [
        "spell_add",
        "item_add",
        "item_edit",
        "item_remove",
        "item_restock",
        "weapon_add",
        "weapon_edit",
        "weapon_remove",
        "equipment_state",
        "skill_points",
        "proficiencies",
        "skill_armor",
        "psionic_pick",
        "ac_override",
        "spell_prep",
        "round_set",
        "round_reset",
      ].sort(),
    );
  });

  it("hides 17 hand-added spells by default and keeps rolls, casts, damage and progress", () => {
    const setup = Array.from({ length: 17 }, (_, i) => spellAdd(i + 1));
    const kept = (
      ["attack", "save", "cast", "hp_change", "buff_change", "dice"] as const
    ).map((k, i) => entry(k, 100 + i));
    const progress = (
      ["spell_learn", "xp_add", "xp_edit", "xp_remove", "class_switch"] as const
    ).map((k, i) => entry(k, 200 + i));
    const all = [...setup, ...kept, ...progress];
    const shown = filterEntries(all, "hideSetup");
    expect(shown.map((e) => e.id)).toEqual(
      [...kept, ...progress].map((e) => e.id),
    );
    expect(filterEntries(all, "all")).toHaveLength(all.length);
    expect(shown.some(isBookkeeping)).toBe(false);
  });

  it("equipping one item is one entry, hidden and back through All", () => {
    const equip = entry("equipment_state", 1, {
      itemId: "sword",
      name: "Sword",
      before: "carried",
      after: "wielded",
    });
    expect(filterEntries([equip], "hideSetup")).toEqual([]);
    expect(filterEntries([equip], "all")).toEqual([equip]);
  });

  it("an unknown kind is shown", () => {
    const odd = {
      ...entry("dice", 1),
      kind: "future_kind",
    } as unknown as LogEntry;
    expect(filterEntries([odd], "hideSetup")).toEqual([odd]);
  });

  // Undo pops the newest transaction even when the default view hides its row: its tooltip names it.
  it("Undo names a hidden top entry, en and fr", async () => {
    const make = async (lng: string) => {
      const inst = i18next.createInstance();
      await inst.init({
        lng,
        fallbackLng: "en",
        defaultNS: "common",
        resources,
        interpolation: { escapeValue: false },
      });
      return inst.t;
    };
    const list = [spellAdd(2), entry("attack", 1)];
    expect(filterEntries(list, "hideSetup").map((e) => e.id)).toEqual([1]);
    expect(undoButtonState(2, list, false, await make("en")).label).toContain(
      "Added to the spellbook",
    );
    expect(undoButtonState(2, list, false, await make("fr")).label).toContain(
      "Ajouté au grimoire",
    );
  });
});
