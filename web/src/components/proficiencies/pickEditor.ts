import { ApiError } from "@/api/http";
import { previewProficiencies } from "@/api/proficiencies";
import type { ProficienciesInput, ProficiencySlots } from "@/types/character";
import type { Refusal } from "./refusal";

/** A pool of picks the one editor edits (ADR 0037 steps 12 and 13): the weapon picks, or the
 * non-weapon ones with the languages that spend the same pool (step 18). */
export type PickPool = "weapon" | "nonWeapon";

/** The rest, in ms, before the preview asks the server (the picks change fast). */
export const PREVIEW_DELAY = 150;

/** A refusal of the server as the editor shows it: the field and code it named, its message; a
 * failure that is not the server's refusal is named on the pool's own field. */
export function refusalOf(e: unknown, pool: PickPool): Refusal {
  const own = pool === "weapon" ? "weapon" : "nonweapon";
  if (e instanceof ApiError)
    return { field: e.field ?? own, code: e.code, message: e.message };
  return { field: own, message: e instanceof Error ? e.message : String(e) };
}

/**
 * The live preview: asks the server's figures for `input` after a short rest and reports them
 * (`figures`, the preview is no longer out of date) or the refusal (`refused`: the figures shown
 * are out of date). Returns the cancel, which drops the wait and the answer of a call made since
 * (a newer input supersedes an older one). Writes and logs nothing server-side.
 */
export function schedulePreview(
  input: ProficienciesInput,
  pool: PickPool,
  on: {
    figures: (slots: ProficiencySlots | null) => void;
    refused: (r: Refusal) => void;
  },
  ask: typeof previewProficiencies = previewProficiencies,
): () => void {
  let current = true;
  const timer = setTimeout(() => {
    ask(input).then(
      (a) => current && on.figures(a.proficiencySlots),
      (e: unknown) => current && on.refused(refusalOf(e, pool)),
    );
  }, PREVIEW_DELAY);
  return () => {
    current = false;
    clearTimeout(timer);
  };
}
