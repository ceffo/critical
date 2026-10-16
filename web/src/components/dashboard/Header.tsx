import { BrandMark } from "@/components/shell/BrandMark";
import { EffectsTimeline } from "./EffectsTimeline";
import { LastResult } from "./LastResult";
import { TurnBlock } from "./TurnBlock";

/**
 * The header card, full width above the character card, rail and stage (ADR 0022 step 2.2), one
 * band left to right: the brand, the turn and initiative with Next turn, the effects timeline beside
 * them (turns and effects are one concept: time), and the last result docked at the right. Hit
 * points are not here. At rest it is at most 120 px tall, its outline included (step 3.1): the turn
 * row plus up to 3 effect rows. Cells are separated by space, never a rule (step 4.1).
 */
export function Header() {
  return (
    <header
      className="zone-card flex min-w-0 shrink-0 items-center gap-6 px-4 py-2"
      data-screen="turn-header"
    >
      <BrandMark />
      <TurnBlock />
      <EffectsTimeline />
      <LastResult />
    </header>
  );
}
