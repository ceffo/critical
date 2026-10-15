import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { entryText } from "@/lib/logSummary";
import { isTypingTarget, isUndoShortcut } from "@/lib/undoShortcut";
import { useLogStore } from "@/stores/log";
import { useTableStore } from "@/stores/table";

/**
 * Global undo (ADR 0010 step 5): pops the top of the server's action stack, any kind, sending the
 * top this client last read. Ctrl/Cmd+Z does the same anywhere on the page except while typing in
 * a field, where the key keeps its text-undo meaning.
 */
export function UndoButton() {
  const { t } = useTranslation();
  const topId = useLogStore((s) => s.topId);
  const top = useLogStore((s) => s.entries.find((e) => e.id === s.topId));
  const busy = useTableStore((s) => s.busy);
  const undo = useTableStore((s) => s.undo);
  const disabled = topId === null || busy;

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!isUndoShortcut(e) || isTypingTarget(document.activeElement as HTMLElement | null))
        return;
      e.preventDefault();
      if (useTableStore.getState().busy) return;
      void useTableStore.getState().undo();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const label =
    topId === null
      ? t("log.undo.nothing")
      : t("log.undo.tooltip", { action: top ? entryText(top, t) : `#${topId}` });

  return (
    <Button
      variant="outline"
      size="sm"
      disabled={disabled}
      title={label}
      aria-label={label}
      aria-keyshortcuts="Control+Z Meta+Z"
      data-testid="undo"
      onClick={() => void undo()}
    >
      {t("log.undo.label")}
    </Button>
  );
}
