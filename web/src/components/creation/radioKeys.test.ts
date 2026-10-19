import { describe, expect, it, vi } from "vitest";
import { nextRadio, radioKeys } from "./radioKeys";

describe("radio group keys", () => {
  it("moves forward on right and down, back on left and up, wrapping", () => {
    expect(nextRadio("ArrowRight", 0, 3)).toBe(1);
    expect(nextRadio("ArrowDown", 2, 3)).toBe(0);
    expect(nextRadio("ArrowLeft", 0, 3)).toBe(2);
    expect(nextRadio("ArrowUp", 2, 3)).toBe(1);
    expect(nextRadio("Enter", 0, 3)).toBeNull();
    expect(nextRadio("ArrowRight", 0, 0)).toBeNull();
  });

  it("has one Tab stop: the chosen option, else the first", () => {
    const chosen = radioKeys(["a", "b", "c"], "b", vi.fn());
    expect(["a", "b", "c"].map(chosen.tabIndex)).toEqual([-1, 0, -1]);
    const none = radioKeys(["a", "b", "c"], "", vi.fn());
    expect(["a", "b", "c"].map(none.tabIndex)).toEqual([0, -1, -1]);
  });

  it("chooses the neighbour on an arrow key and ignores other keys", () => {
    const select = vi.fn();
    const keys = radioKeys(["a", "b", "c"], "b", select);
    const press = (key: string) => {
      const preventDefault = vi.fn();
      keys.onKeyDown({
        key,
        preventDefault,
        currentTarget: { querySelectorAll: () => [] },
      } as never);
      return preventDefault;
    };
    expect(press("ArrowRight")).toHaveBeenCalled();
    expect(select).toHaveBeenLastCalledWith("c");
    press("ArrowLeft");
    expect(select).toHaveBeenLastCalledWith("a");
    select.mockClear();
    expect(press("Tab")).not.toHaveBeenCalled();
    expect(select).not.toHaveBeenCalled();
  });
});
