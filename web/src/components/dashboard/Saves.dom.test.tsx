import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { renderApp } from "@/test/render";
import { Saves } from "./Saves";

const rollSave = vi.hoisted(() => vi.fn());
vi.mock("@/stores/roll", () => ({
  useRollStore: () => ({ rollSave, rolling: false }),
}));
vi.mock("@/stores/character", () => ({
  useCharacterStore: (pick: (s: { character: null }) => unknown) =>
    pick({ character: null }),
}));
vi.mock("@/stores/log", () => ({
  useLogStore: (pick: (s: { entries: [] }) => unknown) => pick({ entries: [] }),
}));
// The spell search is SaveSpell's own; here a button names Polymorph Other and a tag clears it.
vi.mock("./SaveSpell", () => ({
  SaveSpell: ({
    spellId,
    onChange,
  }: {
    spellId?: string;
    onChange: (id: string | undefined) => void;
  }) =>
    spellId ? (
      <button
        type="button"
        data-testid="stub-clear"
        data-spell={spellId}
        onClick={() => onChange(undefined)}
      />
    ) : (
      <button
        type="button"
        data-testid="stub-name"
        onClick={() => onChange("polymorph_other")}
      />
    ),
}));

// ADR 0050 steps 12 and 13 on the save card: the wand opens the spell line without the mind flag
// (a specialist names the spell for his +1), and "Specialist caster", shown once a spell is named,
// reaches the request (the −1). The rule itself is Go's (combatsvc).
describe("Saves: the spell line and the specialist caster flag", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    rollSave.mockResolvedValue(true);
  });

  it("sends the spell and the flag, then clears both for the next save", async () => {
    const { user } = await renderApp(<Saves />);
    expect(screen.queryByTestId("save-spell-line")).toBeNull();
    await user.click(screen.getByTestId("save-spell-toggle"));
    expect(screen.queryByTestId("save-caster-specialist")).toBeNull();
    await user.click(screen.getByTestId("stub-name"));
    await user.click(screen.getByTestId("save-caster-specialist"));
    await user.click(
      screen.getByRole("button", { name: "Roll save vs Petrify/Polymorph" }),
    );
    expect(rollSave).toHaveBeenLastCalledWith(
      "petri_poly",
      false,
      "polymorph_other",
      true,
    );
    expect(screen.queryByTestId("save-spell-line")).toBeNull();
  });

  it("names a spell without the flag; clearing the spell drops the flag", async () => {
    const { user } = await renderApp(<Saves />);
    await user.click(screen.getByTestId("save-spell-toggle"));
    await user.click(screen.getByTestId("stub-name"));
    await user.click(screen.getByTestId("save-caster-specialist"));
    await user.click(screen.getByTestId("stub-clear"));
    await user.click(screen.getByTestId("stub-name"));
    expect(screen.getByTestId("save-caster-specialist")).not.toBeChecked();
    await user.click(
      screen.getByRole("button", { name: "Roll save vs Spells" }),
    );
    expect(rollSave).toHaveBeenLastCalledWith(
      "spell",
      false,
      "polymorph_other",
      false,
    );
  });

  it("closing the wand's line drops the spell; the mind flag still opens it", async () => {
    const { user } = await renderApp(<Saves />);
    const toggle = screen.getByTestId("save-spell-toggle");
    await user.click(toggle);
    await user.click(screen.getByTestId("stub-name"));
    await user.click(toggle);
    expect(screen.queryByTestId("save-spell-line")).toBeNull();
    await user.click(screen.getByTestId("save-mind-affecting"));
    expect(screen.getByTestId("stub-name")).toBeInTheDocument();
    expect(toggle).toBeDisabled();
    await user.click(
      screen.getByRole("button", { name: "Roll save vs Spells" }),
    );
    expect(rollSave).toHaveBeenLastCalledWith("spell", true, undefined, false);
  });
});
