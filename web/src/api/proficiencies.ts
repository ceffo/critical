import type { PicksInput, ProficienciesAnswer } from "@/types/character";
import { post, put } from "./http";

/** Saves the whole list of non-weapon picks (ADR 0037 steps 12 and 13), one move the journal's Undo
 * takes back. Answers the pools, each pick's figures and the slot warnings. */
export function setProficiencies(
  picks: PicksInput,
): Promise<ProficienciesAnswer> {
  return put<ProficienciesAnswer>("/api/proficiencies", picks);
}

/** What the picks would give, as the server derives them, with the save's refusals; writes and
 * logs nothing (the editor's live preview). */
export function previewProficiencies(
  picks: PicksInput,
): Promise<ProficienciesAnswer> {
  return post<ProficienciesAnswer>("/api/proficiencies/preview", picks);
}
