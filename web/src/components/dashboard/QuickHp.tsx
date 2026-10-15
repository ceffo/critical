import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Heart, ArrowUUpLeft, Sword } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useLogStore } from "@/stores/log";
import { useTableStore } from "@/stores/table";
import { entryText } from "@/lib/logSummary";
import { captionClass, fieldClass } from "./fieldStyles";

/** Quick damage/heal entry with a source. The server records it in the action log. */
export function QuickHp() {
  const { t } = useTranslation();
  const { recordHp, undoHp, busy } = useTableStore();
  const [amount, setAmount] = useState("");
  const [source, setSource] = useState("");
  // The log is a stack: undo pops the newest entry, so it is
  // only offered when that entry is an HP change (entries are newest first).
  const top = useLogStore((st) => st.entries.find((e) => e.status !== "voided"));
  const blockedBy = top && top.kind !== "hp_change" ? entryText(top, t) : null;

  const n = Number.parseInt(amount, 10);
  const valid = n > 0 && source.trim() !== "";

  async function apply(sign: 1 | -1) {
    if (!valid) return;
    if (await recordHp(sign * n, source.trim())) {
      setAmount("");
      setSource("");
    }
  }

  return (
    <section
      className="shrink-0 rounded-xl border bg-card p-3"
      role="group"
      aria-label={t("hp.change")}
    >
      <div className="mb-2 flex items-center justify-between">
        <h2 className="text-sm font-semibold">{t("hp.change")}</h2>
        <Button
          size="xs"
          variant="ghost"
          disabled={busy || blockedBy !== null}
          aria-describedby={blockedBy ? "hp-undo-hint" : undefined}
          onClick={() => void undoHp()}
        >
          <ArrowUUpLeft />
          {t("hp.undoLast")}
        </Button>
      </div>
      {blockedBy && (
        <p
          id="hp-undo-hint"
          className="-mt-1 mb-2 truncate text-xs text-muted-foreground"
          data-testid="hp-undo-hint"
          title={t("hp.undoBlocked", { entry: blockedBy })}
        >
          {t("hp.undoBlocked", { entry: blockedBy })}
        </p>
      )}
      <div className="grid grid-cols-[88px_minmax(0,1fr)] gap-2">
        <label className="flex min-w-0 flex-col gap-1">
          <span className={captionClass}>{t("hp.amount")}</span>
          <input
            className={fieldClass}
            type="number"
            min={1}
            inputMode="numeric"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
          />
        </label>
        <label className="flex min-w-0 flex-col gap-1">
          <span className={captionClass}>{t("hp.source")}</span>
          <input
            className={fieldClass}
            type="text"
            placeholder={t("hp.sourcePlaceholder")}
            value={source}
            onChange={(e) => setSource(e.target.value)}
          />
        </label>
        <div className="col-span-2 grid grid-cols-2 gap-2">
        <Button
          variant="destructive"
          className="h-9"
          disabled={!valid || busy}
          onClick={() => void apply(-1)}
        >
          <Sword />
          {t("hp.applyDamage")}
        </Button>
        <Button
          variant="outline"
          className="h-9 text-green-700 dark:text-green-400"
          disabled={!valid || busy}
          onClick={() => void apply(1)}
        >
          <Heart />
          {t("hp.applyHeal")}
        </Button>
        </div>
      </div>
    </section>
  );
}
