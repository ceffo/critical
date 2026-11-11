import { afterEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { renderApp } from "@/test/render";
import { useBuffsStore } from "@/stores/buffs";
import { LongRestButton } from "./LongRestButton";

afterEach(() => useBuffsStore.setState({ round: 0 }));

const rest = () => screen.getByTestId("long-rest");

describe("LongRestButton", () => {
  it("rests at once out of combat, with no confirmation", async () => {
    useBuffsStore.setState({ round: 0 });
    const onRest = vi.fn();
    const { user } = await renderApp(
      <LongRestButton busy={false} onRest={onRest} />,
    );
    await user.click(rest());
    expect(onRest).toHaveBeenCalledTimes(1);
    expect(screen.queryByTestId("long-rest-confirm")).toBeNull();
  });

  it("asks first in a fight: Cancel and Escape send nothing and return focus, Rest sends once", async () => {
    useBuffsStore.setState({ round: 1 });
    const onRest = vi.fn();
    const { user } = await renderApp(
      <LongRestButton busy={false} onRest={onRest} />,
    );
    await user.click(rest());
    const dialog = screen.getByTestId("long-rest-confirm");
    expect(dialog).toHaveTextContent("round 1");
    expect(dialog).toHaveTextContent("spell slot");
    expect(onRest).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByTestId("long-rest-confirm")).toBeNull();
    expect(rest()).toHaveFocus();

    await user.click(rest());
    await user.keyboard("{Escape}");
    expect(screen.queryByTestId("long-rest-confirm")).toBeNull();
    expect(rest()).toHaveFocus();
    expect(onRest).not.toHaveBeenCalled();

    await user.click(rest());
    await user.click(screen.getByRole("button", { name: /^Rest$/ }));
    expect(onRest).toHaveBeenCalledTimes(1);
    expect(screen.queryByTestId("long-rest-confirm")).toBeNull();
  });
});
