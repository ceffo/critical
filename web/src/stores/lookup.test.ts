import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Spell } from "@/types/spells";

const searchSpells = vi.fn();
vi.mock("@/api/spells_db", async (orig) => ({
  ...(await orig<typeof import("@/api/spells_db")>()),
  searchSpells: (...a: unknown[]) => searchSpells(...a),
}));

import { LOOKUP_DEFAULT, isQuerySet, queryKey, useLookupStore } from "./lookup";

const spell = (id: string) => ({ id }) as Spell;

beforeEach(() => {
  searchSpells.mockReset();
  useLookupStore.setState({
    query: LOOKUP_DEFAULT,
    items: [],
    total: 0,
    pages: 0,
    shownKey: "",
    loading: false,
    error: null,
    selectedId: null,
  });
});

describe("lookup query", () => {
  it("opens on Wizard spells, and Reset only shows once something differs", () => {
    expect(useLookupStore.getState().query.cls).toBe("wizard");
    expect(isQuerySet(LOOKUP_DEFAULT)).toBe(false);
    expect(isQuerySet({ ...LOOKUP_DEFAULT, q: "  " })).toBe(false); // blank text is no search
    expect(isQuerySet({ ...LOOKUP_DEFAULT, q: "fire" })).toBe(true);
    expect(isQuerySet({ ...LOOKUP_DEFAULT, cls: "all" })).toBe(true);
    expect(isQuerySet({ ...LOOKUP_DEFAULT, levels: [1] })).toBe(true);
  });

  it("keeps the school when switching between non-Priest modes, drops it entering or leaving Priest", () => {
    const { setQuery } = useLookupStore.getState();
    setQuery({ school: "abjuration", cls: "all" });
    expect(useLookupStore.getState().query.school).toBe("abjuration");
    setQuery({ cls: "priest" });
    expect(useLookupStore.getState().query).toMatchObject({
      cls: "priest",
      school: "",
      sphere: "",
    });
    setQuery({ sphere: "Elemental Fire" });
    setQuery({ cls: "wizard" });
    expect(useLookupStore.getState().query.sphere).toBe("");
  });

  it("a Reset goes back to the default and clears the selection", () => {
    const s = useLookupStore.getState();
    s.setQuery({ q: "fire", cls: "all", levels: [4] });
    s.select("fire_trap");
    s.reset();
    expect(useLookupStore.getState().query).toEqual(LOOKUP_DEFAULT);
    expect(useLookupStore.getState().selectedId).toBeNull();
  });

  it("queryKey ignores surrounding spaces in the text", () => {
    expect(queryKey({ ...LOOKUP_DEFAULT, q: " fire " })).toBe(
      queryKey({ ...LOOKUP_DEFAULT, q: "fire" }),
    );
  });
});

