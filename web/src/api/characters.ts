import type {
  CharacterList,
  CreationDraft,
  CreationOptions,
  HpRoll,
  IdentityFigure,
  IdentityRoll,
  NewCharacter,
  ScoresArrangement,
  ScoresRoll,
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

/**
 * The options with the alignment and sex lists (ADR 0032). A release that predates them answers
 * without (a rolling deploy, ADR 0024): that is rejected as not loaded, never read as "no choice".
 */
export async function fetchCreationChoices(): Promise<CreationOptions> {
  const o = await fetchCreationOptions();
  if (
    !Array.isArray(o.sexes) ||
    o.classes.some((c) => !Array.isArray(c.alignments))
  )
    throw new Error("incomplete creation options");
  return o;
}

/** Creates a character, which becomes current. */
export function createCharacter(body: NewCharacter): Promise<CharacterList> {
  return post<CharacterList>("/api/characters", body);
}

/**
 * A creation draft as far as it is entered (ADR 0034, amendment of 2026-10-20): the sheet it would
 * read as once created, the creation's refusals, the fields still blank and the class minimums
 * missed. Never a 400 for the draft's content; writes nothing.
 */
export function draftCharacter(body: NewCharacter): Promise<CreationDraft> {
  return post<CreationDraft>("/api/characters/draft", body);
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

/** Rolls the six scores by a method (ADR 0032 step 6); writes nothing. The race adds Table 8's adjustments. */
export function rollScores(
  method: string,
  raceCode: string,
): Promise<ScoresRoll> {
  return post<ScoresRoll>("/api/characters/rolls/scores", { method, raceCode });
}

/**
 * The scores a placement gives (ADR 0032 step 6): the server adds the method's base, the totals placed
 * on each ability and the race's Table 8 adjustment, and refuses what the method cannot make (method
 * VI above its max). Writes nothing.
 */
export function arrangeScores(
  method: string,
  raceCode: string,
  placed: Record<string, number[]>,
): Promise<ScoresArrangement> {
  return post<ScoresArrangement>("/api/characters/rolls/scores/arrangement", {
    method,
    raceCode,
    placed,
  });
}

/**
 * Rolls a figure of the identity (ADR 0032 step 5.3): the table's dice for the race, and for a height
 * or a weight the sex. Writes nothing; the player may change the value.
 */
export function rollIdentity(
  raceCode: string,
  sex: string,
  figures: IdentityFigure[],
): Promise<IdentityRoll> {
  return post<IdentityRoll>("/api/characters/rolls/identity", {
    raceCode,
    sex,
    figures,
  });
}

/** Rolls the max HP of a new character (ADR 0032 step 7); writes nothing. */
export function rollHp(cls: string, con: number, xp: number): Promise<HpRoll> {
  return post<HpRoll>("/api/characters/rolls/hp", { class: cls, con, xp });
}
