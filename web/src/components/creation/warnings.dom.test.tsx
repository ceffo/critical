import { describe, expect, it } from "vitest";
import { screen } from "@testing-library/react";
import { renderApp } from "@/test/render";
import type { CharacterWarning } from "@/types/character";
import { useWarningText } from "./warnings";

function Words({ w }: { w: CharacterWarning }) {
  return <p data-testid="words">{useWarningText()(w)}</p>;
}

const warning = {
  code: "specialist_race",
  field: "race",
  class: "illusionist",
  race: "elf",
} as CharacterWarning;

describe("useWarningText: the specialist's race (ADR 0050 step 14.1)", () => {
  it("words it in French and English with the race's name, never the raw code", async () => {
    await renderApp(<Words w={warning} />, { lng: "fr" });
    expect(screen.getByTestId("words").textContent).toMatch(
      /^La race \(.+\) ne figure pas parmi celles que le livre liste/,
    );
    expect(screen.getByTestId("words")).toHaveTextContent("Elfe");
  });

  it("says nothing without the race", async () => {
    await renderApp(<Words w={{ ...warning, race: undefined }} />);
    expect(screen.getByTestId("words")).toBeEmptyDOMElement();
  });
});
