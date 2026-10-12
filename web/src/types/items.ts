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
