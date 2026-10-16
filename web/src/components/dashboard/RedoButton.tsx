import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { redoButtonState } from "@/lib/undoButton";
import { useSpellName } from "@/i18n/spellName";
import { useInventoryName } from "@/i18n/inventoryName";
import {
  hasOpenOverlay,
  isRedoShortcut,
  isTypingTarget,
} from "@/lib/undoShortcut";
import { useLogStore } from "@/stores/log";
import { useTableStore } from "@/stores/table";

/**
 * Redo (ADR 0019): replays the newest popped transaction, sending the one this client last read.
 * Ctrl/Cmd+Shift+Z does the same anywhere on the page, except while typing in a text field (the
 * key keeps its text-redo meaning there) and while a popover, dialog or menu is open.
 */
export function RedoButton() {
  const { t } = useTranslation();
  const spellName = useSpellName();
  const names = useInventoryName();
  const redoTarget = useLogStore((s) => s.redo);
  const busy = useTableStore((s) => s.busy);
  const redo = useTableStore((s) => s.redo);
  const { disabled, label } = redoButtonState(
    redoTarget,
    busy,
    t,
    spellName,
    names,
  );

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (
        !isRedoShortcut(e) ||
        isTypingTarget(document.activeElement as HTMLInputElement | null) ||
        hasOpenOverlay(document)
      )
        return;
      e.preventDefault();
      if (useTableStore.getState().busy) return;
      void useTableStore.getState().redo();
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
      aria-keyshortcuts="Control+Shift+Z Meta+Shift+Z"
      data-testid="redo"
      onClick={() => void redo()}
    >
      {t("log.redo.label")}
    </Button>
  );
}
