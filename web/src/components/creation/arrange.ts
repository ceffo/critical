import { arrangeScores } from "@/api/characters";
import { ApiError } from "@/api/http";
import {
  emptyPlacement,
  placedValues,
  type Placement,
} from "@/lib/creationFlow";
import { ABILITIES, type ScoresRoll } from "@/types/character";
import type { FlowDraft } from "./flow";

/**
 * The draft change a placement makes (ADR 0032 step 6): the server turns the placed totals and the
 * race into the scores (the method's base, the totals, PHB Table 8), which are written into the
 * score fields (they stay editable). The answer is the whole truth: a field the previous answer
 * filled and this one leaves out (a total moved elsewhere, a new roll) is emptied, unless the player
 * typed another value in it since. A refusal (method VI above its max) is thrown as the server's
 * `ApiError` and changes nothing. An older server without the arrangement answers 404: the placement
 * is kept and the scores are typed.
 */
export async function arrangeFor(
  roll: ScoresRoll,
  placement: Placement,
  race: string,
  draft: Pick<FlowDraft, "scores" | "arranged">,
): Promise<Partial<FlowDraft>> {
  try {
    const arranged = await arrangeScores(
      roll.method.code,
      race,
      placedValues(roll, placement),
    );
    const before = draft.arranged?.scores ?? {};
    const next = { ...draft.scores };
    for (const a of ABILITIES) {
      const s = arranged.scores[a];
      const was = before[a];
      if (s !== undefined) next[a] = String(s);
      else if (was !== undefined && next[a].trim() === String(was))
        next[a] = "";
    }
    return { placement, arranged, scores: next };
  } catch (e) {
    if (e instanceof ApiError && (e.status === 404 || e.status === 405))
      return { placement, arranged: null };
    throw e;
  }
}

/**
 * Picking a race (step 2) after a roll: the roll stays and the server adjusts its placement for the
 * new race (PHB Table 8), so the scores and the Race column follow. If it cannot, the roll is
 * dropped rather than left on the old race. A reply that lands after the draft changed again (a
 * second race picked meanwhile) is dropped, its refusal too: only the latest pick's answer counts.
 * `watch` returns a check that stays true until the draft next changes.
 */
export async function pickRace(
  draft: FlowDraft,
  race: string,
  update: (patch: Partial<FlowDraft>) => void,
  watch: () => () => boolean,
): Promise<void> {
  update({ race });
  if (draft.race === race || !draft.roll) return;
  const current = watch();
  try {
    const patch = await arrangeFor(draft.roll, draft.placement, race, draft);
    if (current()) update(patch);
  } catch {
    if (current())
      update({
        roll: null,
        placement: emptyPlacement(),
        picked: null,
        arranged: null,
      });
  }
}
