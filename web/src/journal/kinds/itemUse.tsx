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
export function itemRollText(r: UseResult): string {
  let expr = r.rolls.join(" + ");
  if (r.modifier) expr += plusTerm(r.modifier);
  return `${r.dice}: ${expr} = ${r.total}`;
}

function itemText(p: UseResult, t: TFunction, names: InventoryNamer): string {
  const args = {
    name: names.item(p.itemId, p.name),
    dice: p.dice,
    total: p.total,
    left: p.remaining,
  };
  if (p.removed)
    return t(p.dice ? "log.item.rolledRemoved" : "log.item.usedRemoved", args);
  return t(p.dice ? "log.item.rolled" : "log.item.used", args);
}

export const itemUseKind: KindView<UseResult, UseResult> = {
  decode: decodeItemUse,
  icon: Flask,
  text: (p, w) => itemText(p, w.t, w.names),
  value: (p) => (p.dice ? { text: String(p.total), tone: "ink" } : null),
  rolled: () => true,
  subtitle: (p, w) => w.names.item(p.itemId, p.name),
  short: (p, w, _e, value) => ({
    value,
    what: w.names.item(p.itemId, p.name),
    verdict: null,
  }),
  card: ({ payload }) => <ItemUseCard r={payload} />,
  live: (r) => r,
};

function ItemUseCard({ r }: { r: UseResult }) {
  const { t } = useTranslation();
  const name = useInventoryName().item(r.itemId, r.name);
  const faces = r.dice ? dieFaces(r.dice) : null;
  return (
    <div className="flex flex-col gap-3" data-testid="item-result">
      <div className="text-body text-muted-foreground">
        {t("result.itemHeading", { name })}
      </div>
      {r.dice ? (
        <>
          <div className="flex items-end justify-between gap-3">
            <div className="flex min-w-0 flex-col gap-2">
              <span className={labelClass}>{r.dice}</span>
              <ul className="flex flex-wrap gap-2" data-testid="item-rolls">
                {r.rolls.map((v, i) => (
                  <DieChip key={i} value={v} faces={faces} />
                ))}
              </ul>
            </div>
            <Total
              label={t("result.total")}
              value={r.total}
              testId="item-total"
            />
          </div>
          <div className="text-label text-muted-foreground tabular-nums">
            {itemRollText(r)}
          </div>
        </>
      ) : (
        <p className="text-body text-muted-foreground">{t("result.noRoll")}</p>
      )}
      {r.save && (
        <p className="text-body font-medium" data-testid="item-save">
          {t("result.save", {
            name: t(`saves.categories.${r.save}`, { defaultValue: r.save }),
          })}
        </p>
      )}
      {r.notes && <p className="text-body text-muted-foreground">{r.notes}</p>}
      <p className="text-body text-muted-foreground" data-testid="item-left">
        {r.removed
          ? t("result.leftRemoved", { name })
          : t("result.left", { name, count: r.remaining })}
      </p>
    </div>
  );
}
