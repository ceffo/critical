import type { Character, CharacterDetails } from "@/types/character";
import { patch, request } from "./http";

export function fetchCharacter(): Promise<Character> {
  return request<Character>("/api/character");
}

/** Only the fields present in `body` change; null clears a field (an empty age). */
export function patchCharacterDetails(
  body: Partial<CharacterDetails>,
): Promise<CharacterDetails> {
  return patch<CharacterDetails>("/api/character/details", body);
}
