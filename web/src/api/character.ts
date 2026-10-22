import type {
  Character,
  CharacterDetails,
  LevelUpResult,
  TurnResult,
} from "@/types/character";
import { patch, post, request } from "./http";

export function fetchCharacter(): Promise<Character> {
  return request<Character>("/api/character");
}

/** Only the fields present in `body` change; null clears a field (an empty age). */
export function patchCharacterDetails(
  body: Partial<CharacterDetails>,
): Promise<CharacterDetails> {
  return patch<CharacterDetails>("/api/character/details", body);
}

/**
 * Roll the hit points of `level`, the lowest pending one of the class entry `cls` (ADR 0030 step 7,
 * ADR 0042 step 7; absent: the first active entry), on the server; `die` is the manual entry, the die
 * rolled at the table.
 */
export function rollLevelUp(
  level: number,
  die?: number,
  cls?: string,
): Promise<LevelUpResult> {
  return post<LevelUpResult>("/api/character/levelup/roll", {
    level,
    ...(die === undefined ? {} : { die }),
    ...(cls ? { class: cls } : {}),
  });
}

/** Turn the undead of a PHB Table 61 row (ADR 0030 step 16): the server reads the cell at the
 * realized level and rolls; a refusal (a dash, a class that cannot turn) is an ApiError. */
export function turnUndead(undead: string): Promise<TurnResult> {
  return post<TurnResult>("/api/character/turn-undead", { undead });
}

/** What a class switch did (ADR 0042 step 3): the class left, frozen at `leftAtLevel` with `xp`, and
 * the class taken, at level 1. */
export interface DualClassResult {
  from: string;
  to: string;
  leftAtLevel: number;
  xp: number;
}

/** The character takes `cls` as a new class and its old one is frozen (ADR 0042 step 3). A refusal
 * is an ApiError whose message is the code's wording. */
export function takeNewClass(cls: string): Promise<DualClassResult> {
  return post<DualClassResult>("/api/character/dual-class", { class: cls });
}
