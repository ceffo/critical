import { useTranslation } from "react-i18next";
import { useInventoryName } from "@/i18n/inventoryName";
import type { TFunction } from "i18next";
import { Badge } from "@/components/ui/badge";
import type {
  CastResult,
  DamagePartSave,
  DiceRoll,
  ExposureResult,
} from "@/types/spells";
import { spellTexts, useSpellName } from "@/i18n/spellName";
import { SpellDescription } from "./SpellDescription";
import { formatSigned } from "@/lib/format";
import { cn } from "@/lib/utils";
import { splitText } from "@/lib/slotSplit";
import { modeName } from "@/lib/spellMode";
import { rollLabel, stepsHeader } from "@/lib/spellDice";
import { labelClass } from "./fieldStyles";
import { ExposureControls } from "./ExposureControls";

/** One die, as a small tile. */
function Die({ sides, value }: { sides: number; value: number }) {
  return (
    <span
      title={`d${sides}`}
      className="chip inline-flex min-w-7 items-center justify-center px-1.5 py-0.5 text-body font-semibold tabular-nums"
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
    <span className="inline-flex flex-wrap items-center gap-2 text-body">
      <span className="break-all text-muted-foreground" title={r.formula}>
        {resolved(r)}
      </span>
      {r.dice.map((d, i) => (
        <Die key={i} sides={d.sides} value={d.value} />
      ))}
      {mod !== 0 && r.dice.length > 0 && <span>{formatSigned(mod)}</span>}
      <span className="font-semibold">= {r.total}</span>
    </span>
  );
}

/** A damage instance made of parts: each part's roll and what a save does to it, then the instance total. */
function PartsView({
  parts,
  saves,
  total,
}: {
  parts: DiceRoll[];
  saves: DamagePartSave[];
  total: number;
}) {
  const { t } = useTranslation();
  return (
    <span
      className="inline-flex flex-wrap items-center gap-x-3 gap-y-1"
      data-testid="cast-damage-parts"
    >
      {parts.map((p, j) => (
        <span key={j} className="inline-flex flex-wrap items-center gap-2">
          {p.dice.length > 0 ? (
            <RollView roll={p} />
          ) : (
            <span className="text-body font-semibold">{p.total}</span>
          )}
          {saves[j] && (
            <span className="text-label text-muted-foreground">
              {t(`result.cast.partSave.${saves[j]}`, {
                defaultValue: saves[j],
              })}
            </span>
          )}
        </span>
      ))}
      <span className="text-body font-semibold">Σ {total}</span>
    </span>
  );
}

/** An effect label; unknown keys fall back to "key value". */
function fxLabel(t: TFunction, key: string, value: number): string {
  const shown = [
    "to_hit",
    "damage",
    "save",
    "save_bonus",
    "initiative",
    "attackers_to_hit",
    "attackers_to_hit_first",
    "ac",
  ].includes(key)
    ? formatSigned(value)
    : String(value);
  return t(`result.cast.fx.${key}`, {
    value: shown,
    defaultValue: `${key} ${shown}`,
  });
}

