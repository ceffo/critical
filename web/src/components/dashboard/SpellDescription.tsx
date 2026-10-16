import { cn } from "@/lib/utils";
import { parseSpellText } from "@/lib/spellMarkdown";

/**
 * A spell description in the catalogue's subset (ADR 0021): paragraphs, bullet lists and pipe
 * tables, rendered as React text only (no HTML). Table cells wrap; a table still too wide for the
 * block (many columns) scrolls sideways inside its own box.
 *
 * `columns`: the text flows in two columns when the block is wide enough for two of at least
 * 16rem, 24 px apart (the lookup's description, ADR 0022 steps 2.5 and 3.2); a block never
 * splits across them and a table spans both, so it keeps the whole width.
 */
export function SpellDescription({
  text,
  className,
  testId,
  columns = false,
}: {
  text: string;
  className?: string;
  testId?: string;
  columns?: boolean;
}) {
  return (
    <div
      className={cn(
        columns
          ? "gap-x-6 [columns:2_16rem] *:mb-2 *:break-inside-avoid"
          : "flex flex-col gap-2",
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
              <ul key={i} className="flex list-disc flex-col gap-1 pl-5">
                {b.items.map((item, j) => (
                  <li key={j}>{item}</li>
                ))}
              </ul>
            );
          case "table":
            return (
              <div
                key={i}
                className={cn(
                  "max-w-full overflow-x-auto rounded-md border border-border",
                  columns && "[column-span:all]",
                )}
                data-testid="spell-table"
              >
                <table className="w-full border-collapse text-left">
                  <thead className="bg-muted text-foreground">
                    <tr>
                      {b.head.map((h, j) => (
                        <th
                          key={j}
                          scope="col"
                          className="min-w-20 border-b border-border px-2 py-1 align-bottom font-semibold"
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
                          <td key={k} className="min-w-20 px-2 py-1 align-top">
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
