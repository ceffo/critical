import type {
  EquipmentCatalogue,
  Item,
  ItemBody,
  ItemChange,
  RestockResult,
  StateBody,
  StateChange,
  UseResult,
} from "../types/items";
import { del, patch, post, request } from "./http";

export function fetchItems(): Promise<Item[]> {
  return request<Item[]>("/api/items");
}

export function postUseItem(id: string): Promise<UseResult> {
  return post<UseResult>(`/api/items/${encodeURIComponent(id)}/use`, {});
}

export function postRestockItem(
  id: string,
  count: number,
): Promise<RestockResult> {
  return post<RestockResult>(`/api/items/${encodeURIComponent(id)}/restock`, {
    count,
  });
}

export function postCreateItem(body: ItemBody): Promise<ItemChange> {
  return post<ItemChange>("/api/items", body);
}

/** Only the fields present in `body` change. */
export function patchItem(id: string, body: ItemBody): Promise<ItemChange> {
  return patch<ItemChange>(`/api/items/${encodeURIComponent(id)}`, body);
}

export function deleteItem(id: string): Promise<ItemChange> {
  return del<ItemChange>(`/api/items/${encodeURIComponent(id)}`);
}

/**
 * Wear, take off, wield or put away a piece (ADR 0033 steps 6 and 7): one logged move. `swap` takes
 * off the body armour or shield worn in the way, in the same move.
 */
export function postItemState(
  id: string,
  body: StateBody,
): Promise<StateChange> {
  return post<StateChange>(`/api/items/${encodeURIComponent(id)}/state`, body);
}

/** Take a piece off the review list (ADR 0033 step 27): not a game move; answers the list. */
export function dismissReview(id: string): Promise<Item[]> {
  return del<Item[]>(`/api/items/${encodeURIComponent(id)}/review`);
}

/** The catalogue rows a piece is picked from (ADR 0033 steps 21 and 24), the same for every character. */
export function fetchEquipmentCatalogue(): Promise<EquipmentCatalogue> {
  return request<EquipmentCatalogue>("/api/equipment/catalogue");
}
