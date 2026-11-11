import { useTranslation } from "react-i18next";
import { Plus, Trash } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { labelClass, fieldClass } from "@/components/dashboard/fieldStyles";
import { num, type CritDraft } from "./rulesetDraft";
import { UnsavedMark } from "./UnsavedMark";

/** Wide enough for the largest value: 100, or a multiplier such as 2.5. */
const cellClass = `${fieldClass} !w-16`;

/**
 * The house crit table: D100 ranges and their multipliers, entered at the table when the DM
 * gives the values. The page holds the rows and sends them with its one Save; the server
 * validates the whole table (no gap, no overlap, 1-100, multiplier ≥ 1, decimals allowed) and its
 * message is shown as is. This component only edits rows.
 * A player reads the rows (`editable` false): read-only fields, no add or remove.
 */
export function CritTableCard({
  rows,
  placeholder,
  onChange,
  error,
  dirty,
  editable = true,
}: {
  rows: CritDraft[];
  placeholder: boolean;
  onChange: (rows: CritDraft[]) => void;
  error: string;
  dirty: boolean;
  editable?: boolean;
}) {
  const { t } = useTranslation();

  function edit(i: number, field: keyof CritDraft, value: string) {
    onChange(rows.map((r, j) => (j === i ? { ...r, [field]: value } : r)));
  }

  function addRow() {
    // Start where the last row stops, so a new range is usually just "to" and "×" to fill in.
    const last = rows[rows.length - 1];
    const from = last ? num(last.to) + 1 : 1;
    onChange([
      ...rows,
      { from: Number.isNaN(from) ? "" : String(from), to: "", multiplier: "2" },
    ]);
  }

  return (
    <section
      aria-labelledby="ruleset-crit"
      className="zone-card flex min-w-0 flex-col"
      data-testid="crit-table"
    >
      <div className="card-band flex items-center gap-2 px-4 py-2">
        <h2 id="ruleset-crit" className="section-label">
          {t("critTable.title")}
        </h2>
        <UnsavedMark show={dirty} />
      </div>
      <div className="flex flex-col gap-2 px-4 pt-3 pb-3">
        <p className="text-body text-muted-foreground">
          {placeholder ? t("critTable.placeholder") : t("critTable.help")}
        </p>
        <div className="grid grid-cols-[repeat(3,4rem)_auto] items-end gap-2">
          <span className={labelClass}>{t("critTable.from")}</span>
          <span className={labelClass}>{t("critTable.to")}</span>
          <span className={labelClass}>{t("critTable.multiplier")}</span>
          <span />
          {rows.map((r, i) => (
            <div
              key={i}
              role="group"
              aria-label={t("critTable.row", { n: i + 1 })}
              className="contents"
            >
              {(["from", "to", "multiplier"] as const).map((f) => (
                <input
                  key={f}
                  className={cellClass}
                  // The multiplier takes decimals typed with a dot or a comma ("1.5", "1,5"),
                  // which a number input rejects in some locales: plain text, parsed on save.
                  type={f === "multiplier" ? "text" : "number"}
                  inputMode={f === "multiplier" ? "decimal" : "numeric"}
                  aria-label={t(`critTable.${f}`)}
                  value={r[f]}
                  readOnly={!editable}
                  onChange={(e) => edit(i, f, e.target.value)}
                />
              ))}
              {editable && (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  aria-label={t("critTable.remove", { n: i + 1 })}
                  onClick={() => onChange(rows.filter((_, j) => j !== i))}
                >
                  <Trash />
                </Button>
              )}
            </div>
          ))}
        </div>
        {error && (
          <p role="alert" className="text-body text-danger">
            {error}
          </p>
        )}
        {editable && (
          <div className="flex gap-2">
            <Button
              type="button"
              title={t("critTable.add")}
              aria-label={t("critTable.add")}
              onClick={addRow}
            >
              <Plus />
              {t("critTable.addShort")}
            </Button>
          </div>
        )}
      </div>
    </section>
  );
}
