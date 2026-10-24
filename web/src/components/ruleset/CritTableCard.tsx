import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus, Trash } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { saveCritTable } from "@/api/ruleset";
import { labelClass, fieldClass } from "@/components/dashboard/fieldStyles";
import { parseDecimal } from "@/lib/decimalInput";
import type { CritRow, CritTable } from "@/types/combat";

/** A row being edited: the inputs hold text so a half-typed number is not forced back. */
interface Draft {
  from: string;
  to: string;
  multiplier: string;
}

const toDrafts = (rows: CritRow[]): Draft[] =>
  rows.map((r) => ({
    from: String(r.from),
    to: String(r.to),
    multiplier: String(r.multiplier),
  }));

/** Empty or non-numeric text becomes NaN, which the server refuses with a message naming the row. */
const num = (s: string): number => (s.trim() === "" ? Number.NaN : Number(s));

/**
 * The house crit table: D100 ranges and their multipliers, entered at the table when the DM
 * gives the values. The server validates the whole table on save (no gap, no overlap, 1-100,
 * multiplier ≥ 1, decimals allowed) and its message is shown as is; this component only edits and sends rows.
 * A player reads the rows (`editable` false): read-only fields, no add, remove or save.
 */
export function CritTableCard({
  initial,
  editable = true,
}: {
  initial: CritTable;
  editable?: boolean;
}) {
  const { t } = useTranslation();
  const [table, setTable] = useState<CritTable>(initial);
  const [rows, setRows] = useState<Draft[]>(() => toDrafts(initial.rows));
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  function load(next: CritTable) {
    setTable(next);
    setRows(toDrafts(next.rows));
  }

  function edit(i: number, field: keyof Draft, value: string) {
    setSaved(false);
    setRows((rs) => rs.map((r, j) => (j === i ? { ...r, [field]: value } : r)));
  }

  function addRow() {
    setSaved(false);
    // Start where the last row stops, so a new range is usually just "to" and "×" to fill in.
    const last = rows[rows.length - 1];
    const from = last ? num(last.to) + 1 : 1;
    setRows((rs) => [
      ...rs,
      { from: Number.isNaN(from) ? "" : String(from), to: "", multiplier: "2" },
    ]);
  }

  async function save() {
    setSaving(true);
    setError("");
    try {
      load(
        await saveCritTable(
          rows.map((r) => ({
            from: num(r.from),
            to: num(r.to),
            multiplier: parseDecimal(r.multiplier),
          })),
        ),
      );
      setSaved(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  }

  return (
    <section
      aria-labelledby="ruleset-crit"
      className="zone-card flex min-w-0 flex-col"
      data-testid="crit-table"
    >
      <div className="card-band flex items-center px-4 py-2">
        <h2 id="ruleset-crit" className="section-label">
          {t("critTable.title")}
        </h2>
      </div>
      <div className="flex flex-col gap-2 px-4 pt-3 pb-3">
        <p className="text-body text-muted-foreground">
          {table.placeholder ? t("critTable.placeholder") : t("critTable.help")}
        </p>
        <div className="grid grid-cols-[1fr_1fr_1fr_auto] items-end gap-2">
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
                  className={fieldClass}
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
                  onClick={() => {
                    setSaved(false);
                    setRows((rs) => rs.filter((_, j) => j !== i));
                  }}
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
        {saved && (
          <p className="text-body text-muted-foreground">
            {t("critTable.saved")}
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
            <Button
              type="button"
              disabled={saving}
              title={t("critTable.save")}
              aria-label={t("critTable.save")}
              onClick={() => void save()}
            >
              {t("critTable.saveShort")}
            </Button>
          </div>
        )}
      </div>
    </section>
  );
}
