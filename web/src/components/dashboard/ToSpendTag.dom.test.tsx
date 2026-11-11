import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import { renderApp } from "@/test/render";
import type { LevelGrants } from "@/types/character";
import { ToSpendTag } from "./ToSpendTag";

const none: LevelGrants = {
  weaponSlots: 0,
  nonWeaponSlots: 0,
  skillPoints: 0,
  freeSpells: [],
};

/** Stands for the card: the counts come from the server's character, changed by spending. */
function Card({ onOpenEditor }: { onOpenEditor: (e: string) => void }) {
  const [left, setLeft] = useState<LevelGrants>({
    ...none,
    weaponSlots: 1,
    skillPoints: 30,
  });
  return (
    <>
      <ToSpendTag
        toSpend={left}
        onOpenEditor={onOpenEditor}
        onOpenFreeSpell={() => {}}
      />
      <button onClick={() => setLeft({ ...left, weaponSlots: 0 })}>
        spend slot
      </button>
      <button onClick={() => setLeft(none)}>spend all</button>
    </>
  );
}

// ADR 0037 step 15: the card's tag follows the server's left counts as they change, with no reload.
describe("to-spend tag", () => {
  it("lists what is left, drops a row when its count reaches 0 and the tag when nothing is left", async () => {
    const onOpenEditor = vi.fn();
    const { user } = await renderApp(<Card onOpenEditor={onOpenEditor} />);
    await user.click(screen.getByTestId("to-spend-tag"));
    const pop = screen.getByTestId("level-up-to-spend");
    expect(within(pop).getAllByRole("listitem")).toHaveLength(2);
    await user.click(within(pop).getByRole("button", { name: /weapon/i }));
    expect(onOpenEditor).toHaveBeenCalledWith("weapon");

    await user.click(screen.getByText("spend slot"));
    await user.click(screen.getByTestId("to-spend-tag"));
    expect(
      within(screen.getByTestId("level-up-to-spend")).getAllByRole("listitem"),
    ).toHaveLength(1);

    await user.keyboard("{Escape}");
    await user.click(screen.getByText("spend all"));
    expect(screen.queryByTestId("to-spend-tag")).toBeNull();
  });
});
