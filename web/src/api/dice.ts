import type { DiceFavourite, DiceFavourites, DiceRoll } from "@/types/dice";
import { del, post, put, request } from "./http";

/** Rolls the formula on the server; it is logged (dice). A bad formula is an ApiError (400). */
export function rollDice(formula: string): Promise<DiceRoll> {
  return post<DiceRoll>("/api/roll/dice", { formula });
}

export function fetchFavourites(): Promise<DiceFavourites> {
  return request<DiceFavourites>("/api/dice/favourites");
}

export function addFavourite(formula: string): Promise<DiceFavourite> {
  return post<DiceFavourite>("/api/dice/favourites", { formula });
}

/** Resolves the favourites left. */
export function deleteFavourite(id: number): Promise<DiceFavourites> {
  return del<DiceFavourites>(`/api/dice/favourites/${id}`);
}

/** Sets the whole order from the favourites' ids; resolves the favourites in that order. */
export function reorderFavourites(ids: number[]): Promise<DiceFavourites> {
  return put<DiceFavourites>("/api/dice/favourites/order", { ids });
}
