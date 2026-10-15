/** The key event fields the undo shortcut reads (a KeyboardEvent has them all). */
export interface UndoKey {
  key: string;
  ctrlKey: boolean;
  metaKey: boolean;
  altKey: boolean;
  shiftKey: boolean;
  repeat: boolean;
}

/** Ctrl+Z, or Cmd+Z on macOS (ADR 0010 step 5). Shift (the redo, isRedoShortcut), Alt and auto-repeat
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

/** Ctrl+Shift+Z, or Cmd+Shift+Z on macOS (ADR 0019 step 3): the redo. Alt and auto-repeat are not
 * a redo: one press replays one transaction. */
export function isRedoShortcut(e: UndoKey): boolean {
  return (
    (e.ctrlKey || e.metaKey) &&
    !e.altKey &&
    e.shiftKey &&
    !e.repeat &&
    e.key.toLowerCase() === "z"
  );
}

/** The focused element fields that tell whether the player is typing. */
export interface FocusTarget {
  tagName?: string;
  /** An input's type ("" or missing: text). */
  type?: string;
  isContentEditable?: boolean;
}

/** Inputs that take no typed text: Ctrl/Cmd+Z there has no field meaning, so it stays the undo. */
const NON_TEXT_INPUTS = new Set([
  "checkbox",
  "radio",
  "button",
  "submit",
  "reset",
  "range",
  "color",
  "file",
  "image",
  "hidden",
]);

/** True while the focus is in a text-entry field (text-like input, textarea, select,
 * contenteditable): Ctrl/Cmd+Z there is the field's own text undo. */
export function isTypingTarget(
  target: FocusTarget | null | undefined,
): boolean {
  if (!target) return false;
  if (target.isContentEditable === true) return true;
  switch ((target.tagName ?? "").toUpperCase()) {
    case "TEXTAREA":
    case "SELECT":
      return true;
    case "INPUT":
      return !NON_TEXT_INPUTS.has((target.type ?? "").toLowerCase());
    default:
      return false;
  }
}

/** An open popover, dialog or menu (base-ui popups are role="dialog" with data-open; a closing one
 * keeps data-closed while it animates out). */
export const OPEN_OVERLAY_SELECTOR =
  '[role="dialog"]:not([data-closed]), [role="alertdialog"], [role="menu"], dialog[open]';

/** True while a popover, dialog or menu is open: the shortcut must not pop the entry behind it. */
export function hasOpenOverlay(root: {
  querySelector: (selector: string) => unknown;
}): boolean {
  return root.querySelector(OPEN_OVERLAY_SELECTOR) != null;
}
