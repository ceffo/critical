import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, screen, waitFor } from "@testing-library/react";
import { renderApp } from "@/test/render";
import type { Spell } from "@/types/spells";

const searchSpells = vi.hoisted(() => vi.fn());
vi.mock("@/api/spells_db", async (orig) => ({
  ...(await orig<typeof import("@/api/spells_db")>()),
  searchSpells,
  getSpellFilterOptions: () => Promise.resolve({ schools: [], spheres: [] }),
}));
// The detail pane and the learn button have their own tests; here only the list and its keys.
vi.mock("@/components/dashboard/SpellDetail", () => ({
  SpellDetail: ({ spell }: { spell: Spell }) => (
    <p data-testid="detail">{spell.id}</p>
  ),
  SpellName: ({ spell }: { spell: Spell }) => <span>{spell.id}</span>,
}));
vi.mock("@/components/actions/LearnSpell", () => ({ LearnSpell: () => null }));

import { useLookupStore, LOOKUP_DEFAULT } from "@/stores/lookup";
import { LookupPanel } from "./LookupPanel";

const spell = (id: string) =>
  ({ id, class: "Wizard", level: 1, school: { en: "x" } }) as unknown as Spell;
const rows = ["alpha", "bravo", "charlie"].map(spell);

beforeEach(() => {
  searchSpells.mockReset();
  searchSpells.mockResolvedValue({ items: rows, total: rows.length });
  Element.prototype.scrollIntoView = vi.fn();
  useLookupStore.setState({
    query: LOOKUP_DEFAULT,
    items: [],
    total: 0,
    pages: 0,
    shownKey: "",
    loading: false,
    error: null,
    selectedId: null,
    options: null,
    grant: null,
  });
});

const selectedRow = () =>
  screen.getByRole("option", { selected: true }).getAttribute("data-id");

describe("spell lookup list", () => {
  it("selects the first match, moves the selection with the arrows from the search box, and a click selects", async () => {
    const { user } = await renderApp(<LookupPanel />);
    await waitFor(() => expect(selectedRow()).toBe("alpha"));
    expect(screen.getByTestId("detail")).toHaveTextContent("alpha");

    const box = screen.getByRole("combobox", { name: "Search spells..." });
    await user.click(box);
    await user.keyboard("{ArrowDown}");
    expect(selectedRow()).toBe("bravo");
    await user.keyboard("{ArrowDown}{ArrowDown}");
    expect(selectedRow()).toBe("charlie");
    await user.keyboard("{ArrowUp}");
    expect(selectedRow()).toBe("bravo");
    expect(box).toHaveAttribute("aria-activedescendant", "lookup-row-bravo");

    await user.click(screen.getAllByRole("option")[0]);
    expect(selectedRow()).toBe("alpha");
    expect(screen.getByTestId("detail")).toHaveTextContent("alpha");
  });

  it("arrow-down past the loaded rows waits for the next page and lands on its first row", async () => {
    const page1 = Array.from({ length: 4 }, (_, i) => spell(`a${i}`));
    const page2 = [spell("b0"), spell("b1")];
    let land: (r: { items: Spell[]; total: number }) => void = () => {};
    searchSpells.mockImplementation((_q: unknown, page: number) =>
      page === 1
        ? Promise.resolve({ items: page1, total: 6 })
        : new Promise((resolve) => (land = resolve)),
    );
    const { user } = await renderApp(<LookupPanel />);
    await waitFor(() => expect(selectedRow()).toBe("a0"));
    await user.click(screen.getByRole("combobox"));
    // Four steps down from the first row: the last one is past the loaded rows, the page is still out.
    await user.keyboard("{ArrowDown}{ArrowDown}{ArrowDown}{ArrowDown}");
    expect(selectedRow()).toBe("a3");
    expect(searchSpells).toHaveBeenCalledWith(expect.anything(), 2);
    act(() => land({ items: page2, total: 6 }));
    await waitFor(() => expect(selectedRow()).toBe("b0"));
    expect(screen.getAllByRole("option")).toHaveLength(6);
  });
});
