import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useLogStore } from "@/stores/log";
import { useRollStore, type LastRoll } from "@/stores/roll";
import { formatFactor, formatNumber, formatSigned } from "@/lib/format";
import { cn } from "@/lib/utils";
import { adjustmentChanged, stepAdjustment } from "@/lib/saveAdjust";
import { captionClass, fieldClass } from "./fieldStyles";
import { CastResultView } from "./CastResultView";

/** Full breakdown of the last roll, never a bare number. Values come from the API as-is. */
export function ResultCard() {
  const { t } = useTranslation();
  const last = useRollStore((s) => s.last);

  return (
    <section
      className="flex min-h-0 min-w-0 flex-col rounded-xl border bg-card"
      aria-label={t("result.title")}
    >
      <h2 className="shrink-0 border-b px-4 py-2.5 text-sm font-semibold">
        {t("result.title")}
      </h2>
      <div className="min-h-0 flex-1 overflow-y-auto p-4">
        {!last ? (
          <p className="py-8 text-center text-sm text-muted-foreground">
            {t("result.empty")}
          </p>
        ) : last.kind === "attack" ? (
          <AttackResult roll={last} />
        ) : last.kind === "cast" ? (
          <CastResultView result={last.result} />
        ) : last.kind === "item" ? (
          <ItemResult roll={last} />
        ) : (
          <SaveResult roll={last} />
        )}
      </div>
    </section>
  );
}

function Fact({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="rounded-lg bg-muted/60 px-2.5 py-1.5">
      <dt className={captionClass}>{label}</dt>
      <dd className="font-mono text-lg font-semibold tabular-nums">{children}</dd>
    </div>
  );
}

function Verdict({ ok, yes, no }: { ok: boolean; yes: string; no: string }) {
  return (
    <span
      className={cn(
        "text-3xl font-black uppercase",
        ok ? "text-green-600 dark:text-green-400" : "text-red-600 dark:text-red-400",
      )}
    >
      {ok ? yes : no}
    </span>
  );
}

/** A modifier's label in the player's language (detail = score, weapon, ammo or buff name). */
function modLabel(t: TFunction, m: { kind: string; detail?: string; value: number }): string {
  // The crit term carries the matched crit-table range; the default table (and entries logged
  // before the table existed) have none.
  if (m.kind === "crit" && (!m.detail || m.detail === "placeholder")) {
    return t("result.mods.critPlaceholder");
  }
  return t(`result.mods.${m.kind}`, { detail: m.detail ?? "", value: m.value });
}