describe("lookup search", () => {
  it("selects the first row of a search, and skips the request when its rows are on screen", async () => {
    searchSpells.mockResolvedValue({
      items: [spell("a"), spell("b")],
      total: 2,
    });
    await useLookupStore.getState().search();
    expect(useLookupStore.getState()).toMatchObject({
      selectedId: "a",
      total: 2,
      pages: 1,
    });
    await useLookupStore.getState().search();
    expect(searchSpells).toHaveBeenCalledTimes(1);
  });

  it("a slow response overtaken by a newer query is not written", async () => {
    let release!: (v: unknown) => void;
    searchSpells.mockImplementationOnce(
      () => new Promise((r) => (release = r)),
    );
    searchSpells.mockResolvedValueOnce({
      items: [spell("fireball")],
      total: 1,
    });
    useLookupStore.getState().setQuery({ q: "fir" });
    const slow = useLookupStore.getState().search();
    useLookupStore.getState().setQuery({ q: "fireball" });
    await useLookupStore.getState().search();
    release({ items: [spell("x"), spell("y")], total: 2 });
    await slow;
    expect(useLookupStore.getState().items.map((s) => s.id)).toEqual([
      "fireball",
    ]);
    expect(useLookupStore.getState().total).toBe(1);
  });

  it("loadMore appends the next page without duplicates and keeps the true total", async () => {
    searchSpells.mockResolvedValueOnce({
      items: [spell("a"), spell("b")],
      total: 3,
    });
    await useLookupStore.getState().search();
    searchSpells.mockResolvedValueOnce({
      items: [spell("b"), spell("c")],
      total: 3,
    });
    await useLookupStore.getState().loadMore();
    expect(searchSpells).toHaveBeenLastCalledWith(
      expect.objectContaining({ cls: "wizard" }),
      2,
    );
    expect(useLookupStore.getState().items.map((s) => s.id)).toEqual([
      "a",
      "b",
      "c",
    ]);
    await useLookupStore.getState().loadMore(); // everything is loaded
    expect(searchSpells).toHaveBeenCalledTimes(2);
  });

  it("move steps the selection within the rows, loads the next page near the end", async () => {
    const ids = Array.from({ length: 10 }, (_, i) => `s${i}`);
    searchSpells.mockResolvedValueOnce({ items: ids.map(spell), total: 12 });
    await useLookupStore.getState().search();
    useLookupStore.getState().move(-1);
    expect(useLookupStore.getState().selectedId).toBe("s0"); // clamped at the top
    searchSpells.mockResolvedValueOnce({
      items: [spell("s10"), spell("s11")],
      total: 12,
    });
    useLookupStore.getState().move(6); // within 5 of the end of the loaded rows
    expect(useLookupStore.getState().selectedId).toBe("s6");
    await vi.waitFor(() => expect(searchSpells).toHaveBeenCalledTimes(2));
    await vi.waitFor(() =>
      expect(useLookupStore.getState().items).toHaveLength(12),
    );
    useLookupStore.getState().move(99);
    expect(useLookupStore.getState().selectedId).toBe("s11");
  });

  it("moves asked for while the next page loads land after it, not clamped to the last row", async () => {
    const ids = Array.from({ length: 10 }, (_, i) => `s${i}`);
    searchSpells.mockResolvedValueOnce({ items: ids.map(spell), total: 20 });
    await useLookupStore.getState().search();
    useLookupStore.getState().select("s8");
    let release!: (v: unknown) => void;
    searchSpells.mockReturnValueOnce(new Promise((r) => (release = r)));
    const { move } = useLookupStore.getState();
    move(1); // s9, the last loaded row: the next page starts loading
    move(1);
    move(1);
    move(1);
    expect(useLookupStore.getState().selectedId).toBe("s9");
    move(-1); // an Up cancels one of the waiting Downs
    expect(useLookupStore.getState().selectedId).toBe("s9");
    release({
      items: Array.from({ length: 10 }, (_, i) => spell(`s${10 + i}`)),
      total: 20,
    });
    await vi.waitFor(() =>
      expect(useLookupStore.getState().items).toHaveLength(20),
    );
    // 8 + 4 Downs - 1 Up = 11
    expect(useLookupStore.getState().selectedId).toBe("s11");
  });

  it("a new search drops the moves still waiting", async () => {
    const ids = Array.from({ length: 4 }, (_, i) => `s${i}`);
    searchSpells.mockResolvedValueOnce({ items: ids.map(spell), total: 8 });
    await useLookupStore.getState().search();
    useLookupStore.getState().select("s3");
    let release!: (v: unknown) => void;
    searchSpells.mockReturnValueOnce(new Promise((r) => (release = r)));
    useLookupStore.getState().move(3);
    useLookupStore.getState().setQuery({ q: "x" });
    searchSpells.mockResolvedValueOnce({
      items: [spell("x0"), spell("x1"), spell("x2")],
      total: 3,
    });
    await useLookupStore.getState().search();
    release({ items: [spell("s4")], total: 8 });
    await Promise.resolve();
    expect(useLookupStore.getState().selectedId).toBe("x0");
  });
});
