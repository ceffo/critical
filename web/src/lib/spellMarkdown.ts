// The spell catalogue's description subset (ADR 0021 step 1): blocks separated by a blank line,
// each a paragraph, a "- " bullet list or a GFM pipe table. Parsed into plain data, rendered as
// React text (components/dashboard/SpellDescription.tsx): nothing in a description is ever HTML.

export type SpellBlock =
  | { kind: "paragraph"; text: string }
  | { kind: "list"; items: string[] }
  | { kind: "table"; head: string[]; rows: string[][] };

/** "| --- | :---: |": the delimiter row under a table's header. */
const DELIMITER = /^\|(\s*:?-+:?\s*\|)+$/;

function isTableLine(line: string): boolean {
  return line.length > 1 && line.startsWith("|") && line.endsWith("|");
}

/** The cells of "| a | b |", trimmed. */
function cells(line: string): string[] {
  return line
    .slice(1, -1)
    .split("|")
    .map((c) => c.trim());
}

function block(text: string): SpellBlock {
  const lines = text.split("\n").map((l) => l.trim());
  if (lines.every((l) => l.startsWith("- ")))
    return { kind: "list", items: lines.map((l) => l.slice(2).trim()) };
  if (
    lines.length >= 2 &&
    lines.every(isTableLine) &&
    DELIMITER.test(lines[1].replace(/\s/g, ""))
  ) {
    const head = cells(lines[0]);
    const rows = lines.slice(2).map((l) => {
      const row = cells(l).slice(0, head.length);
      while (row.length < head.length) row.push("");
      return row;
    });
    return { kind: "table", head, rows };
  }
  // Anything else stays text, line breaks kept: a malformed block is shown, never dropped.
  return { kind: "paragraph", text: lines.join("\n") };
}

/** The blocks of a description; blank lines (with or without spaces) separate them. */
export function parseSpellText(text: string): SpellBlock[] {
  return text
    .trim()
    .split(/\n[ \t]*\n/)
    .map((b) => b.trim())
    .filter(Boolean)
    .map(block);
}
