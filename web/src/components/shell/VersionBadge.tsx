import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { readBuild } from "@/api/health";
import { badgeOf, type Build } from "@/lib/build";
import { cn } from "@/lib/utils";

/**
 * The running version, small and muted in the strip under every screen's bottom right corner (the
 * screens end above it: `h-app`, `min-h-app`). A "dev" or "staging" pill comes before it; production
 * shows the version alone. Read from the server once, never guessed from the host (ADR 0024 step 1.4).
 */
export function VersionBadge({ build }: { build?: Build | null }) {
  const { t } = useTranslation();
  const [read, setRead] = useState<Build | null>(null);
  useEffect(() => {
    if (build === undefined) void readBuild().then(setRead);
  }, [build]);
  const badge = badgeOf(
    (build === undefined ? read : build) ?? {
      version: "unknown",
      env: "unknown",
    },
  );
  if (!badge) return null;
  return (
    <div
      data-testid="version-badge"
      className="fixed right-3 bottom-0 z-40 flex h-app-badge items-center gap-1.5 text-label leading-3 text-ink-muted"
    >
      {badge.pill && (
        <span
          data-testid="version-pill"
          className={cn(
            "chip px-1.5 font-semibold",
            badge.pill === "staging" ? "text-warning" : "text-tint",
          )}
        >
          {t(`build.env.${badge.pill}`)}
        </span>
      )}
      <span data-testid="version-text" className="tabular-nums">
        {badge.version}
      </span>
    </div>
  );
}
