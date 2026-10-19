import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { setAcOverride } from "@/api/ac";
import { useTableStore } from "@/stores/table";
import { cn } from "@/lib/utils";

/**
 * Sets the AC override (ADR 0033 step 13): a number -10 to 10 and why, one logged move. The
 * server checks both; a refused field shows its message under it.
 */
export function AcOverrideForm({
  initial,
  onDone,
}: {
  initial: number;
  onDone: () => void;
}) {
  const { t } = useTranslation();
  const submit = useTableStore((s) => s.submit);
  const busy = useTableStore((s) => s.busy);
  const [value, setValue] = useState(String(initial));
  const [label, setLabel] = useState("");
  const [error, setError] = useState<"value" | "label" | null>(null);

  return (
    <form
      className="flex flex-col gap-2"
      data-testid="ac-override-form"
      onSubmit={async (e) => {
        e.preventDefault();
        const n = Number(value);
        if (value.trim() === "" || !Number.isInteger(n)) {
          setError("value");
          return;
        }
        const rejected = await submit(() => setAcOverride(n, label));
        if (rejected?.field === "value" || rejected?.field === "label")
          setError(rejected.field);
        else if (!rejected) onDone();
      }}
    >
      <div className="flex items-start gap-2">
        <label className="flex w-20 shrink-0 flex-col gap-1 text-label text-muted-foreground">
          {t("ac.override.value")}
          <Input
            type="number"
            inputMode="numeric"
            min={-10}
            max={10}
            value={value}
            autoFocus
            aria-invalid={error === "value"}
            data-testid="ac-override-value"
            onChange={(e) => {
              setValue(e.target.value);
              setError(null);
            }}
          />
        </label>
        <label className="flex min-w-0 flex-1 flex-col gap-1 text-label text-muted-foreground">
          {t("ac.override.label")}
          <Input
            value={label}
            maxLength={60}
            placeholder={t("ac.override.labelPlaceholder")}
            aria-invalid={error === "label"}
            data-testid="ac-override-label"
            onChange={(e) => {
              setLabel(e.target.value);
              setError(null);
            }}
          />
        </label>
      </div>
      {error && (
        <p
          className={cn("text-label text-danger")}
          data-testid="ac-override-error"
        >
          {t(`ac.override.errors.${error}`)}
        </p>
      )}
      <div className="flex justify-end gap-2">
        <Button type="button" size="sm" variant="ghost" onClick={onDone}>
          {t("ac.override.cancel")}
        </Button>
        <Button
          type="submit"
          size="sm"
          disabled={busy}
          title={t("ac.override.saveHint")}
          aria-label={t("ac.override.saveHint")}
          data-testid="ac-override-save"
        >
          {t("ac.override.save")}
        </Button>
      </div>
    </form>
  );
}
