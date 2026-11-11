import { describe, expect, it, vi } from "vitest";
import type { PowerPick } from "@/types/character";
import { usePickEdits } from "./usePickEdits";

const telepathy: PowerPick = { kind: "discipline", id: "telepathy" };
const contact: PowerPick = { kind: "power", id: "contact" };
const relearn: PowerPick = { kind: "relearn", id: "contact" };

describe("usePickEdits", () => {
  it("removing a relearn takes one of them only, the power and the other relearn stay", () => {
    const onChange = vi.fn();
    const edits = usePickEdits(
      [telepathy, contact, relearn, relearn],
      false,
      onChange,
    );
    edits.remove(relearn);
    expect(onChange).toHaveBeenCalledWith([telepathy, contact, relearn]);
  });

  it("removing a power takes its relearns with it, and nothing else", () => {
    const onChange = vi.fn();
    const other: PowerPick = { kind: "power", id: "mind_thrust" };
    const edits = usePickEdits(
      [telepathy, contact, other, relearn, relearn],
      false,
      onChange,
    );
    edits.remove(contact);
    expect(onChange).toHaveBeenCalledWith([telepathy, other]);
  });

  it("adds at the end, says what is picked, and sends nothing while busy", () => {
    const onChange = vi.fn();
    const edits = usePickEdits([telepathy], false, onChange);
    expect(edits.isPicked("discipline", "telepathy")).toBe(true);
    expect(edits.isPicked("power", "contact")).toBe(false);
    edits.add(contact);
    expect(onChange).toHaveBeenCalledWith([telepathy, contact]);

    const busy = vi.fn();
    usePickEdits([telepathy], true, busy).add(contact);
    expect(busy).not.toHaveBeenCalled();
  });
});
