import { describe, expect, it } from "vitest";
import type { CharacterList } from "@/types/character";
import {
  initialRoster,
  rosterReducer,
  startedWrite,
  type RosterAction,
  type RosterState,
} from "./rosterWrites";

const identity = {
  race: "human",
  alignment: "true_neutral",
  sex: "female",
  missing: [] as [],
  currentHp: 8,
  maxHp: 12,
};
const entry = (cls: string, level: number) => ({
  class: cls,
  level,
  startingXp: 0,
  state: "active" as const,
  leftAtLevel: 0,
});
const lyra = { id: 1, name: "Lyra", classes: [entry("bard", 3)], ...identity };
const borin = {
  id: 2,
  name: "Borin",
  classes: [entry("fighter", 1)],
  ...identity,
};
const list = (
  names: [string, string],
  currentId: number | null = 1,
): CharacterList => ({
  characters: [
    { ...lyra, name: names[0] },
    { ...borin, name: names[1] },
  ],
  currentId,
});

const run = (s: RosterState, ...actions: RosterAction[]) =>
  actions.reduce(rosterReducer, s);
const loaded = run(initialRoster, {
  type: "loaded",
  list: list(["Lyra", "Borin"]),
});

/** Starts a write and returns the state and its id. */
function start(s: RosterState, fromDialog = true): [RosterState, number] {
  const after = rosterReducer(s, { type: "start", fromDialog });
  const id = startedWrite(s, after);
  expect(id).not.toBeNull();
  return [after, id!];
}

describe("roster writes", () => {
  it("a dialog knows whether its character is the one in play", () => {
    const s = run(loaded, { type: "open", kind: "retire", character: lyra });
    expect(s.opened).toMatchObject({ kind: "retire", current: true });
    const t = run(loaded, { type: "open", kind: "retire", character: borin });
    expect(t.opened).toMatchObject({ current: false });
  });

  it("a write's answer closes its dialog and shows the list it answered", () => {
    const opened = run(loaded, {
      type: "open",
      kind: "rename",
      character: lyra,
    });
    const [pending, id] = start(opened);
    const s = run(pending, {
      type: "done",
      id,
      list: list(["Lyra la Brave", "Borin"]),
    });
    expect(s.opened).toBeNull();
    expect(s.pending).toBeNull();
    expect(s.list?.characters[0]?.name).toBe("Lyra la Brave");
  });

  it("an earlier rename never closes a dialog opened after it", () => {
    // The player renames Lyra, closes the dialog while the request is pending and asks to rename
    // Borin: no dialog opens until the first answer, so its answer cannot close the second one.
    const opened = run(loaded, {
      type: "open",
      kind: "rename",
      character: lyra,
    });
    const [pending, id] = start(opened);
    const closed = run(pending, { type: "close" });
    const second = run(closed, {
      type: "open",
      kind: "rename",
      character: borin,
    });
    expect(second.opened).toBeNull();
    const answered = run(second, {
      type: "done",
      id,
      list: list(["Lyra la Brave", "Borin"]),
    });
    expect(answered.list?.characters[0]?.name).toBe("Lyra la Brave");
    // Once answered, the second rename opens and stays open.
    const reopened = run(answered, {
      type: "open",
      kind: "rename",
      character: borin,
    });
    expect(reopened.opened?.character.id).toBe(borin.id);
  });

  it("a dialog opened after a write began stays open when the write answers", () => {
    // A switch (no dialog) is pending; whatever is open then is not that write's to close.
    const withDialog = run(loaded, {
      type: "open",
      kind: "rename",
      character: borin,
    });
    const [pending, id] = start(withDialog, false);
    const s = run(pending, { type: "done", id, list: list(["Lyra", "Borin"]) });
    expect(s.opened?.character.id).toBe(borin.id);
  });

  it("no second write starts while one is pending", () => {
    const [pending] = start(loaded);
    const again = rosterReducer(pending, { type: "start", fromDialog: true });
    expect(startedWrite(pending, again)).toBeNull();
    expect(again.pending).toBe(pending.pending);
  });

  it("an older answer never replaces the list", () => {
    // The first write failed; a second one is pending when a late answer for the first arrives.
    const [first, firstId] = start(loaded);
    const failed = run(first, { type: "failed", id: firstId });
    const [second, secondId] = start(failed);
    const late = run(second, {
      type: "done",
      id: firstId,
      list: list(["Old", "Borin"]),
    });
    expect(late.list?.characters[0]?.name).toBe("Lyra");
    expect(late.pending?.id).toBe(secondId);
  });

  it("a refused write keeps its dialog open, and the next write may start", () => {
    const opened = run(loaded, {
      type: "open",
      kind: "rename",
      character: lyra,
    });
    const [pending, id] = start(opened);
    const s = run(pending, { type: "failed", id });
    expect(s.opened?.character.id).toBe(lyra.id);
    expect(s.pending).toBeNull();
    start(s);
  });
});
