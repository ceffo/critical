import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { readBuild } from "@/api/health";
import { useNewVersion } from "@/components/shell/useNewVersion";
import { badgeOf, type Build } from "@/lib/build";
import { cn } from "@/lib/utils";

/**
 * The running version, small and muted, placed by each screen inside a box it already has (the
 * dashboard's dice footer, a screen's header card, the login card's language row): it takes no
 * height of its own. A "dev" or "staging" pill comes before it; production shows the version alone.
 * Read from the server, never guessed from the host (ADR 0024 step 1.4). When the server later reports
 * a newer version, a button beside it reloads the page (it never reloads by itself).
 */
export function VersionBadge({
  build,
  className,
}: {
  build?: Build | null;
  className?: string;
}) {
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
  const running = (build === undefined ? read : build)?.version ?? null;
  const newer = useNewVersion(running);
  if (!badge) return null;
  return (
    <div
      data-testid="version-badge"
      className={cn(
        "flex shrink-0 items-center gap-1.5 text-label text-ink-muted",
        className,
      )}
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
      {newer && (
        <button
          type="button"
          data-testid="new-version"
          title={t("build.updateHint")}
          onClick={() => window.location.reload()}
          className="chip cursor-pointer px-1.5 font-semibold text-tint hover:bg-tint/10"
        >
          {t("build.update")}
        </button>
      )}
    </div>
  );
}
