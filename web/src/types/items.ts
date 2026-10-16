// Mirrors api/internal/rest/handler/itemhdl (ItemDTO, PowerDTO, UseResultDTO, RestockResultDTO, ChangeDTO,
// ItemBody).

import type { BuffEffects } from "./buffs";
import type { LocalizedText } from "./localized";

/**
 * How an item's count behaves: stock is used and restocked (stays at 0),
 * charges is used only (gone at 0), daily is used only and refilled by a long rest.
 */
export type ItemBehaviour = "stock" | "charges" | "daily";

export interface Item {
  id: string;
  /** A language map (ADR 0023 step 9): pick with `localized`. */
  name: LocalizedText;
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
  /**
   * The item's own powers (ADR 0016 steps 9 to 16), read only: never sent by the editor. Whether one
   * is on is served with the effects list (GET /api/buffs).
   */
  powers: ItemPowerDef[];
}

/** One of an item's powers: its trigger (minStr; absent: always on) and its effects. */
export interface ItemPowerDef {
  id: string;
  minStr?: number;
  effects: BuffEffects;
}

/** POST /api/items/{id}/use (itemhdl UseResultDTO); the item_use log payload has the same shape. */
export interface UseResult {
  itemId: string;
  name: string;
  /** Absent in entries logged before the item type was recorded. */
  type?: string;
  behaviour?: ItemBehaviour;
  /** The use spent a charges item's last charge: the item is gone until the use is popped. */
  removed?: boolean;
  remaining: number;
  maxCount: number;
  /** Effect dice ("6d6"); empty when the item rolls nothing. */
  dice: string;
  rolls: number[];
  /** The flat "+K" of the dice ("1d8+2"); absent on uses logged before it was recorded. */
  modifier?: number;
  total: number;
  /** Save category the effect allows; empty when none. */
  save: string;
  notes: string;
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

/** Body of POST /api/items and PATCH /api/items/{id} (itemhdl ItemBody); a PATCH changes only the fields present. */
export interface ItemBody {
  /** The languages typed (ADR 0023 step 11): the others are kept. */
  name?: LocalizedText;
  type?: string;
  behaviour?: ItemBehaviour;
  count?: number;
  /** The daily number (daily items only). */
  maxCount?: number;
  magicalBonus?: number;
  ammoKind?: string;
  effectDice?: string;
  effectSave?: string;
  notes?: string;
}

/** POST/PATCH/DELETE /api/items (itemhdl ChangeDTO): the item as it is now (as it was, for a delete). */
export interface ItemChange {
  item: Item;
  logEntryId: number;
}

/** One edited field in an item_edit log payload (values as text). */
export interface FieldChange {
  field: string;
  before: string;
  after: string;
}

/** Payload of item_add, item_edit and item_remove log entries (itemsvc changeRecord). */
export interface ItemChangeRecord {
  itemId: string;
  name: string;
  type: string;
  behaviour: ItemBehaviour;
  count: number;
  maxCount: number;
  magicalBonus: number;
  ammoKind: string;
  effectDice: string;
  effectSave: string;
  notes: string;
  /** Only in item_edit. */
  changes?: FieldChange[];
}
