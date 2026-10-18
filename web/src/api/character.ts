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
 * Roll the hit points of `level`, the lowest pending one (ADR 0030 step 7), on the server; `die` is
 * the manual entry, the die rolled at the table.
 */
export function rollLevelUp(
  level: number,
  die?: number,
): Promise<LevelUpResult> {
  return post<LevelUpResult>(
    "/api/character/levelup/roll",
    die === undefined ? { level } : { level, die },
  );
}

/** Turn the undead of a PHB Table 61 row (ADR 0030 step 16): the server reads the cell at the
 * realized level and rolls; a refusal (a dash, a class that cannot turn) is an ApiError. */
export function turnUndead(undead: string): Promise<TurnResult> {
  return post<TurnResult>("/api/character/turn-undead", { undead });
}
