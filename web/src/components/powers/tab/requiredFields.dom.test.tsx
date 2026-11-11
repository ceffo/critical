import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import { useTranslation } from "react-i18next";
import { renderApp } from "@/test/render";
import { Needs } from "./Required";
import { TargetBand } from "./TargetBand";
import { NO_TARGET, targetNeeds, type TargetDraft } from "./bar";

// The band never fetches here: no defence catalogue.
vi.mock("@/api/powers", () => ({
  searchPowers: () => Promise.resolve({ items: [] }),
}));

/** The band and the reason beside the action, fed by the one condition the Contact form uses. */
function ContactTarget() {
  const { t } = useTranslation();
  const [target, setTarget] = useState<TargetDraft>(NO_TARGET);
  const needs = targetNeeds("table", target);
  return (
    <>
      <TargetBand target={target} needs={needs} onChange={setTarget} />
      <Needs needs={needs} testId="table-need" />
      <button disabled={needs.length > 0}>{t("powers.use.use")}</button>
    </>
  );
}

const band = () => screen.getByTestId("target-band");
const marks = (word: string) => within(band()).queryAllByText(word);

describe("Contact target: required fields", () => {
  beforeEach(() => vi.clearAllMocks());

  it("marks what is missing and names it beside the disabled action, then clears as it is filled", async () => {
    const { user } = await renderApp(<ContactTarget />);
    const need = screen.getByTestId("table-need");
    expect(screen.getByRole("button", { name: "Use" })).toBeDisabled();
    expect(need).toHaveTextContent(
      "Still needed: the opponent’s name, its mind, open or closed, its level.",
    );
    expect(marks("required")).toHaveLength(3);
    expect(screen.getByRole("textbox", { name: "Opponent" })).toBeRequired();

    await user.type(screen.getByTestId("target-name"), "Orc");
    await user.click(within(band()).getByRole("radio", { name: "Open" }));
    expect(need).toHaveTextContent("Still needed: its level.");
    expect(marks("required")).toHaveLength(1);

    await user.type(within(band()).getByLabelText("Level"), "3");
    expect(screen.getByRole("button", { name: "Use" })).toBeEnabled();
    expect(screen.queryByTestId("table-need")).toBeNull();
    expect(marks("required")).toHaveLength(0);
  });

  it("says the same in French", async () => {
    const { user } = await renderApp(<ContactTarget />, { lng: "fr" });
    await user.type(screen.getByTestId("target-name"), "Orc");
    await user.click(within(band()).getByRole("radio", { name: "Ouvert" }));
    expect(screen.getByTestId("table-need")).toHaveTextContent(
      "Il manque : son niveau.",
    );
    expect(marks("obligatoire")).toHaveLength(1);
    await user.clear(screen.getByTestId("target-name"));
    expect(screen.getByTestId("table-need")).toHaveTextContent(
      "Il manque : le nom de l’adversaire",
    );
  });
});
