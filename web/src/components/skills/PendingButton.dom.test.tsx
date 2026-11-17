import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PendingButton } from "./PendingButton";

const show = (pending: boolean, onClick = vi.fn()) =>
  render(
    <PendingButton
      pending={pending}
      label="Edit: Weapons"
      pendingLabel="Something is left"
      onClick={onClick}
      testId="b"
    >
      {"Edit"}
    </PendingButton>,
  );

describe("PendingButton", () => {
  it("is a quiet button with its label when nothing is left", async () => {
    const onClick = vi.fn();
    show(false, onClick);
    const b = screen.getByTestId("b");
    expect(b).not.toHaveAttribute("data-pending");
    expect(b).toHaveAccessibleName("Edit: Weapons");
    await userEvent.click(b);
    expect(onClick).toHaveBeenCalledOnce();
  });

  it("takes the red edge and says what is left when something is", () => {
    show(true);
    const b = screen.getByTestId("b");
    expect(b).toHaveAttribute("data-pending", "true");
    expect(b).toHaveAccessibleName("Edit: Weapons (Something is left)");
    expect(b.querySelector(".bg-danger")).not.toBeNull();
    expect(b).toHaveTextContent("Edit");
  });
});
