import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { readBuild } from "@/api/health";
import { ChangelogPanel } from "@/components/shell/ChangelogPanel";
import { useNewVersion } from "@/components/shell/useNewVersion";
import { badgeOf, type Build } from "@/lib/build";
import { cn } from "@/lib/utils";
import { useSessionStore } from "@/stores/session";

/**
 * Whether the badge opens the changelog without a session (the login screen). The endpoint is
 * session-protected (ADR 0029 step 6.1), so the login screen's badge stays plain text until the
 * endpoint is public: this is the one place to change.
 */
const CHANGELOG_WITHOUT_SESSION = false;

/**
 * The running version, small and muted, placed by each screen inside a box it already has (the
 * dashboard's dice footer, a screen's header card, the login card's language row): it takes no
 * height of its own. A "dev" or "staging" pill comes before it; production shows the version alone.
 * Read from the server, never guessed from the host (ADR 0024 step 1.4). When the server later reports
 * a newer version, a button beside it reloads the page (it never reloads by itself). The version is a
 * button that opens the changelog panel (ADR 0039 step 7) when a session exists.
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
  const [panel, setPanel] = useState(false);
  const openable =
    useSessionStore((s) => s.me !== null) || CHANGELOG_WITHOUT_SESSION;
  if (!badge) return null;
  const label = (
    <>
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
    </>
  );
  return (
    <div
      data-testid="version-badge"
      className={cn(
        "flex shrink-0 items-center gap-1.5 text-label text-ink-muted",
        className,
      )}
    >
      {openable ? (
        <button
          type="button"
          data-testid="version-open"
          aria-haspopup="dialog"
          title={t("changelog.open")}
          onClick={() => setPanel(true)}
          className="flex cursor-pointer items-center gap-1.5 rounded-md px-1 outline-none hover:bg-raised-hover hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          {label}
        </button>
      ) : (
        label
      )}
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
      {openable && (
        <ChangelogPanel open={panel} onClose={() => setPanel(false)} />
      )}
    </div>
  );
}
