import { describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { renderApp, type Language } from "@/test/render";
import { emptyDraft } from "@/lib/itemForm";
import { ItemForm } from "./ItemForm";

// The typed add form (ADR 0033 step 1): step 1 is what the piece is, step 2 only its category's
// fields; Back keeps what was typed.

async function form(lng: Language = "en") {
  const onSave = vi.fn().mockResolvedValue(null);
  const view = await renderApp(
    <ItemForm
      initial={emptyDraft()}
      busy={false}
      onSave={onSave}
      onCancel={() => {}}
    />,
    { lng },
  );
  return { ...view, onSave };
}

const field = (name: string) =>
  document.querySelector(`[data-field='${name}']`);

async function startStep2(
  view: Awaited<ReturnType<typeof form>>,
  name: string,
  category: string,
) {
  await view.user.type(screen.getByLabelText("Name"), name);
  await view.user.selectOptions(screen.getByLabelText("Category"), category);
  await view.user.click(screen.getByRole("button", { name: "Next" }));
}

describe("ItemForm, adding in two steps", () => {
  it("step 1 shows the name and the category only", async () => {
    await form();
    expect(field("name")).not.toBeNull();
    expect(field("category")).not.toBeNull();
    expect(field("count")).toBeNull();
    expect(field("baseAc")).toBeNull();
    expect(field("damageSm")).toBeNull();
  });

  it("a weapon shows weapon fields, no armour field", async () => {
    const v = await form();
    await startStep2(v, "Axe", "weapon");
    for (const f of ["damageSm", "weaponType", "magicToHit", "twoHanded"])
      expect(field(f), f).not.toBeNull();
    for (const f of ["baseAc", "shieldKind", "ammoKind"])
      expect(field(f), f).toBeNull();
    expect(field("name")).toBeNull();
  });

  it("an armour shows armour fields and its +N, no weapon field", async () => {
    const v = await form();
    await startStep2(v, "Plate", "armour");
    for (const f of ["baseAc", "skillColumn", "armorRow", "plus"])
      expect(field(f), f).not.toBeNull();
    for (const f of ["damageSm", "weaponType", "shieldKind"])
      expect(field(f), f).toBeNull();
  });

  it("a plain item shows its count; every shared field sits under More options", async () => {
    const v = await form();
    await startStep2(v, "Rope", "gear");
    expect(field("count")).not.toBeNull();
    for (const f of ["baseAc", "damageSm", "magicKind", "plus"])
      expect(field(f), f).toBeNull();
    const more = screen.getByTestId("item-more") as HTMLDetailsElement;
    expect(more.open).toBe(false);
    await v.user.click(screen.getByText("More options"));
    for (const f of [
      "behaviour",
      "effectDice",
      "effectSave",
      "component",
      "notes",
    ])
      expect(more.contains(field(f)), f).toBe(true);
  });

  it("Back keeps what was typed, and Add sends it", async () => {
    const v = await form();
    await startStep2(v, "Chain", "armour");
    await v.user.clear(screen.getByLabelText("Base AC"));
    await v.user.type(screen.getByLabelText("Base AC"), "5");
    await v.user.click(screen.getByRole("button", { name: "Back" }));
    expect(screen.getByLabelText("Name")).toHaveValue("Chain");
    expect(screen.getByLabelText("Category")).toHaveValue("armour");
    await v.user.click(screen.getByRole("button", { name: "Next" }));
    expect(screen.getByLabelText("Base AC")).toHaveValue("5");
    await v.user.click(screen.getByRole("button", { name: "Add" }));
    expect(v.onSave).toHaveBeenCalledOnce();
    expect(v.onSave.mock.calls[0][0]).toMatchObject({ baseAc: 5 });
  });

  it("Enter in the name moves to step 2 (keyboard flow)", async () => {
    const v = await form();
    await v.user.type(screen.getByLabelText("Name"), "Rope{Enter}");
    expect(field("count")).not.toBeNull();
    expect(screen.getByTestId("item-summary")).toHaveTextContent("Rope");
  });

  it("speaks French", async () => {
    const v = await form("fr");
    await v.user.type(screen.getByLabelText("Nom"), "Corde");
    await v.user.click(screen.getByRole("button", { name: "Suivant" }));
    expect(screen.getByRole("button", { name: "Retour" })).toBeVisible();
    expect(screen.getByText("Plus d'options")).toBeVisible();
  });
});
