import { arrangeScores } from "@/api/characters";
import { ApiError } from "@/api/http";
import { placedValues, type Placement } from "@/lib/creationFlow";
import { ABILITIES, type ScoresRoll } from "@/types/character";
import type { FlowDraft } from "./flow";

/**
 * The draft change a placement makes (ADR 0032 step 6): the server turns the placed totals and the
 * race into the scores (the method's base, the totals, PHB Table 8), which are written into the
 * score fields (they stay editable). A refusal (method VI above its max) is thrown as the server's
 * `ApiError` and changes nothing. An older server without the arrangement answers 404: the placement
 * is kept and the scores are typed.
 */
export async function arrangeFor(
  roll: ScoresRoll,
  placement: Placement,
  race: string,
  scores: FlowDraft["scores"],
): Promise<Partial<FlowDraft>> {
  try {
    const arranged = await arrangeScores(
      roll.method.code,
      race,
      placedValues(roll, placement),
    );
    const next = { ...scores };
    for (const a of ABILITIES) {
      const s = arranged.scores[a];
      if (s !== undefined) next[a] = String(s);
    }
    return { placement, arranged, scores: next };
  } catch (e) {
    if (e instanceof ApiError && (e.status === 404 || e.status === 405))
      return { placement, arranged: null };
    throw e;
  }
}
