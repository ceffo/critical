import { useState, type KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";
import { Brain, Lightning, Moon } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { PSP_ACTIVITIES, type PspActivity } from "@/api/psp";
import { useCharacterStore } from "@/stores/character";
import { useTableStore } from "@/stores/table";
import { cn } from "@/lib/utils";
import { fieldClass, groupHeadClass, labelClass } from "./fieldStyles";

/**
 * The PSP pool (ADR 0043 steps 5 and 6), for a character whose class lists psionics: current over
 * max from the derived character (no fetch of its own; a write's answer sets it, an undo's "psp"
 * key reads the character again), a bar, the hand edit (Enter spends, Shift+Enter recovers, as the
 * HP change) and the recovery over hours of one of Table 6's activities. Nothing for a character
 * with no pool. Every number is the server's.
 */
export function PspPanel() {
  const { t } = useTranslation();
  const pool = useCharacterStore((s) => s.character?.derived?.psp ?? null);
  const { recordPsp, recoverPsp, busy } = useTableStore();
  const [amount, setAmount] = useState("");
  const [source, setSource] = useState("");
  const [hours, setHours] = useState("");
  const [activity, setActivity] = useState<PspActivity>("resting");
  if (!pool) return null;

  const n = Number.parseInt(amount, 10);
  const valid = n > 0;
  const h = Number.parseFloat(hours);
  const validHours = h > 0;
  const pct = pool.max > 0 ? Math.min(100, (pool.current / pool.max) * 100) : 0;

  async function apply(sign: 1 | -1) {
    if (!valid || busy) return;
    const sent = { amount, source };
    if (await recordPsp(sign * n, source.trim())) {
      // The write's answer lands after a while: a field the player changed meanwhile keeps its text.
      setAmount((a) => (a === sent.amount ? "" : a));
      setSource((s) => (s === sent.source ? "" : s));
    }
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key !== "Enter" || e.nativeEvent.isComposing) return;
    e.preventDefault();
    void apply(e.shiftKey ? 1 : -1);
  }

  async function recover() {
    if (!validHours || busy) return;
    const sent = hours;
    if (await recoverPsp(h, activity)) setHours((v) => (v === sent ? "" : v));
  }

  return (
    <section
      className="flex shrink-0 flex-col gap-2"
      role="group"
      aria-label={t("psp.title")}
      data-testid="psp-panel"
    >
      <h2 className={cn(groupHeadClass, "section-label")}>{t("psp.title")}</h2>
      <div className="flex flex-col gap-1">
        <div
          className="flex items-baseline gap-1 tabular-nums"
          data-testid="psp-value"
        >
          <span className="text-title">{pool.current}</span>
          <span className="text-body text-muted-foreground"> / {pool.max}</span>
        </div>
        <div
          className="flex h-2 w-full overflow-hidden rounded bg-raised"
          role="meter"
          aria-label={t("psp.title")}
          aria-valuemin={0}
          aria-valuemax={pool.max}
          aria-valuenow={pool.current}
        >
          <div
            className="h-full bg-info transition-all duration-300"
            style={{ width: `${pct}%` }}
          />
        </div>
      </div>
      <div className="grid grid-cols-[3.5rem_minmax(0,1fr)_minmax(0,1fr)] items-center gap-x-1.5 gap-y-0.5">
        <input
          aria-label={t("psp.amount")}
          className={`${fieldClass} tabular-nums`}
          type="number"
          min={1}
          inputMode="numeric"
          placeholder={t("psp.amountPlaceholder")}
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          onKeyDown={onKeyDown}
        />
        <Button
          className="min-w-0 gap-1 px-2 text-danger"
          disabled={!valid || busy}
          title={t("psp.spendHint")}
          onClick={() => void apply(-1)}
        >
          <Lightning aria-hidden />
          {t("psp.spend")}
        </Button>
        <Button
          className="min-w-0 gap-1 px-2 text-success"
          disabled={!valid || busy}
          title={t("psp.recoverHint")}
          onClick={() => void apply(1)}
        >
          <Brain aria-hidden />
          {t("psp.recover")}
        </Button>
      </div>
      <input
        className={cn(fieldClass, "h-6 text-label")}
        type="text"
        aria-label={t("psp.source")}
        title={t("psp.keysHint")}
        placeholder={t("psp.sourcePlaceholder")}
        value={source}
        onChange={(e) => setSource(e.target.value)}
        onKeyDown={onKeyDown}
      />
      <div className="grid grid-cols-[3.5rem_minmax(0,1fr)_auto] items-end gap-x-1.5">
        <label className="flex min-w-0 flex-col gap-0.5">
          <span className={labelClass}>{t("psp.hours")}</span>
          <input
            className={`${fieldClass} tabular-nums`}
            type="number"
            min={0}
            step="any"
            inputMode="decimal"
            value={hours}
            onChange={(e) => setHours(e.target.value)}
            onKeyDown={(e) => {
              if (e.key !== "Enter" || e.nativeEvent.isComposing) return;
              e.preventDefault();
              void recover();
            }}
          />
        </label>
        <label className="flex min-w-0 flex-col gap-0.5">
          <span className={labelClass}>{t("psp.activityLabel")}</span>
          <select
            className={fieldClass}
            value={activity}
            onChange={(e) => setActivity(e.target.value as PspActivity)}
          >
            {PSP_ACTIVITIES.map((a) => (
              <option key={a} value={a}>
                {t(`psp.activity.${a}`)}
              </option>
            ))}
          </select>
        </label>
        <Button
          className="gap-1 px-2"
          disabled={!validHours || busy}
          title={t("psp.recoverOver")}
          onClick={() => void recover()}
        >
          <Moon aria-hidden />
          {t("psp.recover")}
        </Button>
      </div>
    </section>
  );
}