function AttackResult({ roll }: { roll: Extract<LastRoll, { kind: "attack" }> }) {
  const { t, i18n } = useTranslation();
  const r = roll.result;
  const heading = r.ammo
    ? t("result.attackHeadingAmmo", { weapon: r.weaponName, ammo: r.ammo.name, ac: r.targetAc })
    : t("result.attackHeading", { weapon: r.weaponName, ac: r.targetAc });
  return (
    <div className="flex flex-col gap-3" data-testid="attack-result">
      <div className="text-sm text-muted-foreground">
        {heading}
        {r.targetSize && <> · {t(`result.sizes.${r.targetSize}`)}</>}
      </div>
      <div className="flex items-end justify-between gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <Verdict ok={r.hit} yes={t("result.hit")} no={t("result.miss")} />
          {r.isCrit && (
            <Badge variant="destructive">
              {r.critPlaceholder || r.critFrom === undefined
                ? t("result.critPlaceholder", { mult: r.critMultiplier })
                : t("result.crit", { mult: r.critMultiplier, from: r.critFrom, to: r.critTo })}
            </Badge>
          )}
        </div>
        <div className="text-right">
          <div className={captionClass}>{t("result.totalDamage")}</div>
          <div
            className="font-mono text-4xl font-black tabular-nums leading-none"
            data-testid="total-damage"
          >
            {r.hit ? r.totalDamage : "–"}
          </div>
        </div>
      </div>

      <dl className="grid grid-cols-3 gap-2">
        <Fact label={t("result.d20")}>{r.d20Roll}</Fact>
        <Fact label={t("result.critRoll")}>{r.d100CritRoll}</Fact>
        <Fact label={t("result.margin")}>{formatSigned(r.hitMargin)}</Fact>
        <Fact label={t("result.baseThac0")}>{r.baseThac0}</Fact>
        <Fact label={t("result.effectiveThac0")}>{r.effectiveThac0}</Fact>
        <Fact label={t("result.hitsAc")}>{r.targetACNeeded}</Fact>
      </dl>

      <div>
        <div className={captionClass}>{t("result.modifiers")}</div>
        {r.modifiers.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("result.noModifiers")}</p>
        ) : (
          <ul className="mt-1 flex flex-col gap-0.5 text-sm" data-testid="to-hit-modifiers">
            {r.modifiers.map((m, i) => (
              <li key={i} className="flex justify-between gap-2">
                <span className="min-w-0 truncate">{modLabel(t, m)}</span>
                <span className="font-mono">{formatSigned(m.value)}</span>
              </li>
            ))}
          </ul>
        )}
      </div>

      {r.hit && (
        <div>
          <div className={captionClass}>{t("result.damage")}</div>
          <dl className="mt-1 grid grid-cols-3 gap-1.5 text-center">
            <Fact label={t("result.damageDice")}>
              {r.damageDice} → {r.baseDamage}
            </Fact>
            <Fact label={t("result.damageSpells")}>{r.spellsBuff}</Fact>
            <Fact label={t("result.damageWeapon")}>{r.weaponBuff}</Fact>
            <Fact label={t("result.damageMultiplier")}>{formatNumber(r.multipliers, i18n.language)}</Fact>
            <Fact label={t("result.damageStrength")}>{r.strengthBonus}</Fact>
            <Fact label={t("result.damageBonus")}>{r.bonusDamage}</Fact>
          </dl>
          {r.damageTerms.length > 0 && (
            <ul className="mt-2 flex flex-col gap-0.5 text-sm" data-testid="damage-terms">
              {r.damageTerms.map((d, i) => (
                <li key={i} className="flex justify-between gap-2">
                  <span className="min-w-0 truncate">{modLabel(t, d)}</span>
                  <span className="font-mono">
                    {d.term === "spells" || d.term === "multiplier"
                      ? formatFactor(d.value, i18n.language)
                      : formatSigned(d.value)}
                  </span>
                </li>
              ))}
            </ul>
          )}
          {r.damageBreakdown && (
            <div className="mt-2 rounded-lg border bg-background px-2.5 py-1.5 font-mono text-sm">
              {r.damageBreakdown}
            </div>
          )}
        </div>
      )}

      {r.ammo && (
        <p className="text-sm text-muted-foreground" data-testid="ammo-left">
          {t("result.ammoLeft", { name: r.ammo.name, count: r.ammo.remaining })}
        </p>
      )}
    </div>
  );
}

function SaveResult({ roll }: { roll: Extract<LastRoll, { kind: "save" }> }) {
  const { t } = useTranslation();
  const r = roll.result;
  return (
    <div className="flex flex-col gap-3" data-testid="save-result">
      <div className="text-sm text-muted-foreground">
        {t("result.saveHeading", { name: t(`saves.categories.${roll.category}`) })}
      </div>
      <Verdict ok={r.hit} yes={t("result.passed")} no={t("result.failed")} />
      <dl className="grid grid-cols-3 gap-2">
        <Fact label={t("result.d20")}>{r.d20Roll}</Fact>
        <Fact label={t("result.target")}>{r.effectiveThac0}</Fact>
        <Fact label={t("result.margin")}>{formatSigned(r.hitMargin)}</Fact>
      </dl>
      {r.modifiers.length > 0 && (
        <ul className="flex flex-col gap-0.5 text-sm" data-testid="save-modifiers">
          {r.modifiers.map((m, i) => (
            <li key={i} className="flex justify-between gap-2">
              <span className="min-w-0 truncate">{modLabel(t, m)}</span>
              <span className="font-mono">{formatSigned(m.value)}</span>
            </li>
          ))}
        </ul>
      )}
      {r.damageBreakdown && (
        <div>
          <div className={captionClass}>{t("result.breakdown")}</div>
          <div className="mt-1 rounded-lg border bg-background px-2.5 py-1.5 font-mono text-sm">
            {r.damageBreakdown}
          </div>
        </div>
      )}
      {r.logEntryId !== undefined && (
        // Keyed by entry so a new save starts from its own adjustment.
        <AdjustSave key={r.logEntryId} result={r} />
      )}
    </div>
  );
}

