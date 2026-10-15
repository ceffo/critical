import { useTranslation } from "react-i18next";
import { ArrowRight } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useItemsStore } from "@/stores/items";
import { useRollStore } from "@/stores/roll";
import { combatItems } from "@/lib/combatItems";

/**
 * Objets tab of the Agir workspace: combat consumables with their behaviour, count and dice, used
 * in one click (the server rolls and logs; the result card shows it). Editing and restocking are in
 * the Inventaire workspace.
 */
export function ItemsTab({ onManage }: { onManage: () => void }) {
  const { t } = useTranslation();
  const items = combatItems(useItemsStore((s) => s.items));
  const { spendItem, rolling } = useRollStore();

  return (
    <div className="flex flex-col gap-3">
      {items.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("items.none")}</p>
      ) : (
        <ul aria-label={t("actions.items.list")} className="flex flex-col gap-1">
          {items.map((i) => (
            <li
              key={i.id}
              className="flex h-10 items-center gap-3 rounded-lg px-3 hover:bg-neu-900"
            >
              <span className="min-w-0 flex-1 truncate text-sm">{i.name}</span>
              <span className="shrink-0 text-sm text-muted-foreground" data-testid="item-count">
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
              <span className="w-16 shrink-0 text-right text-sm text-muted-foreground">
                {i.effectDice}
              </span>
              <Button
                size="sm"
                variant="secondary"
                className="w-20 shrink-0"
                disabled={rolling || i.count === 0}
                aria-label={t(i.effectDice ? "actions.items.rollLabel" : "actions.items.useLabel", {
                  name: i.name,
                })}
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
