import { describe, expect, it } from "vitest";
import {
  OPEN_OVERLAY_SELECTOR,
  hasOpenOverlay,
  isTypingTarget,
  isUndoShortcut,
  type UndoKey,
} from "./undoShortcut";

const key = (k: Partial<UndoKey>): UndoKey => ({
  key: "z",
  ctrlKey: false,
  metaKey: false,
  altKey: false,
  shiftKey: false,
  repeat: false,
  ...k,
});

describe("isUndoShortcut", () => {
  it("Ctrl+Z and Cmd+Z, any case", () => {
    expect(isUndoShortcut(key({ ctrlKey: true }))).toBe(true);
    expect(isUndoShortcut(key({ metaKey: true }))).toBe(true);
    expect(isUndoShortcut(key({ ctrlKey: true, key: "Z" }))).toBe(true);
  });
  it("not a bare Z, a redo, Alt+Z or a held key", () => {
    expect(isUndoShortcut(key({}))).toBe(false);
    expect(isUndoShortcut(key({ ctrlKey: true, shiftKey: true }))).toBe(false);
    expect(isUndoShortcut(key({ ctrlKey: true, altKey: true }))).toBe(false);
    expect(isUndoShortcut(key({ ctrlKey: true, repeat: true }))).toBe(false);
    expect(isUndoShortcut(key({ ctrlKey: true, key: "y" }))).toBe(false);
  });
});

describe("isTypingTarget", () => {
  it("fields and editable elements", () => {
    for (const tagName of ["INPUT", "TEXTAREA", "SELECT", "input"])
      expect(isTypingTarget({ tagName })).toBe(true);
    expect(isTypingTarget({ tagName: "DIV", isContentEditable: true })).toBe(true);
  });
  it("text-like inputs, whatever their type", () => {
    for (const type of ["", "text", "number", "search", "email", "TEXT"])
      expect(isTypingTarget({ tagName: "INPUT", type })).toBe(true);
    expect(isTypingTarget({ tagName: "INPUT" })).toBe(true);
  });
  it("not a checkbox, radio or other input that takes no text", () => {
    for (const type of ["checkbox", "radio", "button", "submit", "range", "color"])
      expect(isTypingTarget({ tagName: "INPUT", type })).toBe(false);
  });
  it("not a button, the body or nothing", () => {
    expect(isTypingTarget({ tagName: "BUTTON" })).toBe(false);
    expect(isTypingTarget({ tagName: "BODY", isContentEditable: false })).toBe(false);
    expect(isTypingTarget(null)).toBe(false);
  });
});

describe("hasOpenOverlay", () => {
  // A fake root that answers for the selectors it knows; the real one is document.
  const root = (present: string[]) => ({
    querySelector: (selector: string) => {
      expect(selector).toBe(OPEN_OVERLAY_SELECTOR);
      return present.length > 0 ? {} : null;
    },
  });
  it("true while a popover, dialog or menu is open", () => {
    expect(hasOpenOverlay(root(["dialog"]))).toBe(true);
  });
  it("false with none", () => {
    expect(hasOpenOverlay(root([]))).toBe(false);
  });
  it("the selector covers open popovers, dialogs and menus but not closing ones", () => {
    for (const part of ['[role="dialog"]:not([data-closed])', '[role="alertdialog"]', '[role="menu"]', "dialog[open]"])
      expect(OPEN_OVERLAY_SELECTOR).toContain(part);
  });
});
