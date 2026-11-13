// The item use entry (itemsvc useRecord, the use endpoint's answer, ADR 0016): a charge or a count
// spent, its effect dice rolled.
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { Flask } from "@phosphor-icons/react";
import { type InventoryNamer, useInventoryName } from "@/i18n/inventoryName";
import type { UseResult } from "@/types/items";
import type { LogEntry } from "@/types/log";
import type { KindView } from "../kindView";
import { dieFaces } from "../dice";
import { plusTerm } from "../words";
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

function itemText(p: UseResult, t: TFunction, names: InventoryNamer): string {
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
  value: (p) => (p.dice ? { text: String(p.total), tone: "ink" } : null),
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
interface Roll {
  dice: string;
  rolls: number[];
  modifier?: number;
  total: number;
  save?: string;
}

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
        rolled.map((e, n) => (
          <div key={n} className="flex flex-col gap-3">
            <div className="flex items-end justify-between gap-3">
              <div className="flex min-w-0 flex-col gap-2">
                <span className={labelClass}>{e.dice}</span>
                <ul className="flex flex-wrap gap-2" data-testid="item-rolls">
                  {e.rolls.map((v, i) => (
                    <DieChip key={i} value={v} faces={dieFaces(e.dice)} />
                  ))}
                </ul>
              </div>
              <Total
                label={t("result.total")}
                value={e.total}
                testId="item-total"
              />
            </div>
            <div className="text-label text-muted-foreground tabular-nums">
              {itemRollText({ ...r, ...e })}
            </div>
            {e.save && (
              <p className="text-body font-medium" data-testid="item-save">
                {t("result.save", {
                  name: t(`saves.categories.${e.save}`, {
                    defaultValue: e.save,
                  }),
                })}
              </p>
            )}
          </div>
        ))
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
