import type { ReactNode } from "react";
import { BrandMark } from "@/components/shell/BrandMark";
import { VersionBadge } from "@/components/shell/VersionBadge";
import { cn } from "@/lib/utils";

/**
 * The header card every full-screen page shares (Roster, character creation, Admin, Ruleset): the
 * brand (mark and wordmark, in full at every width, ADR 0027 step 4.6) at the left, the page's
 * title, then the running version and the page's way out (`action`, a Back or Leave button) at the
 * right. The mark is `BrandMark`, never redrawn (ADR 0020). The game shell's header is its own
 * band and does not use this.
 */
export function PageHeader({
  title,
  action,
  className,
}: {
  title: ReactNode;
  /** The way out of the page: its Back (or Leave) button. */
  action?: ReactNode;
  className?: string;
}) {
  return (
    <header
      className={cn(
        "zone-card flex shrink-0 flex-wrap items-center gap-x-4 gap-y-1 px-4 py-2",
        className,
      )}
      data-testid="page-header"
    >
      <BrandMark />
      <h1 className="text-title" data-testid="page-title">
        {title}
      </h1>
      <span className="flex-1" />
      <VersionBadge />
      {action}
    </header>
  );
}
