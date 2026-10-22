import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus, X } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import type { PowerCheckResult } from "@/api/psionics";
import { formatSigned } from "@/lib/format";
import type { Power } from "@/types/powers";
import {
  EMPTY_DRAFT,
  MAX_ENTERED_SCORE,
  MAX_MODIFIER,
  MAX_MODIFIERS,
  MAX_REASON_LEN,
  asks,
  refusalOf,
  requestOf,
  type UseDraft,
} from "./powerUse";

/**
 * The use of one known power (ADR 0043 steps 10 to 13), store-free: the labelled modifiers, the
 * inputs a power asks for (a variable cost or maintenance, a score the book gives in its text), the
 * refusals the numbers already settle in plain words, and the last check once made, with the
 * warning of a second power in the round. `onUse` sends the request and answers the check, or the
 * server's refusal as text.
 */
export function UsePowerForm({
  power,
  powerName,
  currentPsp,
  busy,
  onUse,
}: {
  power: Power;
  powerName: string;
  currentPsp: number;
  busy: boolean;
  onUse: (
    req: NonNullable<ReturnType<typeof requestOf>>,
  ) => Promise<{ check: PowerCheckResult } | { refused: string }>;
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState<UseDraft>(EMPTY_DRAFT);
  const [result, setResult] = useState<PowerCheckResult | null>(null);
  const [refused, setRefused] = useState<string | null>(null);
  const need = asks(power);
  const req = requestOf(power, draft);
  const refusal = refusalOf(power, draft, currentPsp);
  const set = (patch: Partial<UseDraft>) => {
    setDraft((d) => ({ ...d, ...patch }));
    setResult(null);
    setRefused(null);
  };
  const setModifier = (i: number, patch: Partial<UseDraft["modifiers"][0]>) =>
    set({
      modifiers: draft.modifiers.map((m, j) =>
        j === i ? { ...m, ...patch } : m,
      ),
    });

  async function use() {
    if (!req || refusal || busy) return;
    setRefused(null);
    const out = await onUse(req);
    if ("check" in out) setResult(out.check);
    else setRefused(out.refused);
  }

  const number = (
    key: "cost" | "maintenanceCost" | "score",
    label: string,
    hint?: string,
  ) => (
    <label className="flex flex-col gap-0.5">
      <span className={labelClass}>{label}</span>
      <input
        className={fieldClass}
        inputMode="numeric"
        autoComplete="off"
        value={draft[key]}
        aria-label={label}
        placeholder={hint}
        onChange={(e) => set({ [key]: e.target.value })}
      />
    </label>
  );

  return (
    <form
      className="flex flex-col gap-3"
      data-testid="use-power"
      onSubmit={(e) => {
        e.preventDefault();
        void use();
      }}
    >
      {need.cost && number("cost", t("powers.use.cost"))}
      {need.maintenanceCost &&
        number("maintenanceCost", t("powers.use.maintenanceCost"))}
      {need.score &&
        number(
          "score",
          t("powers.use.score"),
          t("powers.use.scoreHint", { max: MAX_ENTERED_SCORE }),
        )}

      <fieldset className="flex flex-col gap-1.5">
        <legend className={labelClass}>{t("powers.use.modifiers")}</legend>
        {draft.modifiers.map((m, i) => (
          <div key={i} className="flex items-center gap-1.5">
            <input
              className={`${fieldClass} !w-16 shrink-0`}
              inputMode="numeric"
              autoComplete="off"
              aria-label={t("powers.use.modifierValue", { n: i + 1 })}
              placeholder={`±${MAX_MODIFIER}`}
              value={m.value}
              onChange={(e) => setModifier(i, { value: e.target.value })}
            />
            <input
              className={`${fieldClass} min-w-0 flex-1`}
              autoComplete="off"
              maxLength={MAX_REASON_LEN}
              aria-label={t("powers.use.modifierReason", { n: i + 1 })}
              placeholder={t("powers.use.reason")}
              value={m.reason}
              onChange={(e) => setModifier(i, { reason: e.target.value })}
            />
            <Button
              type="button"
              className="px-2"
              aria-label={t("powers.use.removeModifier", { n: i + 1 })}
              onClick={() =>
                set({ modifiers: draft.modifiers.filter((_, j) => j !== i) })
              }
            >
              <X aria-hidden />
            </Button>
          </div>
        ))}
        <Button
          type="button"
          className="w-fit gap-1 px-2"
          aria-label={t("powers.use.addModifier")}
          disabled={draft.modifiers.length >= MAX_MODIFIERS}
          onClick={() =>
            set({ modifiers: [...draft.modifiers, { value: "", reason: "" }] })
          }
        >
          <Plus aria-hidden />
          {t("powers.addShort")}
        </Button>
      </fieldset>

      {refusal && (
        <p
          role="alert"
          className="text-body text-danger"
          data-testid="use-refusal"
        >
          {t(`apiError.${refusal}`)}
          {refusal === "not_enough_psp" &&
            ` ${t("powers.use.youHave", { count: currentPsp })}`}
        </p>
      )}
      {refused && (
        <p
          role="alert"
          className="text-body text-danger"
          data-testid="use-refused"
        >
          {refused}
        </p>
      )}
      {result && <CheckResult result={result} />}

      <div className="flex justify-end">
        <Button
          type="submit"
          variant="main"
          disabled={busy || !req || refusal !== null}
          aria-label={t("powers.use.useNamed", { name: powerName })}
        >
          {t("powers.use.use")}
        </Button>
      </div>
    </form>
  );
}

/** The check just made: d20 against the target, the verdict, the PSPs spent, the maintenance
 * set up, and the warnings (as the server raised them). */
export function CheckResult({ result: r }: { result: PowerCheckResult }) {
  const { t } = useTranslation();
  return (
    <div
      className="flex flex-col gap-0.5 rounded-md bg-raised px-3 py-2 text-body"
      role="status"
      data-testid="use-result"
    >
      <span>
        {t("powers.use.rolled", { d20: r.d20, target: r.target })} ·{" "}
        <strong data-outcome={r.outcome}>
          {t(`powers.use.outcome.${r.outcome}`)}
        </strong>
      </span>
      <span className="text-label text-muted-foreground">
        {t("powers.use.spent", { amount: formatSigned(-r.spent) })}
        {r.success && r.maintenanceCost
          ? ` · ${t("powers.use.keptUp", {
              cost: r.maintenanceCost,
              count: r.maintenancePeriod ?? 1,
            })}`
          : ""}
      </span>
      {(r.warnings ?? []).map((w) => (
        <span
          key={w}
          className="text-label text-danger"
          data-testid="use-warning"
        >
          {t(`powers.use.warning.${w}`, { defaultValue: w })}
        </span>
      ))}
    </div>
  );
}
