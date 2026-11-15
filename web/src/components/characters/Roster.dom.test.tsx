import { afterEach, describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import { renderApp } from "@/test/render";
import type { CharacterList, CharacterSummary } from "@/types/character";
import { Roster } from "./Roster";

// The footer reads the running version from the server: none here.
vi.mock("@/components/shell/VersionBadge", () => ({
  VersionBadge: () => null,
}));

const fetchCharacters = vi.fn();
const copyCharacter = vi.fn();
const switchCharacter = vi.fn();
vi.mock("@/api/characters", () => ({
  fetchCharacters: () => fetchCharacters() as unknown,
  fetchOthers: () => Promise.resolve({ characters: [] }),
  fetchCreationOptions: () => Promise.resolve({ maxNameLength: 40 }),
  copyCharacter: (id: number, name: string) =>
    copyCharacter(id, name) as unknown,
  switchCharacter: (id: number) => switchCharacter(id) as unknown,
}));

const bard = (id: number, name: string): CharacterSummary => ({
  id,
  name,
  classes: [
    { class: "bard", level: 3, startingXp: 0, state: "active", leftAtLevel: 0 },
  ],
  currentHp: 9,
  maxHp: 12,
});

afterEach(() => {
  fetchCharacters.mockReset();
  copyCharacter.mockReset();
  switchCharacter.mockReset();
});

describe("Roster: copy", () => {
  it.each([
    [
      "en",
      "Copy",
      "Lyra (copy)",
      "A copy of Lyra was added to your characters.",
    ],
    [
      "fr",
      "Copier",
      "Lyra (copie)",
      "Une copie de Lyra a été ajoutée à vos personnages.",
    ],
  ] as const)(
    "%s: a card's %s sends the name in the player's language, and the copy shows, not current",
    async (lng, label, name, said) => {
      fetchCharacters.mockResolvedValue({
        characters: [bard(1, "Lyra")],
        currentId: 1,
      } satisfies CharacterList);
      copyCharacter.mockResolvedValue({
        characters: [bard(1, "Lyra"), bard(2, name)],
        currentId: 1,
        copiedId: 2,
      } satisfies CharacterList);
      const { user } = await renderApp(<Roster />, { lng });
      const cards = await screen.findAllByTestId("roster-card");
      expect(cards).toHaveLength(1);
      const button = within(cards[0]!).getByTestId("roster-copy");
      expect(button).toHaveTextContent(label);
      await user.click(button);
      expect(copyCharacter).toHaveBeenCalledWith(1, name);
      expect(switchCharacter).not.toHaveBeenCalled();
      const after = await screen.findAllByTestId("roster-card");
      expect(after).toHaveLength(2);
      expect(after.filter((c) => c.hasAttribute("data-current"))).toHaveLength(
        1,
      );
      expect(within(after[0]!).getByText("Lyra")).toBeInTheDocument();
      expect(screen.getByTestId("roster-copied")).toHaveTextContent(said);
    },
  );

  it("shortens a long name so the copy's name fits the server's limit", async () => {
    const long = "L".repeat(40);
    fetchCharacters.mockResolvedValue({
      characters: [bard(1, long)],
      currentId: 1,
    });
    copyCharacter.mockResolvedValue({
      characters: [bard(1, long)],
      currentId: 1,
    });
    const { user } = await renderApp(<Roster />);
    await user.click(await screen.findByTestId("roster-copy"));
    expect(copyCharacter).toHaveBeenCalledWith(1, `${"L".repeat(33)} (copy)`);
  });

  it("says why a copy failed, and the list stays as it was", async () => {
    fetchCharacters.mockResolvedValue({
      characters: [bard(1, "Lyra")],
      currentId: 1,
    });
    copyCharacter.mockRejectedValue(new Error("Character not found."));
    const { user } = await renderApp(<Roster />);
    await user.click(await screen.findByTestId("roster-copy"));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Character not found.",
    );
    expect(screen.getAllByTestId("roster-card")).toHaveLength(1);
    expect(screen.queryByTestId("roster-copied")).toBeNull();
  });
});
