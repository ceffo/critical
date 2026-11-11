import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import { renderApp } from "@/test/render";
import type { CastResult, Spell } from "@/types/spells";
import { AddEffect } from "./AddEffect";

const search = vi.hoisted(() => vi.fn());
const receive = vi.hoisted(() => vi.fn());
vi.mock("@/api/spells_db", () => ({
  NO_QUERY: {},
  searchSpells: search,
}));
vi.mock("@/api/spells", async (orig) => ({
  ...(await orig<object>()),
  receiveSpell: receive,
}));

const spell = (id: string, name: string, level: number) =>
  ({ id, name: { en: name }, class: "Priest", level }) as unknown as Spell;

async function openPicker(lng: "en" | "fr" = "en") {
  const view = await renderApp(<AddEffect />, { lng });
  await view.user.click(screen.getByTestId("add-effect"));
  return view;
}

describe("Received spell picker", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    receive.mockResolvedValue({ spellId: "x" } as CastResult);
  });

  it("applies the spell row that was clicked by its own id when two share a name", async () => {
    search.mockResolvedValue({
      items: [
        spell("fortify", "Fortify", 3),
        spell("fortify_l4", "Fortify", 4),
      ],
      total: 2,
    });
    const { user } = await openPicker();
    await user.type(
      screen.getByLabelText("Search for a received spell"),
      "Fortify",
    );
    await user.type(screen.getByLabelText("Caster level"), "9");
    const row = (await screen.findAllByRole("listitem"))[1];
    await user.click(within(row).getByRole("button", { name: /^Apply/ }));
    expect(receive).toHaveBeenCalledWith("fortify_l4", 9, undefined);
  });

  it("says how many matches are not listed", async () => {
    search.mockResolvedValue({
      items: Array.from({ length: 50 }, (_, i) =>
        spell(`cure_${i}`, `Cure Test ${i}`, 1),
      ),
      total: 60,
    });
    const { user } = await openPicker();
    await user.type(
      screen.getByLabelText("Search for a received spell"),
      "Cure Test",
    );
    expect(await screen.findByTestId("receive-more")).toHaveTextContent(
      "10 more, refine your search",
    );
    expect(screen.getAllByRole("listitem")).toHaveLength(50);
  });

  it.each([
    ["en", "Priest"],
    ["fr", "Prêtre"],
  ] as const)(
    "words the spell's class by its name in %s, never the catalogue code",
    async (lng, label) => {
      search.mockResolvedValue({
        items: [
          { id: "command", name: { en: "Command" }, class: "priest", level: 1 },
        ],
        total: 1,
      });
      const { user } = await openPicker(lng);
      await user.type(screen.getByRole("searchbox"), "comma");
      const row = await screen.findByRole("listitem");
      expect(row).toHaveTextContent(label);
      expect(row).not.toHaveTextContent("priest");
    },
  );
});
