import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import type { DefendResult } from "@/api/psionics";
import { searchPowers } from "@/api/powers";
import { formatSigned } from "@/lib/format";
import { localized } from "@/i18n/localized";
import type { Power } from "@/types/powers";
import {
  CHECK_DIE,
  EMPTY_DEFEND,
  defendRequest,
  type DefendDraft,
} from "./defend";

/**
 * The defend form (ADR 0044 step 8), store-free: the DM's attack mode (any attack mode of the
 * catalogue), its one or two rolls with whether each succeeded, the roll to keep, the server's
 * refusal worded in the dialog and the contests once decided. `onDefend` sends the request and
 * answers the result, or the server's refusal as text.
 */
export function DefendForm({
  busy,
  powerName,
  onDefend,
}: {
  busy: boolean;
  /** The name of a power by id, in the player's language. */
  powerName: (id: string) => string;
  onDefend: (
    req: NonNullable<ReturnType<typeof defendRequest>>,
  ) => Promise<{ result: DefendResult } | { refused: string }>;
}) {
  const { t, i18n } = useTranslation();
  const [modes, setModes] = useState<Power[]>([]);
  const [draft, setDraft] = useState<DefendDraft>(EMPTY_DEFEND);
  const [result, setResult] = useState<DefendResult | null>(null);
  const [refused, setRefused] = useState<string | null>(null);
  const req = defendRequest(draft);

  useEffect(() => {
    let live = true;
    searchPowers({ q: "", discipline: "", kind: "attack_mode" }).then(
      (page) => live && setModes(page.items),
      () => undefined,
    );
    return () => {
      live = false;
    };
  }, []);

  const set = (patch: Partial<DefendDraft>) => {
    setDraft((d) => ({ ...d, ...patch }));
    setResult(null);
    setRefused(null);
  };
  const setRoll = (i: number, patch: Partial<DefendDraft["rolls"][0]>) =>
    set({
      rolls: draft.rolls.map((r, j) => (j === i ? { ...r, ...patch } : r)),
    });

  async function send() {
    if (!req || busy) return;
    setRefused(null);
    const out = await onDefend(req);
    if ("result" in out) setResult(out.result);
    else setRefused(out.refused);
  }

  const two = draft.rolls.length === 2;
  return (
    <form
      className="flex flex-col gap-3"
      data-testid="defend-form"
      onSubmit={(e) => {
        e.preventDefault();
        void send();
      }}
    >
      <label className="flex flex-col gap-0.5">
        <span className={labelClass}>{t("powers.defence.attack")}</span>
        <select
          className={fieldClass}
          value={draft.attack}
          onChange={(e) => set({ attack: e.target.value })}
        >
          <option value="">{t("powers.defence.chooseAttack")}</option>
          {modes.map((m) => (
            <option key={m.id} value={m.id}>
              {localized(i18n.language, m.name)}
            </option>
          ))}
        </select>
      </label>

      <label className="flex items-center gap-2 text-body">
        <input
          type="checkbox"
          checked={two}
          onChange={(e) =>
            set({
              rolls: e.target.checked
                ? [...draft.rolls, { roll: "", success: false }]
                : draft.rolls.slice(0, 1),
              keep: null,
            })
          }
        />
        {t("powers.defence.twoChecks")}
      </label>

      <fieldset className="flex flex-col gap-1.5">
        <legend className={labelClass}>{t("powers.defence.rolls")}</legend>
        {draft.rolls.map((r, i) => (
          <div key={i} className="flex flex-wrap items-center gap-2">
            <input
              className={`${fieldClass} !w-20 shrink-0`}
              inputMode="numeric"
              autoComplete="off"
              aria-label={t("powers.defence.rollN", { n: i + 1 })}
              placeholder={`1–${CHECK_DIE}`}
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

      <label className="flex flex-col gap-0.5">
        <span className={labelClass}>{t("powers.defence.keep")}</span>
        <select
          className={fieldClass}
          value={draft.keep === null ? "" : String(draft.keep)}
          onChange={(e) =>
            set({ keep: e.target.value === "" ? null : Number(e.target.value) })
          }
        >
          <option value="">{t("powers.defence.keepNone")}</option>
          {draft.rolls.map((_, i) => (
            <option key={i} value={i}>
              {t("powers.defence.keepRoll", { n: i + 1 })}
            </option>
          ))}
        </select>
      </label>

      {refused && (
        <p
          role="alert"
          className="text-body text-danger"
          data-testid="defend-refused"
        >
          {refused}
        </p>
      )}
      {result && <DefendOutcome result={result} powerName={powerName} />}

      <div className="flex justify-end">
        <Button type="submit" variant="main" disabled={busy || !req}>
          {t("powers.defence.defend")}
        </Button>
      </div>
    </form>
  );
}

/** The contests just decided: who won each, the defence's check, Table 14's cell for the DM. */
export function DefendOutcome({
  result: r,
  powerName,
}: {
  result: DefendResult;
  powerName: (id: string) => string;
}) {
  const { t } = useTranslation();
  return (
    <div
      className="flex flex-col gap-0.5 rounded-md bg-raised px-3 py-2 text-body"
      role="status"
      data-testid="defend-result"
    >
      <span className="text-label text-muted-foreground">
        {r.defence
          ? t("powers.defence.against", { name: powerName(r.defence) })
          : t("powers.defence.noDefence")}
        {r.modifier !== undefined &&
          ` · ${t("powers.defence.tableModifier", { modifier: formatSigned(r.modifier) })}`}
      </span>
      {r.contests.map((c, i) => (
        <span key={i} data-winner={c.winner}>
          {t("powers.defence.contest", { n: i + 1, roll: c.attack.roll })}
          {c.check &&
            ` · ${t("powers.use.rolled", { d20: c.check.d20, target: c.check.target })}${c.fromKept ? ` (${t("powers.defence.fromKept")})` : ""}`}{" "}
          ·{" "}
          <strong>
            {t(
              c.winner === "attacker"
                ? "log.contest.attackerWon"
                : "log.contest.defended",
            )}
          </strong>
        </span>
      ))}
    </div>
  );
}
