import { EffectsTimeline } from "./EffectsTimeline";
import { TurnBlock } from "./TurnBlock";

/**
 * Header region, full width above the rail and columns: the turn block and the effects timeline
 * (turns and effects are one concept: time).
 */
export function Header() {
  return (
    <header className="flex min-w-0 shrink-0 items-stretch gap-4 bg-card px-4 py-3" data-screen="turn-header">
      <TurnBlock />
      <EffectsTimeline />
    </header>
  );
}
