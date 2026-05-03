export interface HpEvent {
  id: number;
  characterId: number;
  delta: number;
  source: string;
  createdAt: string;
}

export interface HpState {
  currentHp: number;
  maxHp: number;
  events: HpEvent[];
}
