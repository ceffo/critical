import { APP_NAME } from "@/lib/brand";
import { cn } from "@/lib/utils";

/**
 * The Critical! mark (ADR 0020 step 5): a d20 face in the primary accent with the "!" in the
 * secondary highlight; the same drawing as `public/favicon.svg`, in the current theme's colours.
 */
export function BrandIcon({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 32 32"
      aria-hidden
      className={cn("size-6 shrink-0", className)}
    >
      <polygon
        points="16,1.5 28.6,8.75 28.6,23.25 16,30.5 3.4,23.25 3.4,8.75"
        className="fill-tint"
      />
      <polygon
        points="16,7 23.8,20.5 8.2,20.5"
        className="fill-none stroke-on-accent/60"
        strokeWidth="1.6"
        strokeLinejoin="round"
      />
      <rect
        x="14.3"
        y="9.5"
        width="3.4"
        height="9"
        rx="1.7"
        className="fill-highlight"
      />
      <circle cx="16" cy="22.6" r="2" className="fill-highlight" />
    </svg>
  );
}

/** The mark and the wordmark, in the brand face; the app name is the same in every language. */
export function BrandMark({ className }: { className?: string }) {
  return (
    <div
      className={cn("flex shrink-0 items-center gap-2", className)}
      data-testid="brand"
    >
      <BrandIcon />
      <span className="font-display text-key leading-none font-bold tracking-wide select-none">
        {APP_NAME.slice(0, -1)}
        <span className="text-highlight">{APP_NAME.slice(-1)}</span>
      </span>
    </div>
  );
}
