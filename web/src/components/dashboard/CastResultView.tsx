import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Badge } from "@/components/ui/badge";
import type { CastResult, DiceRoll } from "@/types/spells";
import { spellName } from "@/i18n/spellName";
import { formatSigned } from "@/lib/format";
import { cn } from "@/lib/utils";
import { splitText } from "@/lib/slotSplit";
import { labelClass } from "./fieldStyles";

/** One die, as a small tile. */
function Die({ sides, value }: { sides: number; value: number }) {
  return (
    <span
      title={`d${sides}`}
      className="inline-flex min-w-7 items-center justify-center rounded-md border bg-muted/60 px-1.5 py-0.5 font-mono text-sm font-semibold tabular-nums"
    >
      {value}
    </span>
  );
}

/** The dice actually rolled, resolved ("10d6", "1d4+7"): the seed formula ("(min(level,10))d6") stays out of the UI. */
function resolved(r: DiceRoll): string {
  if (r.dice.length === 0) return String(r.total);
  const counts = new Map<number, number>();
  for (const d of r.dice) counts.set(d.sides, (counts.get(d.sides) ?? 0) + 1);
  const mod = r.total - r.dice.reduce((sum, d) => sum + d.value, 0);
  const dice = [...counts].map(([sides, n]) => `${n}d${sides}`).join("+");
  return mod === 0 ? dice : `${dice}${formatSigned(mod)}`;
}

/**
 * A roll from its structured fields: the formula, every die, the flat modifier (total minus the dice)
 * and the total. Wraps, never truncates.
 */
function RollView({ roll: r }: { roll: DiceRoll }) {
  const mod = r.total - r.dice.reduce((sum, d) => sum + d.value, 0);
  return (
    <span className="inline-flex flex-wrap items-center gap-1.5 font-mono text-sm">
      <span className="break-all text-muted-foreground" title={r.formula}>{resolved(r)}</span>
      {r.dice.map((d, i) => (
        <Die key={i} sides={d.sides} value={d.value} />
      ))}
      {mod !== 0 && r.dice.length > 0 && <span>{formatSigned(mod)}</span>}
      <span className="font-bold">= {r.total}</span>
    </span>
  );
}

/** An effect label; unknown keys fall back to "key value". */
function fxLabel(t: TFunction, key: string, value: number): string {
  const shown = ["to_hit", "damage", "save", "save_bonus", "initiative", "attackers_to_hit", "attackers_to_hit_first", "ac"].includes(key)
    ? formatSigned(value)
    : String(value);
  return t(`result.cast.fx.${key}`, { value: shown, defaultValue: `${key} ${shown}` });
}

