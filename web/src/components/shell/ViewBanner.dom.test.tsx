import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { renderApp } from "@/test/render";
import type { SharedCharacter } from "@/types/character";
import { ViewBanner } from "./ViewBanner";

const fetchOthers = vi.fn();
const copyCharacter = vi.fn();
vi.mock("@/api/characters", () => ({
  fetchOthers: () => fetchOthers() as unknown,
  fetchCreationOptions: () => Promise.reject(new Error("no options here")),
  copyCharacter: (id: number, name: string) =>
    copyCharacter(id, name) as unknown,
}));

const listed = (ownerName: string): SharedCharacter => ({
  id: 4,
  name: "Ilyana",
  classes: [],
  owner: { id: "a", name: ownerName },
});

const assign = vi.fn();
beforeEach(() => {
  vi.stubGlobal("location", { ...globalThis.location, assign });
});
afterEach(() => {
  vi.unstubAllGlobals();
  fetchOthers.mockReset();
  copyCharacter.mockReset();
  assign.mockReset();
});

describe("ViewBanner", () => {
  it.each([
    ["en", "Ada", "Viewing Ilyana, played by Ada."],
    ["fr", "Ada", "Vous consultez Ilyana, joué par Ada."],
    ["en", "", "Viewing Ilyana, played by another player."],
    ["fr", "", "Vous consultez Ilyana, joué par un autre joueur."],
  ] as const)(
    "%s, owner %j: names the character and its owner",
    async (lng, owner, want) => {
      fetchOthers.mockResolvedValue({ characters: [listed(owner)] });
      await renderApp(<ViewBanner id={4} />, { lng });
      expect(await screen.findByText(want)).toBeInTheDocument();
    },
  );

  it("says it is read-only and takes the user back to their own character with a full load", async () => {
    fetchOthers.mockResolvedValue({ characters: [listed("Ada")] });
    const { user } = await renderApp(<ViewBanner id={4} />);
    expect(
      await screen.findByText("Read-only: nothing can be changed here."),
    ).toBeInTheDocument();
    await user.click(
      screen.getByRole("button", { name: "Back to your own character" }),
    );
    expect(assign).toHaveBeenCalledWith("/");
  });

  it.each([
    ["en", "Copy", "Ilyana (copy)"],
    ["fr", "Copier", "Ilyana (copie)"],
  ] as const)(
    "%s: Copy sends the name the web composed and shows the roster, where the copy is",
    async (lng, label, want) => {
      fetchOthers.mockResolvedValue({ characters: [listed("Ada")] });
      copyCharacter.mockResolvedValue({
        characters: [],
        currentId: 1,
        copiedId: 9,
      });
      const { user } = await renderApp(<ViewBanner id={4} />, { lng });
      await screen.findByText(/Ilyana, (played by|joué par) Ada/);
      const button = screen.getByTestId("view-copy");
      expect(button).toHaveTextContent(label);
      await user.click(button);
      expect(copyCharacter).toHaveBeenCalledWith(4, want);
      expect(assign).toHaveBeenCalledWith("/characters");
    },
  );

  it("says why a copy failed and stays on the page", async () => {
    fetchOthers.mockResolvedValue({ characters: [listed("Ada")] });
    copyCharacter.mockRejectedValue(new Error("Character not found."));
    const { user } = await renderApp(<ViewBanner id={4} />);
    await screen.findByText("Viewing Ilyana, played by Ada.");
    await user.click(screen.getByTestId("view-copy"));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Character not found.",
    );
    expect(assign).not.toHaveBeenCalled();
    expect(screen.getByTestId("view-copy")).toBeEnabled();
  });

  it("still shows the banner when the list fails", async () => {
    fetchOthers.mockRejectedValue(new Error("x"));
    await renderApp(<ViewBanner id={4} />);
    expect(await screen.findByTestId("view-banner")).toBeInTheDocument();
  });
});
