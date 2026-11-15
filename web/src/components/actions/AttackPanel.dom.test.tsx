import { afterEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { renderApp } from "@/test/render";
import { useItemsStore } from "@/stores/items";
import { useAttackStore } from "@/stores/attack";
import type { AttackFormView } from "@/types/combat";
import { AttackPanel } from "./AttackPanel";

const noWeaponView: AttackFormView = {
  form: {
    weaponId: "",
    previousWeaponId: "",
    ammoItemId: "",
    targetAc: null,
    targetSize: "sm",
    situational: { value: 0, label: "" },
  },
  ammoFromDefault: false,
  ammoChoices: [],
  preview: null,
  maxRealAc: 10,
} as unknown as AttackFormView;

const fetchAttackForm = vi.hoisted(() => vi.fn());
vi.mock("@/api/combat", async (orig) => ({
  ...(await orig<typeof import("@/api/combat")>()),
  fetchAttackForm,
}));

afterEach(() => {
  useItemsStore.setState({ items: [], weapons: [], weaponsLoaded: false });
  useAttackStore.setState({ view: null });
  fetchAttackForm.mockReset();
});

describe("AttackPanel without weapons", () => {
  it("says there is no weapon once the weapons are read, never keeps loading", async () => {
    fetchAttackForm.mockResolvedValue(noWeaponView);
    useItemsStore.setState({ weapons: [], weaponsLoaded: true });
    await renderApp(<AttackPanel />);
    expect(await screen.findByText("No weapons.")).toBeInTheDocument();
    expect(screen.queryByText("Loading weapons…")).not.toBeInTheDocument();
  });

  it("still says loading until the weapons are read", async () => {
    fetchAttackForm.mockResolvedValue(noWeaponView);
    useItemsStore.setState({ weapons: [], weaponsLoaded: false });
    await renderApp(<AttackPanel />);
    expect(screen.getByText("Loading weapons…")).toBeInTheDocument();
  });
});
