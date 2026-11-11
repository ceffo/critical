// The lookup's opening view: a caster with spells opens on "Mine", any other keeps the catalogue
// (Wizard); a free spell's link or a query already changed is left alone; Reset goes back to it.
import { beforeEach, describe, expect, it } from "vitest";
import { isQuerySet, LOOKUP_DEFAULT, useLookupStore } from "./lookup";

beforeEach(() => {
  useLookupStore.setState({
    query: LOOKUP_DEFAULT,
    home: LOOKUP_DEFAULT,
    openedFor: null,
    grant: null,
  });
});

describe("lookup opening view", () => {
  it("opens a caster with spells on Mine", () => {
    useLookupStore.getState().openFor(1, true);
    const s = useLookupStore.getState();
    expect(s.query.cls).toBe("mine");
    expect(isQuerySet(s.query, s.home)).toBe(false);
  });
  it("keeps the catalogue for a caster with no spells and a non-caster", () => {
    useLookupStore.getState().openFor(1, false);
    expect(useLookupStore.getState().query.cls).toBe("wizard");
  });
  it("decides once per character, so a later pick stays", () => {
    useLookupStore.getState().openFor(1, true);
    useLookupStore.getState().setQuery({ cls: "all" });
    useLookupStore.getState().openFor(1, true);
    expect(useLookupStore.getState().query.cls).toBe("all");
  });
  it("leaves a free spell's link and a changed query alone", () => {
    useLookupStore.getState().setQuery({ levels: [2] });
    useLookupStore.getState().openFor(1, true);
    expect(useLookupStore.getState().query.cls).toBe("wizard");
  });
  it("Reset goes back to the opening view", () => {
    useLookupStore.getState().openFor(1, true);
    useLookupStore.getState().setQuery({ cls: "priest" });
    useLookupStore.getState().reset();
    expect(useLookupStore.getState().query.cls).toBe("mine");
  });
});
