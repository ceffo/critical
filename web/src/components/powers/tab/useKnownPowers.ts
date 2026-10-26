import { useEffect, useState } from "react";
import { getPower } from "@/api/powers";
import type { PowerPick } from "@/types/character";
import type { Power } from "@/types/powers";
import { inLearningOrder } from "./model";

/**
 * The powers a character knows, read by id from the catalogue and kept for the next render, in the
 * order they were learned. A power not read yet is left out until it arrives; one the catalogue no
 * longer has stays out.
 */
export function useKnownPowers(picks: PowerPick[]): Power[] {
  const [byId, setById] = useState<Record<string, Power>>({});
  const key = [
    ...new Set(
      picks.filter((p) => p.kind === "power" && !byId[p.id]).map((p) => p.id),
    ),
  ]
    .sort()
    .join(",");
  useEffect(() => {
    if (!key) return;
    let live = true;
    for (const id of key.split(",")) {
      getPower(id).then(
        (p) => live && setById((m) => ({ ...m, [id]: p })),
        () => undefined,
      );
    }
    return () => {
      live = false;
    };
  }, [key]);
  return inLearningOrder(picks, byId);
}
