import { useState, type KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";
import { Drop, Heart } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useTableStore } from "@/stores/table";
import { cn } from "@/lib/utils";
import { fieldClass, groupHeadClass } from "./fieldStyles";

/**
 * HP change (ADR 0022 step 2.3, ADR 0027 step 5.3), on one line: the amount (no visible label), Damage
 * (`Drop`) and Healing (`Heart`), then the source under it, smaller. Enter in either field
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
      className="flex shrink-0 flex-col gap-2"
      role="group"
      aria-label={t("hp.change")}
    >
      <h2 className={cn(groupHeadClass, "section-label")}>{t("hp.change")}</h2>
      <div className="grid grid-cols-[3.5rem_minmax(0,1fr)_minmax(0,1fr)] items-center gap-x-1.5 gap-y-0.5">
        <input
          aria-label={t("hp.amount")}
          className={`${fieldClass} tabular-nums`}
          type="number"
          min={1}
          inputMode="numeric"
          placeholder={t("hp.amountPlaceholder")}
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          onKeyDown={onKeyDown}
        />
        <Button
          className="min-w-0 gap-1 px-2 text-danger"
          disabled={!valid || busy}
          title={t("hp.damageHint")}
          onClick={() => void apply(-1)}
        >
          <Drop aria-hidden />
          {t("hp.applyDamage")}
        </Button>
        <Button
          className="min-w-0 gap-1 px-2 text-success"
          disabled={!valid || busy}
          title={t("hp.healHint")}
          onClick={() => void apply(1)}
        >
          <Heart aria-hidden />
          {t("hp.applyHeal")}
        </Button>
      </div>
      <input
        className={cn(fieldClass, "h-6 text-label")}
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
