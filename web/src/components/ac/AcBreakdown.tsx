import { useTranslation } from "react-i18next";
import { useItemsStore } from "@/stores/items";
import { itemNamer } from "@/lib/itemPowers";
import { buffName, useSpellName } from "@/i18n/spellName";
import { acRowView, type AcNamers } from "@/lib/acView";
import { formatAc } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { ArmourClass } from "@/types/ac";

/** The namers of the breakdown's sources: pieces from the possessions, buffs like the buff list. */
function useAcNamers(): AcNamers {
  const { i18n } = useTranslation();
  const items = useItemsStore((s) => s.items);
  const spellName = useSpellName();
  const named = itemNamer(items, i18n.language);
  return {
    item: (id) => named(id) ?? id,
    buff: (b) => buffName(spellName, b),
  };
}

/**
 * The AC breakdown (ADR 0033 step 10, ADR 0035 item 8): one row per term with its source and its
 * value, in the server's order; a row that does not add is muted with its reason; then the total
 * (the derived AC, with the -10 floor named) and, with a body shield, the AC against missiles. The
 * same rows on the character card's popover and the Inventaire's AC card.
 */
export function AcBreakdown({ ac }: { ac: ArmourClass }) {
  const { t } = useTranslation();
  const names = useAcNamers();
  return (
    <dl
      className="grid grid-cols-[auto_minmax(0,1fr)_auto] gap-x-3 gap-y-1.5 text-body"
      data-testid="ac-breakdown"
    >
      {ac.rows.map((row, i) => {
        const v = acRowView(t, row, names);
        return (
          <div
            key={i}
            className={cn("contents", v.off && "text-muted-foreground")}
            data-testid="ac-row"
            data-term={row.term}
            data-off={row.off}
          >
            <dt className={v.off ? "" : "font-medium"}>{v.term}</dt>
            <dd className="min-w-0 break-words text-muted-foreground">
              {v.source}
              {(v.off || v.missile) && (
                <span className="block text-label">{v.off ?? v.missile}</span>
              )}
            </dd>
            <dd
              className={cn("text-right tabular-nums", v.off && "line-through")}
            >
              {v.value}
            </dd>
          </div>
        );
      })}
      <div className="col-span-3 my-0.5 h-px bg-border" aria-hidden />
      <dt className="font-semibold">{t("ac.total")}</dt>
      <dd className="min-w-0 break-words text-label text-muted-foreground">
        {ac.floored ? t("ac.floored") : t("ac.floor")}
        {ac.missile !== undefined && (
          <span className="block">
            {t("ac.missile", { value: formatAc(ac.missile) })}
          </span>
        )}
      </dd>
      <dd
        className="text-right font-semibold tabular-nums"
        data-testid="ac-derived"
      >
        {formatAc(ac.ac)}
      </dd>
    </dl>
  );
}
