import { Drop, Heart } from "@phosphor-icons/react";
import { describe, expect, it } from "vitest";
import type { LogEntry } from "@/types/log";
import { entryIcon } from "../entry";

// ADR 0027 step 9.7: damage is a drop, healing the heart.
describe("an HP change's journal icon", () => {
  const hp = (hpDelta: number) => ({ kind: "hp_change", hpDelta }) as LogEntry;
  it("is a drop for damage and a heart for healing", () => {
    expect(entryIcon(hp(-7))).toBe(Drop);
    expect(entryIcon(hp(3))).toBe(Heart);
  });
});
