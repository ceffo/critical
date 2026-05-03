export interface BuffEffect {
  id: string;
  name: string;
  type: "charge" | "duration";
  charges: number;
  duration: number;
  effects: Record<string, number>;
}

export interface RoundAdvanceResponse {
  roundNumber: number;
  activeBuffs: BuffEffect[];
  expiredBuffs: BuffEffect[];
}
