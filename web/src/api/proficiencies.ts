import type {
  ProficienciesAnswer,
  ProficienciesInput,
} from "@/types/character";
import type { LocalizedText } from "@/types/localized";
import { post, put, request } from "./http";

/** Saves the whole list of a pool's picks and the languages (ADR 0037 steps 12, 13 and 18), one move
 * the journal's Undo takes back. Answers the pools, each pick's figures and the slot warnings. */
export function setProficiencies(
  picks: ProficienciesInput,
): Promise<ProficienciesAnswer> {
  return put<ProficienciesAnswer>("/api/proficiencies", picks);
}

/** What the picks would give, as the server derives them, with the save's refusals; writes and
 * logs nothing (the editor's live preview). */
export function previewProficiencies(
  picks: ProficienciesInput,
): Promise<ProficienciesAnswer> {
  return post<ProficienciesAnswer>("/api/proficiencies/preview", picks);
}

/** What `GET /api/proficiencies/{id}/description` answers (ADR 0037 step 19.4). */
export interface ProficiencyDescription {
  id: string;
  description: LocalizedText;
}

const descriptions = new Map<string, Promise<ProficiencyDescription>>();

/**
 * A non-weapon proficiency's description in every language, read when its pop-up opens and kept for
 * the page's life: the books' text never changes under a running page. A failed read is not kept,
 * so reopening the pop-up asks again.
 */
export function getProficiencyDescription(
  id: string,
): Promise<ProficiencyDescription> {
  let held = descriptions.get(id);
  if (!held) {
    held = request<ProficiencyDescription>(
      `/api/proficiencies/${encodeURIComponent(id)}/description`,
    );
    descriptions.set(id, held);
    held.catch(() => descriptions.delete(id));
  }
  return held;
}
