import type {
  Item,
  ItemBody,
  ItemChange,
  RestockResult,
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
