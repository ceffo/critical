import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Badge } from "@/components/ui/badge";
import { useRollStore, type LastRoll } from "@/stores/roll";
import { formatFactor, formatSigned } from "@/lib/format";
import { cn } from "@/lib/utils";
import { captionClass } from "./fieldStyles";
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
  return t(`result.mods.${m.kind}`, { detail: m.detail ?? "", value: m.value });
}

function AttackResult({ roll }: { roll: Extract<LastRoll, { kind: "attack" }> }) {
  const { t } = useTranslation();
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
              {t(r.critPlaceholder ? "result.critPlaceholder" : "result.crit", {
                mult: r.critMultiplier,
              })}
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
            <Fact label={t("result.damageMultiplier")}>{r.multipliers}</Fact>
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
                      ? formatFactor(d.value)
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
        <ul className="flex flex-col gap-0.5 text-sm">
          {r.modifiers.map((m, i) => (
            <li key={i} className="flex justify-between gap-2">
              <span className="min-w-0 truncate">{m.source}</span>
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
    </div>
  );
}
