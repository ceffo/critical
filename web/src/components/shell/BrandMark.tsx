import { APP_NAME } from "@/lib/brand";
import { cn } from "@/lib/utils";

/**
 * The Critical! mark, Facet · Bare (ADR 0022 step 7): a hexagon outline in the primary accent with
 * the "!" in the secondary highlight centred on its axis, no front face. The same drawing as
 * `public/favicon.svg` (docs/design/ui-directions/logo/facet-bare.svg), in the current theme's
 * colours.
 */
export function BrandIcon({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 32 32"
      aria-hidden
      className={cn("size-7 shrink-0", className)}
      data-testid="brand-icon"
    >
      <polygon
        points="16,2.5 27.7,9.25 27.7,22.75 16,29.5 4.3,22.75 4.3,9.25"
        className="fill-none stroke-tint"
        strokeWidth="2"
        strokeLinejoin="round"
      />
      <rect
        x="14.5"
        y="8.5"
        width="3"
        height="9.5"
        rx="1.5"
        className="fill-highlight"
      />
      <circle cx="16" cy="21.9" r="1.8" className="fill-highlight" />
    </svg>
  );
}

/**
 * The mark and the Cinzel wordmark (ADR 0020 step 5); the app name is the same in every language.
 * Both show at every layout width (ADR 0027 step 4.6): the brand has its own line at the top of the
 * header's right column, over the last result.
 */
export function BrandMark({ className }: { className?: string }) {
  return (
    <div
      className={cn("flex shrink-0 items-center gap-2", className)}
      data-testid="brand"
      aria-label={APP_NAME}
      role="img"
    >
      <BrandIcon />
      <span
        aria-hidden
        className="font-display text-key leading-none font-bold tracking-wide select-none"
        data-testid="wordmark"
      >
        {APP_NAME.slice(0, -1)}
        <span className="text-highlight">{APP_NAME.slice(-1)}</span>
      </span>
    </div>
  );
}
