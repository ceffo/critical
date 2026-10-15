import { BrandMark } from "@/components/shell/BrandMark";
import { EffectsTimeline } from "./EffectsTimeline";
import { TurnBlock } from "./TurnBlock";

/**
 * Header region, full width above the rail and columns: the brand mark, then the turn block and
 * the effects timeline (turns and effects are one concept: time), side by side. At rest it is the turn row plus up to
 * 3 effect rows, at most 112px (ADR 0017 step 6).
 */
export function Header() {
  return (
    <header
      className="flex min-w-0 shrink-0 items-center gap-3 bg-card px-3 py-2"
      data-screen="turn-header"
    >
      <BrandMark className="pr-3" />
      <TurnBlock />
      <EffectsTimeline />
    </header>
  );
}
