import { useTranslation } from "react-i18next";
import { ArrowRight } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useItemsStore } from "@/stores/items";
import { useRollStore } from "@/stores/roll";
import { combatItems } from "@/lib/combatItems";

/**
 * Items tab of the Act panel (ADR 0022 step 2.5): combat consumables with their behaviour, count and dice, used
 * in one click (the server rolls and logs; the result card shows it). Editing and restocking are in
 * the Inventory activity. Names wrap, never cut (step 1).
 */
export function ItemsTab({ onManage }: { onManage: () => void }) {
  const { t } = useTranslation();
  const items = combatItems(useItemsStore((s) => s.items));
  const { spendItem, rolling } = useRollStore();

  return (
    <div className="flex flex-col gap-3">
      {items.length === 0 ? (
        <p className="text-body text-muted-foreground">{t("items.none")}</p>
      ) : (
        <ul
          aria-label={t("actions.items.list")}
          className="flex flex-col gap-1"
        >
          {items.map((i) => (
            <li
              key={i.id}
              className="grid min-h-8 grid-cols-[minmax(0,1fr)_auto_4rem_5rem] items-center gap-3 rounded-lg px-3 py-1 hover:bg-neu-900"
            >
              <span className="text-body break-words">{i.name}</span>
              <span
                className="text-right text-body whitespace-nowrap text-muted-foreground tabular-nums"
                data-testid="item-count"
              >
                {i.behaviour === "daily"
                  ? t("actions.items.dailyCount", {
                      behaviour: t(`items.behaviour.${i.behaviour}`),
                      count: i.count,
                      max: i.maxCount,
                    })
                  : t("actions.items.count", {
                      behaviour: t(`items.behaviour.${i.behaviour}`),
                      count: i.count,
                    })}
              </span>
              <span className="text-right text-body text-muted-foreground tabular-nums">
                {i.effectDice}
              </span>
              <Button
                size="sm"
                variant="secondary"
                className="w-full"
                disabled={rolling || i.count === 0}
                aria-label={t(
                  i.effectDice
                    ? "actions.items.rollLabel"
                    : "actions.items.useLabel",
                  {
                    name: i.name,
                  },
                )}
                onClick={() => void spendItem(i.id)}
              >
                {t(i.effectDice ? "actions.items.roll" : "actions.items.use")}
              </Button>
            </li>
          ))}
        </ul>
      )}
      <Button variant="ghost" className="self-start" onClick={onManage}>
        {t("actions.items.manage")}
        <ArrowRight />
      </Button>
    </div>
  );
}
