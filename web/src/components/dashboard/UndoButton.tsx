import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { undoButtonState } from "@/lib/undoButton";
import { useSpellName } from "@/i18n/spellName";
import {
  hasOpenOverlay,
  isTypingTarget,
  isUndoShortcut,
} from "@/lib/undoShortcut";
import { useLogStore } from "@/stores/log";
import { useTableStore } from "@/stores/table";

/**
 * Global undo (ADR 0010 step 5): pops the top of the server's action stack, any kind, sending the
 * top this client last read. Ctrl/Cmd+Z does the same anywhere on the page except while typing in
 * a text field (the key keeps its text-undo meaning there) and while a popover, dialog or menu is
 * open (it would pop the entry behind it).
 */
export function UndoButton() {
  const { t } = useTranslation();
  const spellName = useSpellName();
  const topId = useLogStore((s) => s.topId);
  const entries = useLogStore((s) => s.entries);
  const busy = useTableStore((s) => s.busy);
  const undo = useTableStore((s) => s.undo);
  const { disabled, label } = undoButtonState(
    topId,
    entries,
    busy,
    t,
    spellName,
  );

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (
        !isUndoShortcut(e) ||
        isTypingTarget(document.activeElement as HTMLInputElement | null) ||
        hasOpenOverlay(document)
      )
        return;
      e.preventDefault();
      if (useTableStore.getState().busy) return;
      void useTableStore.getState().undo();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

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
