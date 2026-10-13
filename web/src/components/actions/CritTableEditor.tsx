import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { fetchCritTable, saveCritTable } from "@/api/combat";
import { captionClass, fieldClass } from "@/components/dashboard/fieldStyles";
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
 * multiplier ≥ 1) and its message is shown as is; this component only edits and sends rows.
 */
export function CritTableEditor() {
  const { t } = useTranslation();
  const [table, setTable] = useState<CritTable | null>(null);
  const [rows, setRows] = useState<Draft[]>([]);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  function load(next: CritTable) {
    setTable(next);
    setRows(toDrafts(next.rows));
  }

  useEffect(() => {
    fetchCritTable()
      .then(load)
      .catch((e: unknown) =>
        setError(e instanceof Error ? e.message : String(e)),
      );
  }, []);

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
            multiplier: num(r.multiplier),
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
    <details
      className="mt-4 max-w-xl rounded-lg border px-3 py-2"
      data-testid="crit-table"
    >
      <summary className="cursor-pointer text-sm font-medium">
        {t("critTable.title")}
      </summary>
      <div className="mt-3 flex flex-col gap-2">
        <p className="text-sm text-muted-foreground">
          {table?.placeholder
            ? t("critTable.placeholder")
            : t("critTable.help")}
        </p>
        <div className="grid grid-cols-[1fr_1fr_1fr_auto] items-end gap-2">
          <span className={captionClass}>{t("critTable.from")}</span>
          <span className={captionClass}>{t("critTable.to")}</span>
          <span className={captionClass}>{t("critTable.multiplier")}</span>
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
                  type="number"
                  inputMode="numeric"
                  aria-label={t(`critTable.${f}`)}
                  value={r[f]}
                  onChange={(e) => edit(i, f, e.target.value)}
                />
              ))}
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
                <Trash2 />
              </Button>
            </div>
          ))}
        </div>
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        {saved && (
          <p className="text-sm text-muted-foreground">
            {t("critTable.saved")}
          </p>
        )}
        <div className="flex gap-2">
          <Button type="button" variant="outline" onClick={addRow}>
            <Plus />
            {t("critTable.add")}
          </Button>
          <Button
            type="button"
            disabled={saving || table === null}
            onClick={() => void save()}
          >
            {t("critTable.save")}
          </Button>
        </div>
      </div>
    </details>
  );
}
