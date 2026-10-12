import type { Character } from "@/types/character";
import { request } from "./http";

export function fetchCharacter(): Promise<Character> {
  return request<Character>("/api/character");
}
