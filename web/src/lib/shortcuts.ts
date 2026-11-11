import { useEffect, type RefObject } from "react";
import { hasOpenOverlay, isTypingTarget } from "@/lib/undoShortcut";

/** The key event fields a single-key shortcut reads. */
export interface ShortcutKey {
  key: string;
  ctrlKey: boolean;
  metaKey: boolean;
  altKey: boolean;
  shiftKey: boolean;
  repeat: boolean;
  defaultPrevented: boolean;
}

/** An element that opts out of the single-key shortcuts for everything inside it (the dice composer). */
export const SHORTCUTS_OFF = "data-shortcuts-off";

/**
 * True when `e` is the plain key `key`: no Ctrl, Cmd, Alt or Shift (so the browser's and the OS's
 * Ctrl/Cmd/Alt combinations stay theirs), no auto-repeat (a held key acts once), and nobody else
 * handled it already.
 */
export function isPlainKey(e: ShortcutKey, key: string): boolean {
  return (
    !e.ctrlKey &&
    !e.metaKey &&
    !e.altKey &&
    !e.shiftKey &&
    !e.repeat &&
    !e.defaultPrevented &&
    e.key.toLowerCase() === key.toLowerCase()
  );
}

/**
 * True while a single letter belongs to the page: the focus is not in a text field, a select or
 * contenteditable, not in a block that opted out (the dice composer), and no dialog, popover or
 * menu is open.
 */
export function shortcutsLive(doc: Document): boolean {
  const active = doc.activeElement as HTMLElement | null;
  if (isTypingTarget(active)) return false;
  if (active?.closest(`[${SHORTCUTS_OFF}]`)) return false;
  return !hasOpenOverlay(doc);
}

/**
 * The plain key `key` clicks the button in `ref` when it is on screen and enabled. The click is the
 * button's own path: its handler, one server write, one Undo step. A disabled or hidden button
 * does nothing, as a click on it would (the reason is the one the button shows).
 */
export function useKeyClicks(
  key: string,
  ref: RefObject<HTMLButtonElement | null>,
) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!isPlainKey(e, key) || !shortcutsLive(document)) return;
      const button = ref.current;
      // A pane of another activity stays mounted but hidden: its button is not "on screen".
      if (!button || !button.checkVisibility()) return;
      e.preventDefault();
      if (!button.disabled) button.click();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [key, ref]);
}
