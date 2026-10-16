import {
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent,
  type PointerEvent,
} from "react";
import { useTranslation } from "react-i18next";
import { ActionArea } from "@/components/actions/ActionArea";
import { CombatLog } from "@/components/dashboard/CombatLog";
import { JournalBar } from "@/components/dashboard/JournalBar";
import { ResultCard } from "@/components/dashboard/ResultCard";
import {
  clampJournalHeight,
  JOURNAL_MIN_PX,
  SPLIT_STEP_PX,
} from "@/lib/journalSplit";
import { useSettingsStore } from "@/stores/settings";

/** The handle's row: 24 px (h-6). */
const HANDLE_PX = 24;
/** A press that moves less than this (px) is a click, which toggles; more is a drag. */
const CLICK_SLOP_PX = 4;

/** The stage's height, following resizes. */
function useHeight(ref: React.RefObject<HTMLDivElement | null>): number {
  const [height, setHeight] = useState(0);
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const measure = () => setHeight(el.clientHeight);
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, [ref]);
  return height;
}

/**
 * The Act activity's stage (ADR 0022 step 2.5): the "now" band (the act panel beside the result
 * panel, act : result = minmax(28rem, 1.35fr) | minmax(24rem, 1fr)), then the split handle, then the
 * journal. By default the band takes its content's height up to 60 % of the stage and the journal
 * (at least 14rem) the rest. Dragging the handle sets the journal's height, a click on it
 * folds the journal into a one-line bar and back (step 5.4). Height and collapsed state are this device's display
 * preferences (stores/settings.ts), never sent to the API.
 */
export function ActStage({ onOpenInventory }: { onOpenInventory: () => void }) {
  const { t } = useTranslation();
  const journalHeight = useSettingsStore((s) => s.journalHeight);
  const collapsed = useSettingsStore((s) => s.journalCollapsed);
  const update = useSettingsStore((s) => s.update);
  const stageRef = useRef<HTMLDivElement>(null);
  const journalRef = useRef<HTMLDivElement>(null);
  const stage = useHeight(stageRef);
  // The height while dragging; saved once the pointer is released.
  const [dragging, setDragging] = useState<number | null>(null);

  const wanted = dragging ?? journalHeight;
  const height =
    wanted === null || stage === 0
      ? null
      : clampJournalHeight(wanted, stage, HANDLE_PX);
  const shownHeight = (): number =>
    height ??
    journalRef.current?.getBoundingClientRect().height ??
    JOURNAL_MIN_PX;

  const onPointerDown = (ev: PointerEvent<HTMLDivElement>) => {
    if (ev.button !== 0) return;
    ev.preventDefault();
    const handle = ev.currentTarget;
    handle.setPointerCapture(ev.pointerId);
    const startY = ev.clientY;
    const start = collapsed ? JOURNAL_MIN_PX : shownHeight();
    let last = start;
    let dragged = false;
    const move = (e: globalThis.PointerEvent) => {
      if (!dragged && Math.abs(e.clientY - startY) < CLICK_SLOP_PX) return;
      dragged = true;
      last = clampJournalHeight(start - (e.clientY - startY), stage, HANDLE_PX);
      setDragging(last);
    };
    const up = () => {
      handle.removeEventListener("pointermove", move);
      handle.removeEventListener("pointerup", up);
      handle.removeEventListener("pointercancel", up);
      setDragging(null);
      if (dragged) update({ journalHeight: last, journalCollapsed: false });
      else update({ journalCollapsed: !collapsed });
    };
    handle.addEventListener("pointermove", move);
    handle.addEventListener("pointerup", up);
    handle.addEventListener("pointercancel", up);
  };

  const onKeyDown = (ev: KeyboardEvent<HTMLDivElement>) => {
    if (ev.key === "Enter" || ev.key === " ") {
      ev.preventDefault();
      update({ journalCollapsed: !collapsed });
      return;
    }
    const step =
      ev.key === "ArrowUp"
        ? SPLIT_STEP_PX
        : ev.key === "ArrowDown"
          ? -SPLIT_STEP_PX
          : 0;
    if (step === 0) return;
    ev.preventDefault();
    const from = collapsed ? JOURNAL_MIN_PX : shownHeight();
    update({
      journalHeight: clampJournalHeight(from + step, stage, HANDLE_PX),
      journalCollapsed: false,
    });
  };

  // Default: the band at its content's height up to 60 %, the journal the rest (at least 14rem).
  // Dragged or collapsed: the journal (or its bar) has its height, the band the rest.
  const band: CSSProperties =
    collapsed || height !== null
      ? { flex: "1 1 0" }
      : { flex: "0 1 auto", maxHeight: "60%" };
  const journal: CSSProperties = collapsed
    ? { flex: "0 0 auto" }
    : height !== null
      ? { flex: `0 0 ${height}px` }
      : { flex: `1 0 ${JOURNAL_MIN_PX}px` };

  return (
    <div
      ref={stageRef}
      className="flex min-h-0 min-w-0 flex-1 flex-col"
      data-testid="act-stage"
    >
      <div
        className="grid min-h-0 grid-cols-[minmax(28rem,1.35fr)_minmax(24rem,1fr)] grid-rows-[minmax(0,1fr)] gap-3"
        style={band}
        data-testid="now-band"
      >
        <ActionArea onOpenInventory={onOpenInventory} />
        <ResultCard />
      </div>
      <div className="flex h-6 shrink-0 items-center justify-center">
        <div
          role="separator"
          aria-orientation="horizontal"
          aria-label={t("shell.journal.resize")}
          aria-valuemin={JOURNAL_MIN_PX}
          aria-valuemax={Math.max(JOURNAL_MIN_PX, stage - HANDLE_PX)}
          aria-valuenow={collapsed || height === null ? undefined : height}
          aria-valuetext={t(
            collapsed
              ? "shell.journal.stateCollapsed"
              : "shell.journal.stateExpanded",
          )}
          tabIndex={0}
          title={t("shell.journal.handleHint")}
          className="group flex h-full w-24 cursor-row-resize touch-none items-center justify-center rounded-md outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
          data-testid="journal-handle"
          onPointerDown={onPointerDown}
          onKeyDown={onKeyDown}
        >
          <span className="h-1 w-16 rounded-full bg-neu-700 transition-colors group-hover:bg-tint" />
        </div>
      </div>
      <div
        ref={journalRef}
        className="flex min-h-0 min-w-0 flex-col"
        style={journal}
        data-testid="journal-slot"
      >
        {collapsed ? <JournalBar /> : <CombatLog />}
      </div>
    </div>
  );
}
