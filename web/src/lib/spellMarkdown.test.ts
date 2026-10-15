import { describe, expect, it } from "vitest";
import { parseSpellText } from "./spellMarkdown";

describe("parseSpellText", () => {
  it("splits paragraphs on blank lines", () => {
    expect(parseSpellText("One.\n\nTwo.")).toEqual([
      { kind: "paragraph", text: "One." },
      { kind: "paragraph", text: "Two." },
    ]);
  });

  it("reads consecutive '- ' lines as a bullet list", () => {
    expect(parseSpellText("Copies:\n\n- One copy.\n- Another.")).toEqual([
      { kind: "paragraph", text: "Copies:" },
      { kind: "list", items: ["One copy.", "Another."] },
    ]);
  });

  it("reads a pipe table: header, delimiter row, body rows", () => {
    expect(
      parseSpellText(
        "| Level | Colour |\n| --- | :---: |\n| 1 | Pearl |\n| 2 | Ruby |",
      ),
    ).toEqual([
      {
        kind: "table",
        head: ["Level", "Colour"],
        rows: [
          ["1", "Pearl"],
          ["2", "Ruby"],
        ],
      },
    ]);
  });

  it("pads a short row and cuts a long one to the header's width", () => {
    expect(
      parseSpellText("| A | B |\n| --- | --- |\n| 1 |\n| 1 | 2 | 3 |"),
    ).toEqual([
      {
        kind: "table",
        head: ["A", "B"],
        rows: [
          ["1", ""],
          ["1", "2"],
        ],
      },
    ]);
  });

  it("keeps a malformed table or a mixed block as text, never drops it", () => {
    expect(parseSpellText("| A | B |\n| 1 | 2 |")).toEqual([
      { kind: "paragraph", text: "| A | B |\n| 1 | 2 |" },
    ]);
    expect(parseSpellText("Intro\n- item")).toEqual([
      { kind: "paragraph", text: "Intro\n- item" },
    ]);
  });

  it("keeps markup as plain text: no HTML, no emphasis", () => {
    expect(parseSpellText("<b>bold</b> *star*")).toEqual([
      { kind: "paragraph", text: "<b>bold</b> *star*" },
    ]);
  });

  it("tolerates blank lines holding spaces and surrounding whitespace", () => {
    expect(parseSpellText("  One.\n  \nTwo.\n")).toEqual([
      { kind: "paragraph", text: "One." },
      { kind: "paragraph", text: "Two." },
    ]);
    expect(parseSpellText("")).toEqual([]);
  });
});
