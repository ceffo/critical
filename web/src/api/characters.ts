import type {
  CharacterList,
  CreationOptions,
  NewCharacter,
} from "@/types/character";
import { patch, post, request } from "./http";

/** The user's characters and the current one (ADR 0029 step 7.2). */
export function fetchCharacters(): Promise<CharacterList> {
  return request<CharacterList>("/api/characters");
}

/** Makes one of the user's characters current. */
export function switchCharacter(id: number): Promise<CharacterList> {
  return request<CharacterList>("/api/characters/current", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ id }),
  });
}

/** What the creation form offers. */
export function fetchCreationOptions(): Promise<CreationOptions> {
  return request<CreationOptions>("/api/characters/options");
}

/** Creates a character, which becomes current. */
export function createCharacter(body: NewCharacter): Promise<CharacterList> {
  return post<CharacterList>("/api/characters", body);
}

/** Retires a character (ADR 0029 step 7.3: hidden, never deleted); the server picks the next current one. */
export function retireCharacter(id: number): Promise<CharacterList> {
  return patch<CharacterList>(`/api/characters/${id}`, { retired: true });
}

/** Renames one of the user's characters (ADR 0029 step 7.3); the server trims and checks the name. */
export function renameCharacter(
  id: number,
  name: string,
): Promise<CharacterList> {
  return patch<CharacterList>(`/api/characters/${id}`, { name });
}
