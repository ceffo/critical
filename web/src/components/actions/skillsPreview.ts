import { ApiError } from "@/api/http";
import { previewSkillPoints } from "@/api/skills";
import { parseWhole } from "@/lib/wholeNumber";
import type { RogueSkill, Skills } from "@/types/skills";

/** The points typed in the editor as the server takes them (zeros left out); null while a field is
 * not a whole number. Parsing only: the server checks the ranges. */
export function typedPoints(
  values: Record<string, string>,
  skills: RogueSkill[],
): Record<string, number> | null {
  const points: Record<string, number> = {};
  for (const s of skills) {
    const v = parseWhole(values[s.id] ?? "") ?? 0;
    if (Number.isNaN(v)) return null;
    if (v !== 0) points[s.id] = v;
  }
  return points;
}

/** The class skills of an answer, a thief's or a bard's. */
export function classSkillsOf(s: Skills): RogueSkill[] {
  return s.thiefSkills.length > 0 ? s.thiefSkills : s.bardSkills;
}

/** The answer of a preview: the derived skills, or why none came (a refused field of the typed
 * points, or no answer at all). */
export type PreviewResult =
  | { ok: true; skills: Skills }
  | { ok: false; field?: string; message?: string };

/**
 * What the server derives from the points typed so far (ADR 0030 steps 17 and 18): the skills with
 * their terms and the budget. Nothing is written or logged. A field that is not a whole number, a
 * refusal (its field is named) or a failed call is `ok: false`: the editor then marks what it
 * shows as out of date.
 */
export async function previewDraft(
  values: Record<string, string>,
  skills: RogueSkill[],
  fetchPreview: typeof previewSkillPoints = previewSkillPoints,
): Promise<PreviewResult> {
  const points = typedPoints(values, skills);
  if (!points) return { ok: false };
  try {
    return { ok: true, skills: await fetchPreview(points) };
  } catch (e) {
    return e instanceof ApiError
      ? { ok: false, field: e.field, message: e.message }
      : { ok: false };
  }
}
