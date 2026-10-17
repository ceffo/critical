/**
 * Shared classes for native inputs/selects so they match ui/input (28px, ADR 0017 step 5) and never
 * overflow their cell. Fields sit on the card with the field edge (ADR 0026 step 5.7), never a
 * darker "hole".
 */
export const fieldClass =
  "h-7 w-full min-w-0 rounded-lg border border-input bg-surface px-2 text-body outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:text-ink-muted";

/** Sentence-case label for fields and stat names (ADR 0017 `text-label`, 11px). */
export const labelClass = "text-label font-medium text-muted-foreground";

/**
 * A group's head inside the character card (ADR 0026 step 6.3): its section label (and what sits
 * beside it) on the sub-band tone, run to the card's edges over its 16 px side padding.
 */
export const groupHeadClass = "group-band bleed-x-4 py-1";
