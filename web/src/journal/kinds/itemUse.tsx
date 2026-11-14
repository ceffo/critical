// The item use entry (itemsvc useRecord, the use endpoint's answer, ADR 0016): a charge or a count
// spent, its effect dice rolled.
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { Flask } from "@phosphor-icons/react";
import { type InventoryNamer, useInventoryName } from "@/i18n/inventoryName";
import type {
  AbilityEffectResult,
  AppliedBuff,
  UseResult,
} from "@/types/items";
import type { LogEntry } from "@/types/log";
import type { KindView } from "../kindView";
import { dieFaces } from "../dice";
import { powerEffectsText } from "@/lib/itemPowers";
import { counters, plusTerm } from "../words";
import { DieChip, Total, labelClass } from "../parts";

/** The use as recorded, with its entry id. */
export function decodeItemUse(e: LogEntry): UseResult | null {
  const p = e.payload as Partial<UseResult> | null;
  return p && typeof p.name === "string" && typeof p.remaining === "number"
    ? { ...(p as UseResult), logEntryId: e.id }
    : null;
}

/** An item's effect roll: "6d6: 1 + 2 + 3 + 4 + 5 + 6 = 21", "1d8+2: 5 + 2 = 7". */
export function itemRollText(
  r: Pick<UseResult, "dice" | "rolls" | "modifier" | "total">,
): string {
  let expr = r.rolls.join(" + ");
  if (r.modifier) expr += plusTerm(r.modifier);
  return `${r.dice}: ${expr} = ${r.total}`;
}

/** The item's name, and the ability's after it when it has its own ("Wand of lightning · Bolt"). */
function usedName(p: UseResult, names: InventoryNamer): string {
  const item = names.item(p.itemId, p.name);
  const ability = p.abilityName ? names.text(p.abilityName, "") : "";
  return ability ? `${item} · ${ability}` : item;
}

/** What the use's heals took from the HP ledger and what they rolled, summed; null when it healed nothing. */
function healed(p: UseResult): { applied: number; rolled: number } | null {
  const heals = (p.effects ?? []).filter((e) => e.kind === "heal" && e.applied);
  if (heals.length === 0) return null;
  return {
    applied: heals.reduce((n, e) => n + (e.applied?.hpDelta ?? 0), 0),
    rolled: heals.reduce((n, e) => n + e.total, 0),
  };
}

/** The buffs the use put on the list, in effect order. */
function buffsOf(p: UseResult): AppliedBuff[] {
  return (p.effects ?? []).flatMap((e) => (e.buff ? [e.buff] : []));
}

function itemText(p: UseResult, t: TFunction, names: InventoryNamer): string {
  const withBuffs = [
    baseText(p, t, names),
    ...buffsOf(p).map((b) =>
      t("log.item.buff", { name: b.name, counters: counters(t, b.rounds) }),
    ),
  ].join(", ");
  return healText(p, t, withBuffs);
}

function healText(p: UseResult, t: TFunction, base: string): string {
  const h = healed(p);
  if (!h) return base;
  // Nothing regained (already at max HP) is worded plainly, never as a cap.
  const key =
    h.applied === 0
      ? "log.item.healNone"
      : h.applied < h.rolled
        ? "log.item.healCapped"
        : "log.item.heal";
  const heal = t(key, { applied: h.applied, total: h.rolled });
  return `${base}, ${heal}`;
}

function baseText(p: UseResult, t: TFunction, names: InventoryNamer): string {
  const args = {
    name: usedName(p, names),
    dice: p.dice,
    total: p.total,
    left: p.remaining,
  };
  if (p.removed)
    return t(p.dice ? "log.item.rolledRemoved" : "log.item.usedRemoved", args);
  // An unlimited ability has nothing left to count (ADR 0049 step 5).
  if (p.limit === "unlimited")
    return t(
      p.dice ? "log.item.rolledUnlimited" : "log.item.usedUnlimited",
      args,
    );
  return t(p.dice ? "log.item.rolled" : "log.item.used", args);
}

export const itemUseKind: KindView<UseResult, UseResult> = {
  decode: decodeItemUse,
  icon: Flask,
  text: (p, w) => itemText(p, w.t, w.names),
  value: (p) => {
    const h = healed(p);
    if (h) return { text: `+${h.applied}`, tone: "heal" };
    return p.dice ? { text: String(p.total), tone: "ink" } : null;
  },
  rolled: () => true,
  subtitle: (p, w) => usedName(p, w.names),
  short: (p, w, _e, value) => ({
    value,
    what: usedName(p, w.names),
    verdict: null,
  }),
  card: ({ payload }) => <ItemUseCard r={payload} />,
  live: (r) => r,
};

