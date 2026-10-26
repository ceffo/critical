import type { PowerPick } from "@/types/character";

/**
 * The edits of a pick list, each one a whole new list for `onChange` (ADR 0043 step 9). Removing a
 * power takes its relearns with it (a relearn of a power not known is malformed); removing a
 * relearn takes that one only, the first of its power.
 */
export function usePickEdits(
  picks: PowerPick[],
  busy: boolean,
  onChange: (next: PowerPick[]) => void | Promise<void>,
) {
  const change = (next: PowerPick[]) => {
    if (!busy) void onChange(next);
  };
  return {
    isPicked: (kind: PowerPick["kind"], id: string) =>
      picks.some((p) => p.kind === kind && p.id === id),
    add: (pick: PowerPick) => change([...picks, pick]),
    remove: (pick: PowerPick) => {
      if (pick.kind === "relearn") {
        const at = picks.findIndex(
          (p) => p.kind === "relearn" && p.id === pick.id,
        );
        change(picks.filter((_, i) => i !== at));
        return;
      }
      change(
        picks.filter(
          (p) =>
            !(
              p.id === pick.id &&
              (p.kind === pick.kind || p.kind === "relearn")
            ),
        ),
      );
    },
  };
}
