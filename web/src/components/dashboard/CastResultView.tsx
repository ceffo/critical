import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Badge } from "@/components/ui/badge";
import type { CastResult, DiceRoll } from "@/types/spells";
import { spellName } from "@/i18n/spellName";
import { formatSigned } from "@/lib/format";
import { captionClass } from "./fieldStyles";

/** "2d4+1: [3, 1] = 6": the formula, every die and the total, as the API rolled them. */
function rollText(r: DiceRoll): string {
  const dice = r.dice.length > 0 ? ` [${r.dice.map((d) => d.value).join(", ")}]` : "";
  return `${r.formula}${dice} = ${r.total}`;
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
                <dt className={captionClass}>{t("result.cast.rounds")}</dt>
                <dd className="font-mono" data-testid="cast-rounds">
                  {rollText(b.durationRoll)}
                </dd>
              </div>
            )}
            {b.chargesRoll && (
              <div className="rounded-lg bg-muted/60 px-2.5 py-1.5">
                <dt className={captionClass}>
                  {b.chargeUnit
                    ? t(`result.cast.units.${b.chargeUnit}`, { defaultValue: t("result.cast.charges") })
                    : t("result.cast.charges")}
                </dt>
                <dd className="font-mono" data-testid="cast-charges">
                  {rollText(b.chargesRoll)}
                </dd>
              </div>
            )}
            {b.strGainRoll && (
              <div className="rounded-lg bg-muted/60 px-2.5 py-1.5">
                <dt className={captionClass}>{t("result.cast.strGain")}</dt>
                <dd className="font-mono" data-testid="cast-str-gain">
                  {rollText(b.strGainRoll)}
                </dd>
              </div>
            )}
          </dl>
          {Object.keys(b.effects).length > 0 && (
            <div>
              <div className={captionClass}>{t("result.cast.tracked")}</div>
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

      {r.damage && (
        <div className="flex flex-col gap-2" data-testid="cast-damage">
          <div className="flex items-end justify-between gap-2">
            <div className="text-sm">
              {t("result.cast.damageRoll", { instances: r.damage.instances, dice: r.damage.dice })}
              {r.damage.rounds > 1 && <>, {t("result.cast.damageRounds", { rounds: r.damage.rounds })}</>}
              {r.damage.type && <> · {r.damage.type}</>}
            </div>
            <div className="text-right">
              <div className={captionClass}>{t("result.cast.damage")}</div>
              <div
                className="font-mono text-4xl font-black tabular-nums leading-none"
                data-testid="cast-damage-total"
              >
                {r.damage.total}
              </div>
            </div>
          </div>
          {r.damage.rolls.length > 0 && (
            <ul className="flex flex-col gap-0.5 font-mono text-sm">
              {r.damage.rolls.map((roll, i) => (
                <li key={i} className="flex justify-between gap-2">
                  <span className="text-muted-foreground">{t("result.cast.instance", { n: i + 1 })}</span>
                  <span className="min-w-0 truncate">{rollText(roll)}</span>
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
          <div className={captionClass}>{t("result.cast.byHand")}</div>
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
          <div className={captionClass}>{t("result.cast.note")}</div>
          <p className="text-sm">{r.note}</p>
        </div>
      )}
      {r.spell?.description && (
        <div>
          <div className={captionClass}>{t("result.cast.text")}</div>
          <p className="whitespace-pre-line text-sm text-muted-foreground" data-testid="cast-text">
            {r.spell.description}
          </p>
        </div>
      )}
    </div>
  );
}
