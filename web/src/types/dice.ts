// Mirrors api/internal/rest/handler/dicehdl (RollDTO, GroupDTO, FavouriteDTO).

/** The dice of one kind in a roll; subtotal is signed (negative for a subtracted group). */
export interface DiceGroup {
  sides: number;
  rolls: number[];
  negative: boolean;
  subtotal: number;
}

/** A free roll as POST /api/roll/dice returns it, and the payload of a dice entry (without
 * logEntryId). formula is the server's normal form of what was sent. */
export interface DiceRoll {
  formula: string;
  groups: DiceGroup[];
  modifier: number;
  total: number;
  logEntryId?: number;
}

export interface DiceFavourite {
  id: number;
  formula: string;
}

export interface DiceFavourites {
  favourites: DiceFavourite[];
}
