import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import { renderApp } from "@/test/render";
import type { LevelLimit, SpellbookEntry } from "@/types/spells";
import { PrepareEditor } from "./PrepareEditor";

// The details popup fetches the spell: here it is a marker that it was asked to open.
vi.mock("./SpellDetailsDialog", () => ({
  SpellDetailsDialog: ({ spell }: { spell: { name: string } | null }) =>
    spell ? <div role="dialog">{spell.name}</div> : null,
}));

const spellbook: SpellbookEntry[] = [
  { id: "alarm", name: "Alarm", level: 1 },
  { id: "burning_hands", name: "Burning Hands", level: 1 },
  { id: "chromatic_orb", name: "Chromatic Orb", level: 1 },
  { id: "mirror_image", name: "Mirror Image", level: 2 },
];
const limits: LevelLimit[] = [
  { level: 1, max: 3 },
  { level: 2, max: 1 },
];

function editor(initial: string[], onSave = vi.fn()) {
  return renderApp(
    <PrepareEditor
      spellbook={spellbook}
      limits={limits}
      initial={initial}
      busy={false}
      onSave={onSave}
      onCancel={() => {}}
    />,
  );
}

const draft = (level: number) => screen.getByTestId(`draft-level-${level}`);
const save = () => screen.getByRole("button", { name: "Save preparation" });

describe("PrepareEditor", () => {
  beforeEach(() => vi.clearAllMocks());

  it("starts from today's preparation and the level filter narrows both columns", async () => {
    const { user } = await editor(["alarm", "mirror_image"]);
    expect(draft(1)).toHaveTextContent("Alarm");
    expect(draft(2)).toHaveTextContent("Mirror Image");
    expect(screen.getByTestId("prepared-dots-1")).toHaveAttribute(
      "data-state",
      "aee",
    );

    const chip = within(
      screen.getByRole("group", { name: "Filter by level" }),
    ).getByRole("button", { name: "Level 2" });
    await user.click(chip);
    expect(screen.queryByTestId("draft-level-1")).toBeNull();
    expect(screen.queryByTestId("grimoire-level-1")).toBeNull();
    expect(screen.getByTestId("grimoire-level-2")).toBeVisible();
    await user.click(chip);
    expect(draft(1)).toBeVisible();
  });

  it("allows 4 of 3 but gates Save until a copy comes off; the last − removes the spell", async () => {
    const onSave = vi.fn();
    const { user } = await editor(["alarm", "mirror_image"], onSave);
    await user.click(screen.getByRole("button", { name: "Add Burning Hands" }));
    await user.click(screen.getByRole("button", { name: "Add Chromatic Orb" }));
    await user.click(
      screen.getByRole("button", { name: "One more copy of Alarm" }),
    );
    expect(
      within(draft(1)).getAllByTestId("prepared-count")[0],
    ).toHaveTextContent("2");
    expect(screen.getByTestId("prepared-dots-1")).toHaveAttribute(
      "data-state",
      "aaao",
    );
    expect(save()).toBeDisabled();
    expect(screen.getByTestId("over-budget")).toHaveTextContent("level 1");

    await user.click(screen.getByRole("button", { name: "Remove Alarm" }));
    expect(save()).toBeEnabled();
    await user.click(
      screen.getByRole("button", { name: "Remove Mirror Image" }),
    );
    expect(draft(2)).not.toHaveTextContent("Mirror Image");

    await user.click(save());
    expect(onSave).toHaveBeenCalledTimes(1);
    expect([...onSave.mock.calls[0][0]].sort()).toEqual([
      "alarm",
      "burning_hands",
      "chromatic_orb",
    ]);
  });

  it("a spell's name opens its details", async () => {
    const { user } = await editor(["alarm"]);
    await user.click(within(draft(1)).getByRole("button", { name: /^Alarm$/ }));
    expect(screen.getByRole("dialog")).toHaveTextContent("Alarm");
  });

  it("lists a prepared spell the spellbook no longer has as removed and does not send it", async () => {
    const onSave = vi.fn();
    const { user } = await editor(["alarm", "shield"], onSave);
    const stale = screen.getByTestId("stale-prepared");
    expect(stale).toHaveTextContent("will be removed");
    expect(within(stale).getAllByRole("listitem")).toHaveLength(1);
    expect(draft(1)).toHaveTextContent("Alarm");
    await user.click(save());
    expect(onSave).toHaveBeenCalledWith(["alarm"]);
  });
});
