export interface HpEvent {
  id: number;
  characterId: number;
  delta: number;
  source: string;
  createdAt: string;
}

/** The character's condition from its HP, decided by the server (ADR 0011 step 2). */
export type HpCondition = "ok" | "badlyWounded" | "unconscious" | "dead";

export interface HpState {
  currentHp: number;
  maxHp: number;
  /** Lowest HP the server allows (ADR 0011 step 1: -10). */
  floor: number;
  condition: HpCondition;
  events: HpEvent[];
}
