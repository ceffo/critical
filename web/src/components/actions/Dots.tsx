import { cn } from "@/lib/utils";
import type { DotState } from "@/lib/slotDots";

/**
 * The fill of a slot dot (ADR 0027 steps 10.3 and 11.5): magic while castable (or prepared), solid
 * muted ink once used, an outline when nothing is prepared in it, danger for a spell prepared beyond
 * the level's limit.
 */
const DOT_CLASS: Record<DotState, string> = {
  available: "bg-magic",
  used: "bg-ink-muted",
  empty: "border border-input",
  over: "bg-danger",
};

/**
 * A row of dots, shared by the Spells tab and the preparation editor. The count is in `label`
 * (`aria-label` and tooltip); `data-state` spells the dots out (a: available, u: used, e: empty,
 * o: over).
 */
export function Dots({
  dots,
  label,
  testId,
}: {
  dots: DotState[];
  label: string;
  testId: string;
}) {
  return (
    <span
      role="img"
      className="flex flex-wrap items-center gap-1"
      aria-label={label}
      title={label}
      data-testid={testId}
      data-state={dots.map((d) => d[0]).join("")}
    >
      {dots.map((d, i) => (
        <span key={i} className={cn("size-2.5 rounded-full", DOT_CLASS[d])} />
      ))}
    </span>
  );
}
