import { cn } from "@/lib/utils";
import { parseSpellText } from "@/lib/spellMarkdown";

/**
 * A spell description in the catalogue's subset (ADR 0021): paragraphs, bullet lists and pipe
 * tables, rendered as React text only (no HTML). Table cells wrap; a table still too wide for the
 * block (many columns) scrolls sideways inside its own box.
 *
 * One column wherever it is shown (the lookup and the cast result), its measure capped at 75
 * characters (ADR 0022 steps 2.5 and 3.5). Blocks are 16 px apart, clearly more than the lines of a
 * paragraph; a table's cells have room (12 x 8 px) and its header is a raised band.
 */
export function SpellDescription({
  text,
  className,
  testId,
}: {
  text: string;
  className?: string;
  testId?: string;
}) {
  return (
    <div
      className={cn(
        "flex max-w-[75ch] flex-col gap-4 leading-relaxed",
        className,
      )}
      data-testid={testId}
    >
      {parseSpellText(text).map((b, i) => {
        switch (b.kind) {
          case "paragraph":
            return (
              <p key={i} className="whitespace-pre-line">
                {b.text}
              </p>
            );
          case "list":
            return (
              <ul key={i} className="flex list-disc flex-col gap-2 pl-5">
                {b.items.map((item, j) => (
                  <li key={j}>{item}</li>
                ))}
              </ul>
            );
          case "table":
            return (
              <div
                key={i}
                className="max-w-full overflow-x-auto rounded-md border border-border"
                data-testid="spell-table"
              >
                <table className="w-full border-collapse text-left">
                  <thead className="bg-raised text-foreground">
                    <tr>
                      {b.head.map((h, j) => (
                        <th
                          key={j}
                          scope="col"
                          className="min-w-20 border-b border-border px-3 py-2 align-bottom font-semibold"
                        >
                          {h}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {b.rows.map((row, j) => (
                      <tr
                        key={j}
                        className="border-b border-border last:border-b-0"
                      >
                        {row.map((c, k) => (
                          <td key={k} className="min-w-20 px-3 py-2 align-top">
                            {c}
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            );
        }
      })}
    </div>
  );
}
