import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import { renderApp } from "@/test/render";
import { useDiceStore } from "@/stores/dice";
import { DiceComposer } from "./DiceComposer";

const rollDice = vi.hoisted(() => vi.fn());
vi.mock("@/api/dice", () => ({
  rollDice,
  fetchFavourites: vi.fn().mockResolvedValue({ favourites: [] }),
  addFavourite: vi.fn(),
  deleteFavourite: vi.fn(),
  reorderFavourites: vi.fn(),
}));
vi.mock("@/stores/table", () => ({
  useTableStore: {
    getState: () => ({
      submit: async (action: () => Promise<unknown>) => {
        try {
          await action();
          return null;
        } catch (e) {
          return { field: "formula", message: (e as Error).message };
        }
      },
      fail: vi.fn(),
    }),
  },
}));

describe("dice composer collapse", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useDiceStore.setState({ open: false, formula: "", error: null });
    rollDice.mockResolvedValue({ formula: "d6", logEntryId: undefined });
  });

  it("starts as one button and expands into the formula field", async () => {
    const { user } = await renderApp(<DiceComposer />);
    const toggle = screen.getByRole("button", { name: "Dice" });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByLabelText("Formula")).toBeNull();
    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByLabelText("Formula")).toHaveFocus();
  });

  it("collapses on Escape and puts focus back on the button", async () => {
    const { user } = await renderApp(<DiceComposer />);
    await user.click(screen.getByRole("button", { name: "Dice" }));
    await user.keyboard("{Escape}");
    expect(screen.queryByLabelText("Formula")).toBeNull();
    expect(screen.getByRole("button", { name: "Dice" })).toHaveFocus();
  });

  it("expands by keyboard and collapses again after a roll", async () => {
    const { user } = await renderApp(<DiceComposer />);
    await user.tab();
    await user.keyboard("{Enter}");
    await user.type(screen.getByLabelText("Formula"), "d6{Enter}");
    await waitFor(() => expect(rollDice).toHaveBeenCalledWith("d6"));
    await waitFor(() => expect(screen.queryByLabelText("Formula")).toBeNull());
    expect(screen.getByRole("button", { name: "Dice" })).toHaveFocus();
  });

  it("stays open when the server refuses the formula", async () => {
    rollDice.mockRejectedValue(new Error("no"));
    const { user } = await renderApp(<DiceComposer />);
    await user.click(screen.getByRole("button", { name: "Dice" }));
    await user.type(screen.getByLabelText("Formula"), "d6{Enter}");
    await waitFor(() => expect(rollDice).toHaveBeenCalled());
    expect(screen.getByLabelText("Formula")).toBeInTheDocument();
  });

  it("a formula sent from elsewhere expands it", async () => {
    await renderApp(<DiceComposer />);
    useDiceStore.getState().fill("2d6");
    expect(await screen.findByLabelText("Formula")).toHaveValue("2d6");
  });
});
