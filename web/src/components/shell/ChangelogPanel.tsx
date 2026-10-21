import { useEffect, useMemo, useRef, useSyncExternalStore } from "react";
import { useTranslation } from "react-i18next";
import { X } from "@phosphor-icons/react";
import { fetchChangelog } from "@/api/changelog";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogTitle,
} from "@/components/ui/dialog";
import { createChangelogPages, type PagesState } from "@/lib/changelogPages";
import type { ChangelogVersion } from "@/types/changelog";

/** How far from the end of the list (px) the next page is asked for. */
const NEAR_END_PX = 240;

/**
 * The changelog window (ADR 0039 step 7), opened from the version badge: a modal overlay of two
 * thirds of the viewport's width and height, so no screen's height changes. Versions newest first,
 * loaded by infinite scroll. The bullets are English as served (step 5); the chrome is translated.
 */
export function ChangelogPanel({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        data-testid="changelog-panel"
        className="h-[66.667dvh] max-h-none w-[66.667vw] max-w-none gap-3"
      >
        <div className="flex items-center gap-2">
          <DialogTitle className="section-label flex-1">
            {t("changelog.title")}
          </DialogTitle>
          <DialogClose
            render={
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={t("changelog.close")}
                title={t("changelog.close")}
              />
            }
          >
            <X aria-hidden />
          </DialogClose>
        </div>
        <ChangelogList />
      </DialogContent>
    </Dialog>
  );
}

/** The list inside the open window: it mounts with it, so every opening reads the first page afresh. */
function ChangelogList() {
  const pages = useMemo(() => createChangelogPages(fetchChangelog), []);
  const state = useSyncExternalStore(pages.subscribe, pages.getSnapshot);
  const box = useRef<HTMLDivElement>(null);

  // After every change: ask for the next page while the end is near, which also fills a box the
  // first page does not overflow. A failure waits for the Retry button instead of looping.
  const nearEnd = () => {
    const el = box.current;
    return (
      !!el && el.scrollHeight - el.scrollTop - el.clientHeight < NEAR_END_PX
    );
  };
  useEffect(() => {
    if (!state.loading && !state.failed && !state.done && nearEnd())
      void pages.loadMore();
  });

  return (
    <div
      ref={box}
      data-testid="changelog-list"
      onScroll={() => {
        if (!state.failed && nearEnd()) void pages.loadMore();
      }}
      className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto pr-1"
    >
      <Versions state={state} />
      <Status state={state} onRetry={() => void pages.loadMore()} />
    </div>
  );
}

function Versions({ state }: { state: PagesState }) {
  return (
    <>
      {state.versions.map((v) => (
        <VersionBlock key={v.version} v={v} />
      ))}
    </>
  );
}

function VersionBlock({ v }: { v: ChangelogVersion }) {
  const { t } = useTranslation();
  const none = v.features.length === 0 && v.fixes.length === 0;
  return (
    <section
      data-testid="changelog-version"
      aria-label={v.version}
      className="flex flex-col gap-2 rounded-lg bg-raised p-3"
    >
      <h3 className="text-key tabular-nums">{v.version}</h3>
      {none && (
        <p className="text-body text-ink-muted">{t("changelog.none")}</p>
      )}
      <Group title={t("changelog.features")} items={v.features} />
      <Group title={t("changelog.fixes")} items={v.fixes} />
    </section>
  );
}

function Group({ title, items }: { title: string; items: string[] }) {
  if (items.length === 0) return null;
  return (
    <div className="flex flex-col gap-1">
      <h4 className="section-label">{title}</h4>
      <ul className="list-disc space-y-1 pl-5 text-body">
        {items.map((s, i) => (
          <li key={i}>{s}</li>
        ))}
      </ul>
    </div>
  );
}

function Status({
  state,
  onRetry,
}: {
  state: PagesState;
  onRetry: () => void;
}) {
  const { t } = useTranslation();
  if (state.failed)
    return (
      <div
        role="alert"
        data-testid="changelog-error"
        className="flex flex-wrap items-center gap-2 text-body text-danger"
      >
        <span>{t("changelog.error")}</span>
        <Button onClick={onRetry}>{t("changelog.retry")}</Button>
      </div>
    );
  if (state.loading)
    return (
      <p
        role="status"
        data-testid="changelog-loading"
        className="text-body text-ink-muted"
      >
        {t("changelog.loading")}
      </p>
    );
  if (state.done && state.versions.length === 0)
    return (
      <p data-testid="changelog-empty" className="text-body text-ink-muted">
        {t("changelog.empty")}
      </p>
    );
  return null;
}
