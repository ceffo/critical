import { describe, expect, it } from "vitest";
import { isTypingTarget, isUndoShortcut, type UndoKey } from "./undoShortcut";

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
  it("not a button, the body or nothing", () => {
    expect(isTypingTarget({ tagName: "BUTTON" })).toBe(false);
    expect(isTypingTarget({ tagName: "BODY", isContentEditable: false })).toBe(false);
    expect(isTypingTarget(null)).toBe(false);
  });
});