/** What a cast or a received spell did: buff with its rolls, damage rolls, or the text only. */
export function CastResultView({ result: r }: { result: CastResult }) {
  const { t, i18n } = useTranslation();
  const names = useInventoryName();
  const spellName = useSpellName();
  const name = spellName(r.spellId, r.name);
  const description = r.spell
    ? spellTexts(i18n.language, r.spell).description
    : "";
  const heading =
    r.source === "received"
      ? t("result.cast.receivedHeading", { name, casterLevel: r.casterLevel })
      : r.source === "effect"
        ? t("result.cast.effectHeading", { name, casterLevel: r.casterLevel })
        : t("result.cast.castHeading", {
            name,
            level: r.level,
            casterLevel: r.casterLevel,
          });
  const b = r.buff;

  return (
    <div className="flex flex-col gap-3" data-testid="cast-result">
      <div className="text-body text-muted-foreground">{heading}</div>
      {r.mode && (
        <p className="text-body" data-testid="cast-mode">
          {t("result.cast.mode", { mode: modeName(t, r.spellId, r.mode) })}
        </p>
      )}
      {r.spell?.class && (
        <div
          className="text-label text-muted-foreground"
          data-testid="cast-spell-ident"
        >
          {r.spell.class} · {t("lookup.levelShort", { level: r.spell.level })}
        </div>
      )}

      {b && (
        <div className="flex flex-col gap-2" data-testid="cast-buff">
          <div className="flex items-center gap-2">
            <span className="text-title">
              {/* The buff a cast puts on is the spell's: named from the catalogue. */}
              {spellName(r.spellId, b.name)}
            </span>
            <Badge variant={b.applied ? "chosen" : "default"}>
              {b.applied ? t("result.cast.onYou") : t("result.cast.notOnYou")}
            </Badge>
          </div>
          <dl className="grid grid-cols-2 gap-2 text-body">
            {b.durationRoll && (
              <div className="rounded-lg bg-raised px-3 py-2">
                <dt className={labelClass}>{t("result.cast.rounds")}</dt>
                <dd data-testid="cast-rounds">
                  <RollView roll={b.durationRoll} />
                </dd>
              </div>
            )}
            {b.chargesRoll && (
              <div className="rounded-lg bg-raised px-3 py-2">
                <dt className={labelClass}>
                  {b.chargeUnit
                    ? t(`result.cast.units.${b.chargeUnit}`, {
                        defaultValue: t("result.cast.charges"),
                      })
                    : t("result.cast.charges")}
                </dt>
                <dd data-testid="cast-charges">
                  <RollView roll={b.chargesRoll} />
                </dd>
              </div>
            )}
            {b.strGainRoll && (
              <div className="rounded-lg bg-raised px-3 py-2">
                <dt className={labelClass}>{t("result.cast.strGain")}</dt>
                <dd data-testid="cast-str-gain">
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
                    <Badge>{fxLabel(t, k, v)}</Badge>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}

      {r.attack && (
        <div
          className="flex flex-col gap-2 rounded-lg bg-raised p-3"
          data-testid="cast-attack"
        >
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span
              data-testid="cast-attack-verdict"
              className={cn(
                "text-key",
                r.attack.hit ? "text-success" : "text-danger",
              )}
            >
              {r.attack.hit
                ? t("result.cast.attackHit")
                : t("result.cast.attackMiss")}
            </span>
            <span className="inline-flex items-center gap-2 text-body">
              <span className={labelClass}>d20</span>
              <Die sides={20} value={r.attack.d20Roll} />
            </span>
          </div>
          <p className="text-body text-muted-foreground">
            {t("result.cast.attackNeeds", {
              needed: r.attack.targetACNeeded,
              ac: r.attack.targetAc,
              thac0: r.attack.effectiveThac0,
            })}
          </p>
        </div>
      )}

      {r.consumedItem && (
        <p className="text-body" data-testid="cast-item">
          {t("result.cast.itemSpent", {
            name: names.item(r.consumedItem.itemId, r.consumedItem.name),
            left: r.consumedItem.remaining,
          })}
        </p>
      )}

      {r.extraSlots && r.extraSlots.length > 0 && (
        <p className="text-body" data-testid="cast-extra-slots">
          {t("result.cast.extraSlots", { slots: splitText(t, r.extraSlots) })}
        </p>
      )}

      {r.damage && (
        <div className="flex flex-col gap-2" data-testid="cast-damage">
          <div className="flex flex-wrap items-end justify-between gap-2">
            <div className="min-w-0 text-body">
              {r.damage.stepsPer ? (
                stepsHeader(t, r.damage.stepsPer, r.damage.rolls.length)
              ) : (
                <>
                  {t("result.cast.damageRoll", {
                    instances: r.damage.instances,
                    dice: r.damage.rolls[0]
                      ? resolved(r.damage.rolls[0])
                      : r.damage.dice,
                  })}
                  {r.damage.rounds > 1 && (
                    <>
                      ,{" "}
                      {t("result.cast.damageRounds", {
                        rounds: r.damage.rounds,
                      })}
                    </>
                  )}
                </>
              )}
              {r.damage.type && <> · {r.damage.type}</>}
            </div>
            <div className="text-right">
              <div className={labelClass}>{t("result.cast.damage")}</div>
              <div
                className="text-headline tabular-nums text-danger"
                data-testid="cast-damage-total"
              >
                {r.damage.total}
              </div>
              {r.damage.totalOnSave !== undefined && (
                <div
                  className="text-body tabular-nums text-muted-foreground"
                  data-testid="cast-damage-on-save"
                >
                  {t("result.cast.totalOnSave", {
                    total: r.damage.totalOnSave,
                  })}
                </div>
              )}
            </div>
          </div>
          {r.damage.rolls.length > 0 && (
            <ul className="flex flex-col gap-1">
              {r.damage.rolls.map((roll, i) => (
                <li
                  key={i}
                  className="flex flex-wrap items-center gap-x-2 gap-y-1"
                  data-testid="cast-damage-roll"
                >
                  <span className="text-label text-muted-foreground">
                    {rollLabel(t, r.damage?.stepsPer, roll.step, i)}
                  </span>
                  {roll.parts && r.damage?.parts ? (
                    <PartsView
                      parts={roll.parts}
                      saves={r.damage.parts.map((p) => p.save)}
                      total={roll.total}
                    />
                  ) : roll.step &&
                    roll.dice.length === 0 &&
                    roll.total === 0 ? (
                    <span className="text-body text-muted-foreground">
                      {t("result.cast.noDamage")}
                    </span>
                  ) : (
                    <RollView roll={roll} />
                  )}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {r.exposure && (
        <ExposureControls
          spellId={r.spellId}
          casterLevel={r.casterLevel}
          exposure={r.exposure}
        />
      )}

      {r.save && (
        <p className="text-body" data-testid="cast-save">
          {t("result.cast.save", {
            type: r.save.type,
            effect: t(`result.cast.saveEffects.${r.save.effect}`, {
              defaultValue: r.save.effect,
            }),
          })}
        </p>
      )}

      {r.textOnly && (
        <p
          className="rounded-lg border border-dashed px-3 py-2 text-body"
          data-testid="cast-text-only"
        >
          {t(`result.cast.textOnly.${r.textOnly}`)}
        </p>
      )}

      {r.untracked.length > 0 && (
        <div>
          <div className={labelClass}>{t("result.cast.byHand")}</div>
          <ul
            className="mt-1 flex flex-col gap-1 text-body"
            data-testid="cast-untracked"
          >
            {r.untracked.map((u, i) => (
              <li key={i}>
                {fxLabel(t, u.key, u.value)}
                {u.condition && (
                  <span className="text-muted-foreground">
                    {" "}
                    ({u.condition})
                  </span>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}

      {description && (
        <details className="group rounded-lg border px-3 py-2">
          <summary className={cn(labelClass, "cursor-pointer select-none")}>
            {t("result.cast.text")}
          </summary>
          <SpellDescription
            text={description}
            className="mt-2 text-body text-muted-foreground"
            testId="cast-text"
          />
        </details>
      )}
    </div>
  );
}

/**
 * One round of exposure to a spell's area (ADR 0008 step 16), from the exposure endpoint or its
 * journal entry: the round, the mode, the dice and the total, and whether it came off the Bard's HP
 * or is the DM's. While it is the latest result, the next round of exposure rolls from here (the
 * entry records the exposure served, so a journal entry offers it too).
 */
export function ExposureResultView({
  result: r,
  interactive,
}: {
  result: ExposureResult;
  interactive: boolean;
}) {
  const { t } = useTranslation();
  const spellName = useSpellName();
  const name = spellName(r.spellId, r.name);
  const roll = r.damage.rolls[0];
  const type = r.damage.type
    ? t(`spellDice.types.${r.damage.type}`, { defaultValue: r.damage.type })
    : "";
  return (
    <div className="flex flex-col gap-3" data-testid="exposure-result">
      <div className="text-body text-muted-foreground">
        {t("exposure.heading", {
          name,
          round: r.round,
          casterLevel: r.casterLevel,
        })}
      </div>
      {r.mode && (
        <p className="text-body" data-testid="exposure-mode">
          {t("result.cast.mode", { mode: modeName(t, r.spellId, r.mode) })}
        </p>
      )}
      <div className="flex flex-wrap items-end justify-between gap-2">
        <div
          className="flex min-w-0 flex-wrap items-center gap-2 text-body"
          data-testid="exposure-roll"
        >
          {roll &&
            (roll.parts && r.damage.parts ? (
              <PartsView
                parts={roll.parts}
                saves={r.damage.parts.map((p) => p.save)}
                total={roll.total}
              />
            ) : roll.dice.length > 0 ? (
              <RollView roll={roll} />
            ) : (
              <span className="font-semibold tabular-nums">{roll.total}</span>
            ))}
          {type && <span className="text-muted-foreground">{type}</span>}
        </div>
        <div className="text-right">
          <div className={labelClass}>{t("result.cast.damage")}</div>
          <div
            className="text-headline tabular-nums text-danger"
            data-testid="exposure-total"
          >
            {r.damage.total}
          </div>
        </div>
      </div>
      <div className="flex flex-col gap-1">
        <p
          className={cn("text-body tabular-nums", r.onSelf && "text-danger")}
          data-testid="exposure-target"
        >
          {r.onSelf
            ? t("exposure.onYou", { delta: formatSigned(r.hpDelta) })
            : t("exposure.forDm")}
        </p>
        {r.onSelf && r.hpDelta !== -r.damage.total && (
          <p
            className="text-label text-muted-foreground"
            data-testid="exposure-floored"
          >
            {t("log.floored", { requested: formatSigned(-r.damage.total) })}
          </p>
        )}
      </div>
      {interactive && r.exposure && (
        <ExposureControls
          key={r.entryId}
          spellId={r.spellId}
          casterLevel={r.casterLevel}
          exposure={r.exposure}
          initial={{ mode: r.mode, round: r.round + 1, onSelf: r.onSelf }}
        />
      )}
    </div>
  );
}