/**
 * The adjustment of the save on the card (the latest one): − / + around a signed value starting at
 * the current adjustment, an optional reason, Apply. The server recomputes verdict and margin.
 */
function AdjustSave({ result: r }: { result: Extract<LastRoll, { kind: "save" }>["result"] }) {
  const { t } = useTranslation();
  const adjustLastSave = useRollStore((s) => s.adjustLastSave);
  const rolling = useRollStore((s) => s.rolling);
  const voided = useLogStore((s) =>
    s.entries.some((e) => e.id === r.logEntryId && e.status === "voided"),
  );
  const [value, setValue] = useState(r.adjustment ?? 0);
  const [reason, setReason] = useState(r.adjustmentReason ?? "");
  if (voided) return null;
  const changed = adjustmentChanged(r, value, reason);

  return (
    <form
      className="flex flex-col gap-2 rounded-lg border p-2.5"
      aria-label={t("result.adjust.label")}
      onSubmit={(ev) => {
        ev.preventDefault();
        if (changed) void adjustLastSave(value, value === 0 ? "" : reason.trim());
      }}
    >
      <div className={captionClass}>{t("result.adjust.title")}</div>
      <div className="grid grid-cols-[auto_minmax(0,1fr)] items-center gap-2">
        <div className="flex items-center gap-1">
          <Button
            type="button"
            size="sm"
            variant="outline"
            aria-label={t("result.adjust.decrease")}
            onClick={() => setValue((v) => stepAdjustment(v, -1))}
          >
            −
          </Button>
          <output
            className="w-10 text-center font-mono text-lg font-semibold tabular-nums"
            aria-label={t("result.adjust.value")}
            data-testid="save-adjustment"
          >
            {formatSigned(value)}
          </output>
          <Button
            type="button"
            size="sm"
            variant="outline"
            aria-label={t("result.adjust.increase")}
            onClick={() => setValue((v) => stepAdjustment(v, 1))}
          >
            +
          </Button>
        </div>
        <input
          className={fieldClass}
          type="text"
          maxLength={80}
          value={reason}
          aria-label={t("result.adjust.reason")}
          placeholder={t("result.adjust.reason")}
          onChange={(ev) => setReason(ev.target.value)}
        />
      </div>
      <div className="flex justify-end">
        <Button type="submit" size="sm" disabled={rolling || !changed}>
          {t("result.adjust.apply")}
        </Button>
      </div>
    </form>
  );
}

function ItemResult({ roll }: { roll: Extract<LastRoll, { kind: "item" }> }) {
  const { t } = useTranslation();
  const r = roll.result;
  return (
    <div className="flex flex-col gap-3" data-testid="item-result">
      <div className="text-sm text-muted-foreground">{t("result.itemHeading", { name: r.name })}</div>
      {r.dice ? (
        <>
          <div className="flex items-end justify-between gap-2">
            <Badge variant="outline">{r.dice}</Badge>
            <div className="text-right">
              <div className={captionClass}>{t("result.total")}</div>
              <div
                className="font-mono text-4xl font-black tabular-nums leading-none"
                data-testid="item-total"
              >
                {r.total}
              </div>
            </div>
          </div>
          <div>
            <div className={captionClass}>{t("result.rolls")}</div>
            <ul className="mt-1 flex flex-wrap gap-1.5" data-testid="item-rolls">
              {r.rolls.map((v, i) => (
                <li key={i} className="rounded-md bg-muted/60 px-2 py-0.5 font-mono text-sm">
                  {v}
                </li>
              ))}
            </ul>
          </div>
          <div className="rounded-lg border bg-background px-2.5 py-1.5 font-mono text-sm">
            {r.breakdown}
          </div>
        </>
      ) : (
        <p className="text-sm text-muted-foreground">{t("result.noRoll")}</p>
      )}
      {r.save && (
        <p className="text-sm font-medium" data-testid="item-save">
          {t("result.save", { name: t(`saves.categories.${r.save}`, { defaultValue: r.save }) })}
        </p>
      )}
      {r.notes && <p className="text-sm text-muted-foreground">{r.notes}</p>}
      <p className="text-sm text-muted-foreground" data-testid="item-left">
        {t("result.left", { name: r.name, count: r.remaining })}
      </p>
    </div>
  );
}
