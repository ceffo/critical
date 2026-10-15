/** The key event fields the undo shortcut reads (a KeyboardEvent has them all). */
export interface UndoKey {
  key: string;
  ctrlKey: boolean;
  metaKey: boolean;
  altKey: boolean;
  shiftKey: boolean;
  repeat: boolean;
}

/** Ctrl+Z, or Cmd+Z on macOS (ADR 0010 step 5). Shift (redo elsewhere), Alt and auto-repeat
 * from a held key are not an undo: one press pops one entry. */
export function isUndoShortcut(e: UndoKey): boolean {
  return (
    (e.ctrlKey || e.metaKey) &&
    !e.altKey &&
    !e.shiftKey &&
    !e.repeat &&
    e.key.toLowerCase() === "z"
  );
}

/** The focused element fields that tell whether the player is typing. */
export interface FocusTarget {
  tagName?: string;
  isContentEditable?: boolean;
}

const TYPING_TAGS = new Set(["INPUT", "TEXTAREA", "SELECT"]);

/** True while the focus is in a field: Ctrl/Cmd+Z there is the field's own text undo. */
export function isTypingTarget(target: FocusTarget | null | undefined): boolean {
  if (!target) return false;
  return (
    TYPING_TAGS.has((target.tagName ?? "").toUpperCase()) ||
    target.isContentEditable === true
  );
}
