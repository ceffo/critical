import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus, X } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import type { AttackResult } from "@/api/psionics";
import { searchPowers } from "@/api/powers";
import { formatSigned } from "@/lib/format";
import { localized } from "@/i18n/localized";
import type { Power } from "@/types/powers";
import {
  EMPTY_ATTACK,
  MAX_TARGET_LEN,
  attackRequest,
  type AttackDraft,
} from "./attack";
import { checkText } from "./defend";
import { MAX_MODIFIER, MAX_MODIFIERS, MAX_REASON_LEN } from "./powerUse";

/**
 * The attack form (ADR 0044 step 7), store-free: the opponent (a label, its mind open or closed, its
 * level and defence), the DM's roll for each of the two contests when it has a defence, the labelled
 * modifiers, the server's refusal worded in the dialog and the answer once made. `onAttack` sends the
 * request and answers the result, or the server's refusal as text.
 */
export function AttackForm({
  busy,
  powerName,
  onAttack,
}: {
  busy: boolean;
  /** The name of the attack mode, in the player's language. */
  powerName: string;
  onAttack: (
    req: NonNullable<ReturnType<typeof attackRequest>>,
  ) => Promise<{ result: AttackResult } | { refused: string }>;
}) {
  const { t, i18n } = useTranslation();
  const [defences, setDefences] = useState<Power[]>([]);
  const [draft, setDraft] = useState<AttackDraft>(EMPTY_ATTACK);
  const [result, setResult] = useState<AttackResult | null>(null);
  const [refused, setRefused] = useState<string | null>(null);
  const req = attackRequest(draft);

  useEffect(() => {
    let live = true;
    searchPowers({ q: "", discipline: "", kind: "defence_mode" }).then(
      (page) => live && setDefences(page.items),
      () => undefined,
    );
    return () => {
      live = false;
    };
  }, []);

  const set = (patch: Partial<AttackDraft>) => {
    setDraft((d) => ({ ...d, ...patch }));
    setResult(null);
    setRefused(null);
  };
  const setRoll = (i: number, patch: Partial<AttackDraft["rolls"][0]>) =>
    set({
      rolls: draft.rolls.map((r, j) => (j === i ? { ...r, ...patch } : r)),
    });
  const setModifier = (
    i: number,
    patch: Partial<AttackDraft["modifiers"][0]>,
  ) =>
    set({
      modifiers: draft.modifiers.map((m, j) =>
        j === i ? { ...m, ...patch } : m,
      ),
    });

  async function send() {
    if (!req || busy) return;
    setRefused(null);
    const out = await onAttack(req);
    if ("result" in out) setResult(out.result);
    else setRefused(out.refused);
  }

  return (
    <form
      className="flex flex-col gap-3"
      data-testid="attack-form"
      onSubmit={(e) => {
        e.preventDefault();
        void send();
      }}
    >
      <label className="flex flex-col gap-0.5">
        <span className={labelClass}>{t("powers.attack.target")}</span>
        <input
          className={fieldClass}
          autoComplete="off"
          maxLength={MAX_TARGET_LEN}
          aria-label={t("powers.attack.target")}
          value={draft.target}
          onChange={(e) => set({ target: e.target.value })}
        />
      </label>

      <div className="flex flex-wrap gap-3">
        <label className="flex min-w-32 flex-1 flex-col gap-0.5">
          <span className={labelClass}>{t("powers.attack.mind")}</span>
          <select
            className={fieldClass}
            value={draft.mind}
            aria-label={t("powers.attack.mind")}
            onChange={(e) => set({ mind: e.target.value as "open" | "closed" })}
          >
            <option value="closed">{t("powers.attack.closed")}</option>
            <option value="open">{t("powers.attack.open")}</option>
          </select>
        </label>
        <label className="flex w-24 flex-col gap-0.5">
          <span className={labelClass}>{t("powers.attack.level")}</span>
          <input
            className={fieldClass}
            inputMode="numeric"
            autoComplete="off"
            aria-label={t("powers.attack.level")}
            value={draft.level}
            onChange={(e) => set({ level: e.target.value })}
          />
        </label>
      </div>

      <label className="flex flex-col gap-0.5">
        <span className={labelClass}>{t("powers.attack.defence")}</span>
        <select
          className={fieldClass}
          value={draft.defence}
          aria-label={t("powers.attack.defence")}
          onChange={(e) => set({ defence: e.target.value })}
        >
          <option value="">{t("powers.attack.noDefence")}</option>
          {defences.map((d) => (
            <option key={d.id} value={d.id}>
              {localized(i18n.language, d.name)}
            </option>
          ))}
        </select>
      </label>

      {draft.defence && (
        <fieldset className="flex flex-col gap-1.5">
          <legend className={labelClass}>{t("powers.defence.rolls")}</legend>
          {draft.rolls.map((r, i) => (
            <div key={i} className="flex flex-wrap items-center gap-2">
              <input
                className={`${fieldClass} !w-20 shrink-0`}
                inputMode="numeric"
                autoComplete="off"
                aria-label={t("powers.defence.rollN", { n: i + 1 })}
                placeholder="1–20"
                value={r.roll}
                onChange={(e) => setRoll(i, { roll: e.target.value })}
              />
              <label className="flex items-center gap-1.5 text-body">
                <input
                  type="checkbox"
                  checked={r.success}
                  onChange={(e) => setRoll(i, { success: e.target.checked })}
                />
                {t("powers.defence.succeeded", { n: i + 1 })}
              </label>
            </div>
          ))}
        </fieldset>
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

      {refused && (
        <p
          role="alert"
          className="text-body text-danger"
          data-testid="attack-refused"
        >
          {refused}
        </p>
      )}
      {result && <AttackOutcome result={result} />}

      <div className="flex justify-end">
        <Button
          type="submit"
          variant="main"
          disabled={busy || !req}
          aria-label={t("powers.attack.attackNamed", { name: powerName })}
        >
          {t("powers.attack.attack")}
        </Button>
      </div>
    </form>
  );
}

/** The attack just made: each check and who won it, the effect to apply, the tangents, the cost. */
export function AttackOutcome({ result: r }: { result: AttackResult }) {
  const { t } = useTranslation();
  return (
    <div
      className="flex flex-col gap-0.5 rounded-md bg-raised px-3 py-2 text-body"
      role="status"
      data-testid="attack-result"
    >
      {r.attackVersusDefence !== undefined && (
        <span className="text-label text-muted-foreground">
          {t("powers.defence.tableModifier", {
            modifier: formatSigned(r.attackVersusDefence),
          })}
        </span>
      )}
      {r.checks.map((c, i) => (
        <span key={i} data-winner={c.winner} data-effect={c.effect ?? false}>
          {t("powers.attack.check", { n: i + 1 })} ·{" "}
          {checkText(c.check, false, t)} ·{" "}
          <strong>
            {t(
              c.winner !== "attacker"
                ? "log.contest.defended"
                : c.effect
                  ? "log.contest.attackerWonEffect"
                  : "log.contest.attackerWon",
            )}
          </strong>
        </span>
      ))}
      <span className="text-label" data-testid="attack-tangents">
        {r.contact
          ? t("log.attackParts.contact")
          : t("powers.attack.tangents", {
              target: r.target,
              from: r.tangentsBefore,
              to: r.tangentsAfter,
            })}
        {r.broken && ` · ${t("log.attackParts.broken", { target: r.broken })}`}
      </span>
      <span className="text-label text-muted-foreground">
        {r.spent < r.cost
          ? t("log.powerParts.halfCost", { spent: r.spent, cost: r.cost })
          : t("powers.use.spent", { amount: formatSigned(-r.spent) })}
      </span>
      {(r.warnings ?? []).map((w) => (
        <span key={w} className="text-label text-danger">
          {t(`powers.use.warning.${w}`, { defaultValue: w })}
        </span>
      ))}
    </div>
  );
}
