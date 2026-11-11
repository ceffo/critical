import { useRef } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SHORTCUTS_OFF, useKeyClicks } from "./shortcuts";

function Probe({
  onClick,
  disabled = false,
}: {
  onClick: () => void;
  disabled?: boolean;
}) {
  const ref = useRef<HTMLButtonElement>(null);
  useKeyClicks("n", ref);
  return (
    <div>
      <button ref={ref} onClick={onClick}>
        Next round
      </button>
      <input aria-label="Target" />
      <div {...{ [SHORTCUTS_OFF]: "" }}>
        <button>Roll</button>
      </div>
      <div role="dialog" aria-label="Popup">
        <button>Inside</button>
      </div>
    </div>
  );
}

describe("a plain key clicking a button", () => {
  it("clicks the button on the plain key and on nothing else", async () => {
    const onClick = vi.fn();
    render(<Probe onClick={onClick} />);
    // A dialog is always open in this probe, so close it for the plain case.
    screen.getByRole("dialog").remove();
    const user = userEvent.setup();
    await user.keyboard("n");
    expect(onClick).toHaveBeenCalledTimes(1);
    await user.keyboard("{Control>}n{/Control}");
    await user.keyboard("{Alt>}n{/Alt}");
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("does nothing while typing in a field, in an opted-out block or with a dialog open", async () => {
    const onClick = vi.fn();
    render(<Probe onClick={onClick} />);
    const user = userEvent.setup();
    // The dialog is open: even a plain key on the page is the dialog's.
    await user.keyboard("n");
    expect(onClick).not.toHaveBeenCalled();
    screen.getByRole("dialog").remove();
    await user.click(screen.getByLabelText("Target"));
    await user.keyboard("n");
    await user.click(screen.getByRole("button", { name: "Roll" }));
    await user.keyboard("n");
    expect(onClick).not.toHaveBeenCalled();
  });
});
