export interface BuffEffect {
  id: string;
  name: string;
  type: "charge" | "duration";
  charges: number;
  duration: number;
  effects: Record<string, number>;
}

/** GET /api/round */
export interface RoundResponse {
  round: number;
}

/** POST /api/round/advance */
export interface RoundAdvanceResponse {
  roundNumber: number;
  activeBuffs: BuffEffect[];
  expiredBuffs: BuffEffect[];
}
