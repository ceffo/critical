import type { KeyboardEvent } from "react";

/** The index an arrow key moves a radio group's choice to (ARIA radio group: right and down go
 * forward, left and up go back, both wrap), or null for any other key. */
export function nextRadio(
  key: string,
  index: number,
  count: number,
): number | null {
  if (count === 0) return null;
  if (key === "ArrowRight" || key === "ArrowDown") return (index + 1) % count;
  if (key === "ArrowLeft" || key === "ArrowUp")
    return (index - 1 + count) % count;
  return null;
}

/**
 * Keyboard behaviour of a `role="radiogroup"` of `role="radio"` buttons: one Tab stop (the chosen
 * option, else the first) and arrows that choose and focus the neighbour. Put `onKeyDown` on the
 * group and `tabIndex(value)` on each option.
 */
export function radioKeys(
  values: readonly string[],
  current: string,
  select: (value: string) => void,
) {
  const at = values.indexOf(current);
  return {
    onKeyDown: (e: KeyboardEvent<HTMLElement>) => {
      const to = nextRadio(e.key, Math.max(at, 0), values.length);
      const value = to === null ? undefined : values[to];
      if (value === undefined) return;
      e.preventDefault();
      select(value);
      const options =
        e.currentTarget.querySelectorAll<HTMLElement>('[role="radio"]');
      options[to!]?.focus();
    },
    tabIndex: (value: string) =>
      value === current || (at < 0 && value === values[0]) ? 0 : -1,
  };
}
