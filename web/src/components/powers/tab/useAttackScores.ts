import { useEffect, useState } from "react";
import {
  previewAttackScores,
  type AttackScore,
  type PowerModifier,
} from "@/api/psionics";

/** What the rows show for the attack modes: the server's figure per power id, or why there is none. */
export type AttackScores =
  | { status: "ready"; byPower: Map<string, AttackScore> }
  | { status: "failed" }
  | { status: "loading" };

/**
 * The score to beat of every known attack mode against the defence the Target band names, in one
 * call (POST /api/psionics/attack/preview): the web computes nothing. A new defence, adjustment or
 * set of known modes asks again; an answer for an older request is dropped. Nothing is asked while
 * `enabled` is false (no attack mode known).
 */
export function useAttackScores(
  defence: string,
  modifiers: PowerModifier[],
  enabled: boolean,
  knownKey: string,
): AttackScores {
  const request = JSON.stringify({ defence, modifiers, knownKey });
  const [view, setView] = useState<{
    key: string;
    scores: AttackScores;
  } | null>(null);
  useEffect(() => {
    if (!enabled) return;
    let live = true;
    previewAttackScores(defence, modifiers).then(
      (a) =>
        live &&
        setView({
          key: request,
          scores: {
            status: "ready",
            byPower: new Map(a.attacks.map((s) => [s.power, s])),
          },
        }),
      () => live && setView({ key: request, scores: { status: "failed" } }),
    );
    return () => {
      live = false;
    };
    // The request is its JSON: a new key is a new preview.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [request, enabled]);
  return enabled && view?.key === request ? view.scores : { status: "loading" };
}
