import { useEffect, useSyncExternalStore, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { BrandIcon } from "@/components/shell/BrandMark";
import { APP_NAME } from "@/lib/brand";

/** The narrowest window the app supports, in CSS px (ADR 0009 step 3). */
export const MIN_WIDTH = 1024;

const QUERY = `(min-width: ${MIN_WIDTH}px)`;

function subscribe(onChange: () => void) {
  if (typeof window.matchMedia !== "function") return () => {};
  const list = window.matchMedia(QUERY);
  list.addEventListener("change", onChange);
  return () => list.removeEventListener("change", onChange);
}

/** True while the window is wide enough; true where the browser can't tell (tests). */
function wideEnough() {
  return typeof window.matchMedia !== "function"
    ? true
    : window.matchMedia(QUERY).matches;
}

/**
 * Below the supported width (ADR 0009 step 3) the page shows one plain message instead of the
 * layout, which is a fixed desktop one. It follows the window as it is resized, with no reload; the
 * page scrolls. Every screen but Login is wrapped in it.
 */
export function MinWidthGate({ children }: { children: ReactNode }) {
  const wide = useSyncExternalStore(subscribe, wideEnough, () => true);
  const { t } = useTranslation();
  useEffect(() => {
    if (!wide) document.title = `${t("minWidth.title")} · ${APP_NAME}`;
  }, [wide, t]);
  if (wide) return <>{children}</>;
  return (
    <main
      className="flex min-h-dvh items-center justify-center p-3"
      data-screen="min-width"
    >
      <section
        aria-labelledby="min-width-title"
        className="stage-card flex w-full max-w-[28rem] flex-col gap-4 px-6 py-6"
        data-testid="min-width-message"
      >
        <div
          className="flex items-center gap-3"
          role="img"
          aria-label={APP_NAME}
        >
          <BrandIcon className="size-12" />
          <span
            aria-hidden
            className="font-display text-hero leading-none font-bold tracking-wide select-none"
          >
            {APP_NAME.slice(0, -1)}
            <span className="text-highlight">{APP_NAME.slice(-1)}</span>
          </span>
        </div>
        <h1 id="min-width-title" className="section-label">
          {t("minWidth.title")}
        </h1>
        <p className="text-body text-ink-muted">
          {t("minWidth.body", { width: MIN_WIDTH })}
        </p>
      </section>
    </main>
  );
}
