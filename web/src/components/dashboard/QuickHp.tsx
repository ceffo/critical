import { useState, type KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";
import { Heart, Sword } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useTableStore } from "@/stores/table";
import { fieldClass } from "./fieldStyles";

/**
 * Damage / healing entry: an amount, Damage and Healing, and the source. Enter in either field
 * applies damage, Shift+Enter healing. The source is optional; the server records the change in
 * the action log.
 */
export function QuickHp() {
  const { t } = useTranslation();
  const { recordHp, busy } = useTableStore();
  const [amount, setAmount] = useState("");
  const [source, setSource] = useState("");
  const n = Number.parseInt(amount, 10);
  const valid = n > 0;

  async function apply(sign: 1 | -1) {
    if (!valid || busy) return;
    if (await recordHp(sign * n, source.trim())) {
      setAmount("");
      setSource("");
    }
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key !== "Enter" || e.nativeEvent.isComposing) return;
    e.preventDefault();
    void apply(e.shiftKey ? 1 : -1);
  }

  return (
    <section
      className="flex shrink-0 flex-col gap-2 rounded-lg bg-neu-900/60 p-3"
      role="group"
      aria-label={t("hp.change")}
    >
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-title">{t("hp.change")}</h2>
      </div>
      <div className="grid grid-cols-[minmax(3.5rem,5rem)_minmax(0,1fr)_minmax(0,1fr)] gap-1">
        <input
          className={`${fieldClass} tabular-nums`}
          type="number"
          min={1}
          inputMode="numeric"
          aria-label={t("hp.amount")}
          placeholder={t("hp.amountPlaceholder")}
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          onKeyDown={onKeyDown}
        />
        <Button
          variant="outline"
          className="min-w-0 border-danger/60 text-danger hover:bg-danger/10 hover:text-danger"
          disabled={!valid || busy}
          title={t("hp.damageHint")}
          onClick={() => void apply(-1)}
        >
          <Sword className="max-[1440px]:hidden" />
          {t("hp.applyDamage")}
        </Button>
        <Button
          className="min-w-0 bg-success text-on-accent hover:bg-success/85"
          disabled={!valid || busy}
          title={t("hp.healHint")}
          onClick={() => void apply(1)}
        >
          <Heart className="max-[1440px]:hidden" />
          {t("hp.applyHeal")}
        </Button>
      </div>
      <input
        className={fieldClass}
        type="text"
        aria-label={t("hp.source")}
        title={t("hp.keysHint")}
        placeholder={t("hp.sourcePlaceholder")}
        value={source}
        onChange={(e) => setSource(e.target.value)}
        onKeyDown={onKeyDown}
      />
    </section>
  );
}
