import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { renderApp } from "@/test/render";
import { QuickHp } from "./QuickHp";

const recordHp = vi.hoisted(() => vi.fn());
vi.mock("@/stores/table", () => ({
  useTableStore: () => ({ recordHp, busy: false }),
}));

describe("HP change box", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    recordHp.mockResolvedValue(true);
  });

  it("applies damage on Enter and healing on Shift+Enter, then empties both fields", async () => {
    const { user } = await renderApp(<QuickHp />);
    const amount = screen.getByLabelText("Amount");
    const source = screen.getByLabelText("Source");

    await user.type(amount, "6");
    await user.type(source, "orc{Enter}");
    expect(recordHp).toHaveBeenLastCalledWith(-6, "orc");
    expect(amount).toHaveValue(null);
    expect(source).toHaveValue("");

    await user.type(amount, "4");
    await user.type(source, "potion{Shift>}{Enter}{/Shift}");
    expect(recordHp).toHaveBeenLastCalledWith(4, "potion");
  });

  it("keeps the buttons off until an amount is typed and sends nothing without one", async () => {
    const { user } = await renderApp(<QuickHp />);
    expect(screen.getByRole("button", { name: /Damage/ })).toBeDisabled();
    await user.type(screen.getByLabelText("Source"), "x{Enter}");
    expect(recordHp).not.toHaveBeenCalled();
    await user.type(screen.getByLabelText("Amount"), "3");
    expect(screen.getByRole("button", { name: /Damage/ })).toBeEnabled();
  });
});
