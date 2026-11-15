import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { renderApp } from "@/test/render";
import type { SharedCharacter } from "@/types/character";
import { Others } from "./Others";

const fetchOthers = vi.fn();
vi.mock("@/api/characters", () => ({
  fetchOthers: () => fetchOthers() as unknown,
}));

const mage = (
  id: number,
  name: string,
  owner: { id: string; name: string },
  level = 3,
): SharedCharacter => ({
  id,
  name,
  classes: [
    { class: "mage", level, startingXp: 0, state: "active", leftAtLevel: 0 },
  ],
  race: "elf",
  alignment: "neutral_good",
  currentHp: 11,
  maxHp: 14,
  owner,
});

const assign = vi.fn();
beforeEach(() => {
  vi.stubGlobal("location", { ...globalThis.location, assign });
});
afterEach(() => {
  vi.unstubAllGlobals();
  fetchOthers.mockReset();
  assign.mockReset();
});

describe("Others", () => {
  it.each([
    ["en", "Other players' characters", "Unnamed player", "Mage · level 3"],
    [
      "fr",
      "Personnages des autres joueurs",
      "Joueur sans nom",
      "Magicien · niveau 3",
    ],
  ] as const)(
    "%s: groups by owner, words an owner without a name itself, shows name, class and level",
    async (lng, title, unnamed, classLevel) => {
      fetchOthers.mockResolvedValue({
        characters: [
          mage(5, "Ilyana", { id: "a", name: "Ada" }),
          mage(6, "Orrin", { id: "a", name: "Ada" }, 5),
          mage(9, "Bram", { id: "b", name: "" }),
        ],
      });
      await renderApp(<Others />, { lng });
      expect(
        await screen.findByRole("heading", { name: title }),
      ).toBeInTheDocument();
      const owners = screen.getAllByTestId("roster-owner");
      expect(owners).toHaveLength(2);
      expect(
        within(owners[0]!).getByRole("heading", { name: "Ada" }),
      ).toBeInTheDocument();
      expect(within(owners[0]!).getAllByTestId("other-card")).toHaveLength(2);
      expect(within(owners[0]!).getByText("Ilyana")).toBeInTheDocument();
      expect(within(owners[0]!).getByText(classLevel)).toBeInTheDocument();
      expect(
        within(owners[1]!).getByRole("heading", { name: unnamed }),
      ).toBeInTheDocument();
    },
  );

  it("opens a character read-only through a full load on its own address", async () => {
    fetchOthers.mockResolvedValue({
      characters: [mage(5, "Ilyana", { id: "a", name: "Ada" })],
    });
    const { user } = await renderApp(<Others />);
    await user.click(
      await screen.findByRole("button", { name: "View Ilyana, read-only" }),
    );
    expect(assign).toHaveBeenCalledWith("/view/5");
  });

  it("shows nothing when no other user has a character", async () => {
    fetchOthers.mockResolvedValue({ characters: [] });
    const { container } = await renderApp(<Others />);
    await waitFor(() => expect(fetchOthers).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
  });

  it("says the list could not be loaded, in the player's language", async () => {
    fetchOthers.mockRejectedValue(new Error("boom"));
    await renderApp(<Others />, { lng: "fr" });
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Les personnages des autres joueurs n'ont pas pu être chargés.",
    );
  });
});