/** One rolled effect: what the card draws (an ability's effect, or the flat dice of an older entry). */
type Roll = Pick<
  AbilityEffectResult,
  | "kind"
  | "dice"
  | "rolls"
  | "modifier"
  | "total"
  | "save"
  | "full"
  | "applied"
  | "buff"
  | "spellId"
  | "cast"
>;

function ItemUseCard({ r }: { r: UseResult }) {
  const { t } = useTranslation();
  const names = useInventoryName();
  const name = usedName(r, names);
  // The effects the use rolled (an ability), else the flat dice of the entry (a use before abilities).
  const rolled: Roll[] = r.effects?.length
    ? r.effects
    : r.dice
      ? [
          {
            kind: "dice" as const,
            dice: r.dice,
            rolls: r.rolls,
            modifier: r.modifier,
            total: r.total,
            save: r.save,
          },
        ]
      : [];
  return (
    <div className="flex flex-col gap-3" data-testid="item-result">
      <div className="text-body text-muted-foreground">
        {t("result.itemHeading", { name })}
      </div>
      {rolled.length > 0 ? (
        rolled.map((e, n) => <EffectBlock key={n} e={e} r={r} />)
      ) : (
        <p className="text-body text-muted-foreground">{t("result.noRoll")}</p>
      )}
      {r.notes && <p className="text-body text-muted-foreground">{r.notes}</p>}
      {r.limit !== "unlimited" && (
        <p className="text-body text-muted-foreground" data-testid="item-left">
          {r.removed
            ? t("result.leftRemoved", { name })
            : t("result.left", { name, count: r.remaining })}
        </p>
      )}
    </div>
  );
}

/** One effect on the card: a dice effect's rolls, or a heal's rolls (none for a full heal) and the HP it moved. */
function EffectBlock({ e, r }: { e: Roll; r: UseResult }) {
  const { t } = useTranslation();
  if (e.kind === "buff" && e.buff) return <BuffBlock b={e.buff} />;
  // The spell effect's own breakdown (damage, heal, buff of the cast) is drawn with its slice; here
  // the card names the spell cast, so a use never shows an empty dice block.
  if (e.kind === "spell") {
    const spell = typeof e.cast?.spell === "string" ? e.cast.spell : e.spellId;
    return (
      <p className="text-body" data-testid="item-spell">
        {t("result.itemSpell", { spell })}
      </p>
    );
  }
  const heal = e.kind === "heal";
  const dice = e.dice ?? "";
  // A heal's headline is what the ledger took, the roll is the working under it.
  const headline = heal && e.applied ? e.applied.hpDelta : e.total;
  return (
    <div
      className="flex flex-col gap-3"
      data-testid={heal ? "item-heal" : undefined}
    >
      <div className="flex items-end justify-between gap-3">
        <div className="flex min-w-0 flex-col gap-2">
          <span className={labelClass}>
            {heal && e.full ? t("result.itemHealFull") : dice}
          </span>
          {e.rolls.length > 0 && (
            <ul className="flex flex-wrap gap-2" data-testid="item-rolls">
              {e.rolls.map((v, i) => (
                <DieChip key={i} value={v} faces={dieFaces(dice)} />
              ))}
            </ul>
          )}
        </div>
        <Total
          label={heal ? t("result.itemHealed") : t("result.total")}
          value={headline}
          testId={heal ? "item-heal-total" : "item-total"}
        />
      </div>
      {dice && (
        <div className="text-label text-muted-foreground tabular-nums">
          {itemRollText({ ...r, ...e, dice })}
        </div>
      )}
      {heal && e.applied && (
        <>
          <p className="text-body text-muted-foreground tabular-nums">
            {t("result.cast.cardHealRange", {
              before: e.applied.hpBefore,
              after: e.applied.hpAfter,
            })}
          </p>
          {!e.full && e.applied.hpDelta > 0 && e.applied.hpDelta < e.total && (
            <p
              className="text-body text-muted-foreground"
              data-testid="item-heal-capped"
            >
              {t("result.cast.cardHealCapped", { total: e.total })}
            </p>
          )}
        </>
      )}
      {e.save && (
        <p className="text-body font-medium" data-testid="item-save">
          {t("result.save", {
            name: t(`saves.categories.${e.save}`, { defaultValue: e.save }),
          })}
        </p>
      )}
    </div>
  );
}

/** A buff effect on the card: the row it put on the list, its rounds and what it changes. */
function BuffBlock({ b }: { b: AppliedBuff }) {
  const { t } = useTranslation();
  const effects = powerEffectsText(t, b.effects);
  return (
    <div className="flex flex-col gap-1" data-testid="item-buff">
      <span className={labelClass}>{t("result.itemBuffed")}</span>
      <p className="text-body font-medium" data-testid="item-buff-name">
        {b.name} · {counters(t, b.rounds)}
      </p>
      {effects && (
        <p
          className="text-body text-muted-foreground"
          data-testid="item-buff-effects"
        >
          {effects}
        </p>
      )}
    </div>
  );
}
