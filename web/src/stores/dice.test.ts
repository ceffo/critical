import { beforeEach, describe, expect, it, vi } from "vitest";

const api = vi.hoisted(() => ({ reorder: vi.fn() }));

vi.mock("@/api/dice", () => ({
  reorderFavourites: api.reorder,
  addFavourite: vi.fn(),
  deleteFavourite: vi.fn(),
  fetchFavourites: vi.fn(),
  rollDice: vi.fn(),
}));

import { useDiceStore } from "./dice";

const favs = [
  { id: 4, formula: "d4" },
  { id: 6, formula: "d6" },
  { id: 8, formula: "d8" },
];

describe("moveFavourite", () => {
  beforeEach(() => {
    api.reorder.mockReset();
    useDiceStore.setState({ favourites: favs });
  });

  it("sends the whole new order and shows the server's answer", async () => {
    const answer = [favs[1], favs[0], favs[2]];
    api.reorder.mockResolvedValue({ favourites: answer });
    await useDiceStore.getState().moveFavourite(6, -1);
    expect(api.reorder).toHaveBeenCalledExactlyOnceWith([6, 4, 8]);
    expect(useDiceStore.getState().favourites).toEqual(answer);
  });

  it("moves a favourite later", async () => {
    api.reorder.mockResolvedValue({ favourites: favs });
    await useDiceStore.getState().moveFavourite(4, 1);
    expect(api.reorder).toHaveBeenCalledExactlyOnceWith([6, 4, 8]);
  });

  it("sends nothing past either end", async () => {
    await useDiceStore.getState().moveFavourite(4, -1);
    await useDiceStore.getState().moveFavourite(8, 1);
    expect(api.reorder).not.toHaveBeenCalled();
  });
});
