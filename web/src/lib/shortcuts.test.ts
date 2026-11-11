import { describe, expect, it } from "vitest";
import { isPlainKey, type ShortcutKey } from "./shortcuts";

const press = (over: Partial<ShortcutKey> = {}): ShortcutKey => ({
  key: "n",
  ctrlKey: false,
  metaKey: false,
  altKey: false,
  shiftKey: false,
  repeat: false,
  defaultPrevented: false,
  ...over,
});

describe("isPlainKey", () => {
  it("takes the plain letter in either case", () => {
    expect(isPlainKey(press(), "n")).toBe(true);
    expect(isPlainKey(press({ key: "N" }), "n")).toBe(true);
  });
  it.each([
    ["Ctrl", { ctrlKey: true }],
    ["Cmd", { metaKey: true }],
    ["Alt", { altKey: true }],
    ["Shift", { shiftKey: true }],
    ["a held key", { repeat: true }],
    ["a handled key", { defaultPrevented: true }],
    ["another key", { key: "m" }],
  ])("refuses %s", (_name, over) => {
    expect(isPlainKey(press(over), "n")).toBe(false);
  });
});
