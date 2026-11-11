import { describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { renderApp } from "@/test/render";
import type { CastTarget } from "@/types/spells";
import { CastButton, castMenu } from "./SpellsPanel";

const render = (target: CastTarget | undefined, onCast = vi.fn()) =>
  renderApp(
    <CastButton name="Slow" disabled={false} target={target} onCast={onCast} />,
  );

describe("castMenu", () => {
  it("lists the default first, then the other target, from the served data", () => {
    expect(castMenu({ default: "self", alternate: "other" })).toEqual([
      "self",
      "other",
    ]);
    expect(castMenu({ default: "other", alternate: "self" })).toEqual([
      "other",
      "self",
    ]);
  });

  it("holds a single entry when nothing is served or both targets are the same", () => {
    expect(castMenu(undefined)).toEqual([]);
    expect(castMenu({ default: "self", alternate: "self" })).toEqual(["self"]);
  });
});

describe("CastButton", () => {
  it("has no arrow when the menu would hold a single entry", async () => {
    await render(undefined);
    expect(screen.getByRole("button", { name: "Cast Slow" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: /lands on/ })).toBeNull();
    await render({ default: "self", alternate: "self" });
    expect(screen.queryByRole("button", { name: /lands on/ })).toBeNull();
  });

  it("names the arrow and explains it in its tooltip, then lists both targets with the default ticked", async () => {
    const onCast = vi.fn();
    const { user } = await render(
      { default: "self", alternate: "other" },
      onCast,
    );
    const arrow = screen.getByRole("button", {
      name: "Choose who Slow lands on",
    });
    expect(arrow.getAttribute("title")).toBe(
      "Cast Slow on me or on an ally: the tick marks the default",
    );
    await user.click(arrow);
    const items = screen.getAllByRole("menuitemradio");
    expect(items.map((i) => i.textContent)).toEqual(["On me", "On an ally"]);
    expect(items.map((i) => i.getAttribute("aria-checked"))).toEqual([
      "true",
      "false",
    ]);
    await user.click(items[1]);
    expect(onCast).toHaveBeenCalledWith(false);
  });

  it("opens from the keyboard and casts the chosen target", async () => {
    const onCast = vi.fn();
    const { user } = await render(
      { default: "other", alternate: "self" },
      onCast,
    );
    screen.getByRole("button", { name: "Choose who Slow lands on" }).focus();
    await user.keyboard("{Enter}");
    await user.keyboard("{ArrowDown}{Enter}");
    expect(onCast).toHaveBeenCalledWith(true);
  });
});
