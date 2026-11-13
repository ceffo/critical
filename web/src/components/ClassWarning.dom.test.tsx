import { describe, expect, it } from "vitest";
import { screen } from "@testing-library/react";
import { renderApp } from "@/test/render";
import { NonproficientTag } from "./ClassWarning";

describe("NonproficientTag's hint", () => {
  it.each([
    [
      "en",
      true,
      /^No proficiency with this weapon: .* Pick it by name in the Skills tab\.$/,
    ],
    ["en", false, /no catalogue weapon: pick one in its item’s edit\.$/],
    ["fr", true, /par son nom dans l’onglet Compétences\.$/],
    ["fr", false, /dans la modification de l’objet\.$/],
  ] as const)("%s, has a catalogue code: %s", async (lng, hasCode, hint) => {
    await renderApp(<NonproficientTag penalty={-2} hasCode={hasCode} />, {
      lng,
    });
    expect(screen.getByTestId("nonproficient")).toHaveAttribute(
      "title",
      expect.stringMatching(hint),
    );
  });
});
