import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Heart, Undo2, Sword } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useTableStore } from "@/stores/table";
import { captionClass, fieldClass } from "./fieldStyles";

/** Quick damage/heal entry with a source. The server records it in the action log. */
export function QuickHp() {
  const { t } = useTranslation();
  const { recordHp, undoHp, busy } = useTableStore();
  const [amount, setAmount] = useState("");
  const [source, setSource] = useState("");

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
          disabled={busy}
          onClick={() => void undoHp()}
        >
          <Undo2 />
          {t("hp.undoLast")}
        </Button>
      </div>
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
