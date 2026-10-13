// Mirrors api/internal/rest/handler/itemhdl (ItemDTO).

export interface Item {
  id: string;
  name: string;
  kind: "ammo" | "charged" | "consumable";
  magic_bonus: number;
  /** Launcher kind the ammo feeds ("arrow"). */
  ammo_kind: string;
  count: number;
  max_count: number;
  per_day: boolean;
  effect_dice: string;
  effect_save: string;
  notes: string;
}

/** POST /api/items/{id}/use (itemhdl UseResultDTO); the item_use log payload has the same shape. */
export interface UseResult {
  itemId: string;
  name: string;
  kind: "charged" | "consumable";
  remaining: number;
  maxCount: number;
  /** Effect dice ("6d6"); empty when the item rolls nothing. */
  dice: string;
  rolls: number[];
  total: number;
  /** Save category the effect allows; empty when none. */
  save: string;
  notes: string;
  breakdown: string;
  logEntryId?: number;
}
