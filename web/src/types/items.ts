// Mirrors api/internal/rest/handler/itemhdl (ItemDTO, UseResultDTO, RestockResultDTO).

/**
 * How an item's count behaves (critical-qki.13): stock is used and restocked (stays at 0),
 * charges is used only (gone at 0), daily is used only and refilled by a long rest.
 */
export type ItemBehaviour = "stock" | "charges" | "daily";

export interface Item {
  id: string;
  name: string;
  /** Free-text label: arrow, flask, wand, gem… */
  type: string;
  behaviour: ItemBehaviour;
  /** Magical bonus (+N): to hit and damage when fired as ammo. */
  magicalBonus: number;
  /** Launcher kind the item feeds ("arrow"); empty when it is not ammunition. */
  ammoKind: string;
  count: number;
  /** A daily item's refill value. */
  maxCount: number;
  effectDice: string;
  effectSave: string;
  notes: string;
}

/** POST /api/items/{id}/use (itemhdl UseResultDTO); the item_use log payload has the same shape. */
export interface UseResult {
  itemId: string;
  name: string;
  /** Absent in entries logged before qki.13. */
  type?: string;
  behaviour?: ItemBehaviour;
  /** The use spent a charges item's last charge: the item is gone until the use is popped. */
  removed?: boolean;
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

/** POST /api/items/{id}/restock (itemhdl RestockResultDTO); the item_restock log payload has the same shape. */
export interface RestockResult {
  itemId: string;
  name: string;
  type: string;
  added: number;
  count: number;
  logEntryId?: number;
}