/** What a cast or a received spell did: buff with its rolls, damage rolls, or the text only. */
export function CastResultView({ result: r }: { result: CastResult }) {
  const { t } = useTranslation();
  const name = spellName(t, r.spellId, r.name);
  const heading =
    r.source === "received"
      ? t("result.cast.receivedHeading", { name, casterLevel: r.casterLevel })
      : t("result.cast.castHeading", { name, level: r.level, casterLevel: r.casterLevel });
  const b = r.buff;

  return (
    <div className="flex flex-col gap-3" data-testid="cast-result">
      <div className="text-sm text-muted-foreground">{heading}</div>
      {r.spell?.class && (
        <div className="text-xs text-muted-foreground" data-testid="cast-spell-ident">
          {r.spell.class} · {t("lookup.levelShort", { level: r.spell.level })}
        </div>
      )}

      {b && (
        <div className="flex flex-col gap-2" data-testid="cast-buff">
          <div className="flex items-center gap-2">
            <span className="text-2xl font-black">{b.name}</span>
            <Badge variant={b.applied ? "default" : "secondary"}>
              {b.applied ? t("result.cast.onYou") : t("result.cast.notOnYou")}
            </Badge>
          </div>
          <dl className="grid grid-cols-2 gap-1.5 text-sm">
            {b.durationRoll && (
              <div className="rounded-lg bg-muted/60 px-2.5 py-1.5">
                <dt className={labelClass}>{t("result.cast.rounds")}</dt>
                <dd className="font-mono" data-testid="cast-rounds">
                  <RollView roll={b.durationRoll} />
                </dd>
              </div>
            )}
            {b.chargesRoll && (
              <div className="rounded-lg bg-muted/60 px-2.5 py-1.5">
                <dt className={labelClass}>
                  {b.chargeUnit
                    ? t(`result.cast.units.${b.chargeUnit}`, { defaultValue: t("result.cast.charges") })
                    : t("result.cast.charges")}
                </dt>
                <dd className="font-mono" data-testid="cast-charges">
                  <RollView roll={b.chargesRoll} />
                </dd>
              </div>
            )}
            {b.strGainRoll && (
              <div className="rounded-lg bg-muted/60 px-2.5 py-1.5">
                <dt className={labelClass}>{t("result.cast.strGain")}</dt>
                <dd className="font-mono" data-testid="cast-str-gain">
                  <RollView roll={b.strGainRoll} />
                </dd>
              </div>
            )}
          </dl>
          {Object.keys(b.effects).length > 0 && (
            <div>
              <div className={labelClass}>{t("result.cast.tracked")}</div>
              <ul className="mt-1 flex flex-wrap gap-1">
                {Object.entries(b.effects).map(([k, v]) => (
                  <li key={k}>
                    <Badge variant="outline">{fxLabel(t, k, v)}</Badge>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}

      {r.attack && (
        <div className="flex flex-col gap-2 rounded-lg border p-2.5" data-testid="cast-attack">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span
              data-testid="cast-attack-verdict"
              className={cn(
                "text-3xl font-black uppercase",
                r.attack.hit ? "text-green-600 dark:text-green-400" : "text-red-600 dark:text-red-400",
              )}
            >
              {r.attack.hit ? t("result.cast.attackHit") : t("result.cast.attackMiss")}
            </span>
            <span className="inline-flex items-center gap-1.5 text-sm">
              <span className={labelClass}>d20</span>
              <Die sides={20} value={r.attack.d20Roll} />
            </span>
          </div>
          <p className="text-sm text-muted-foreground">
            {t("result.cast.attackNeeds", {
              needed: r.attack.targetACNeeded,
              ac: r.attack.targetAc,
              thac0: r.attack.effectiveThac0,
            })}
          </p>
        </div>
      )}

      {r.consumedItem && (
        <p className="text-sm" data-testid="cast-item">
          {t("result.cast.itemSpent", { name: r.consumedItem.name, left: r.consumedItem.remaining })}
        </p>
      )}

      {r.extraSlots && r.extraSlots.length > 0 && (
        <p className="text-sm" data-testid="cast-extra-slots">
          {t("result.cast.extraSlots", { slots: splitText(t, r.extraSlots) })}
        </p>
      )}

      {r.damage && (
        <div className="flex flex-col gap-2" data-testid="cast-damage">
          <div className="flex flex-wrap items-end justify-between gap-2">
            <div className="min-w-0 text-sm">
              {t("result.cast.damageRoll", { instances: r.damage.instances, dice: r.damage.rolls[0] ? resolved(r.damage.rolls[0]) : r.damage.dice })}
              {r.damage.rounds > 1 && <>, {t("result.cast.damageRounds", { rounds: r.damage.rounds })}</>}
              {r.damage.type && <> · {r.damage.type}</>}
            </div>
            <div className="text-right">
              <div className={labelClass}>{t("result.cast.damage")}</div>
              <div
                className="font-mono text-5xl font-black tabular-nums leading-none text-red-600 dark:text-red-400"
                data-testid="cast-damage-total"
              >
                {r.damage.total}
              </div>
            </div>
          </div>
          {r.damage.rolls.length > 0 && (
            <ul className="flex flex-col gap-1">
              {r.damage.rolls.map((roll, i) => (
                <li key={i} className="flex flex-wrap items-center gap-x-2 gap-y-1">
                  <span className="text-xs text-muted-foreground">{t("result.cast.instance", { n: i + 1 })}</span>
                  <RollView roll={roll} />
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {r.save && (
        <p className="text-sm" data-testid="cast-save">
          {t("result.cast.save", {
            type: r.save.type,
            effect: t(`result.cast.saveEffects.${r.save.effect}`, { defaultValue: r.save.effect }),
          })}
        </p>
      )}

      {r.textOnly && (
        <p className="rounded-lg border border-dashed px-2.5 py-1.5 text-sm" data-testid="cast-text-only">
          {t(`result.cast.textOnly.${r.textOnly}`)}
        </p>
      )}

      {r.untracked.length > 0 && (
        <div>
          <div className={labelClass}>{t("result.cast.byHand")}</div>
          <ul className="mt-1 flex flex-col gap-0.5 text-sm" data-testid="cast-untracked">
            {r.untracked.map((u, i) => (
              <li key={i}>
                {fxLabel(t, u.key, u.value)}
                {u.condition && <span className="text-muted-foreground"> ({u.condition})</span>}
              </li>
            ))}
          </ul>
        </div>
      )}

      {r.note && (
        <div>
          <div className={labelClass}>{t("result.cast.note")}</div>
          <p className="text-sm">{r.note}</p>
        </div>
      )}
      {r.spell?.description && (
        <details className="group rounded-lg border px-2.5 py-1.5">
          <summary className={cn(labelClass, "cursor-pointer select-none")}>{t("result.cast.text")}</summary>
          <p className="mt-1.5 whitespace-pre-line text-sm text-muted-foreground" data-testid="cast-text">
            {r.spell.description}
          </p>
        </details>
      )}
    </div>
  );
}
