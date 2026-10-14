import type { Item, RestockResult, UseResult } from "../types/items";
import { post, request } from "./http";

export function fetchItems(): Promise<Item[]> {
  return request<Item[]>("/api/items");
}

export function postUseItem(id: string): Promise<UseResult> {
  return post<UseResult>(`/api/items/${encodeURIComponent(id)}/use`, {});
}

export function postRestockItem(id: string, count: number): Promise<RestockResult> {
  return post<RestockResult>(`/api/items/${encodeURIComponent(id)}/restock`, { count });
}
