import { EffectsTimeline } from "./EffectsTimeline";
import { TurnBlock } from "./TurnBlock";

/**
 * Header region, full width above the rail and columns: the turn block and the effects timeline
 * (turns and effects are one concept: time), side by side in one row of at most 64px (ADR 0017
 * step 6).
 */
export function Header() {
  return (
    <header
      className="flex min-w-0 shrink-0 items-center gap-3 bg-card px-3 py-2"
      data-screen="turn-header"
    >
      <TurnBlock />
      <EffectsTimeline />
    </header>
  );
}
