import { useTranslation } from "react-i18next";
import { localized } from "@/i18n/localized";
import { ArrowRight } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useItemsStore } from "@/stores/items";
import { useRollStore } from "@/stores/roll";
import { type AbilityRow, abilityRows, limitText } from "@/lib/itemAbilities";
import { equipped } from "@/lib/equipment";

/**
 * Items tab of the Act panel (ADR 0049 step 14): one row per ability of every item that has some,
 * with its limit and Use. Whether a row can be used, and why not, is the server's (`usable`,
 * `reason`); the web derives neither. Editing and restocking are in the Inventory activity. Names
 * wrap, never cut (ADR 0022 step 1).
 */
export function ItemsTab({ onManage }: { onManage: () => void }) {
  const { t, i18n } = useTranslation();
  const rows = abilityRows(useItemsStore((s) => s.items));
  const { activateAbility, rolling } = useRollStore();

  return (
    <div className="flex flex-col gap-3">
      {rows.length === 0 ? (
        <p className="text-body text-muted-foreground">
          {t("actions.items.empty")}
        </p>
      ) : (
        <ul
          aria-label={t("actions.items.list")}
          className="flex flex-col gap-1"
        >
          {rows.map(({ item, ability }) => (
            <AbilityLine
              key={`${item.id}/${ability.id}`}
              row={{ item, ability }}
              lang={i18n.language}
              rolling={rolling}
              onUse={() => void activateAbility(item.id, ability.id)}
            />
          ))}
        </ul>
      )}
      <Button
        variant="ghost"
        className="self-start"
        title={t("actions.items.manageLabel")}
        aria-label={t("actions.items.manageLabel")}
        onClick={onManage}
      >
        {t("actions.items.manage")}
        <ArrowRight />
      </Button>
    </div>
  );
}

function AbilityLine({
  row: { item, ability },
  lang,
  rolling,
  onUse,
}: {
  row: AbilityRow;
  lang: string;
  rolling: boolean;
  onUse: () => void;
}) {
  const { t } = useTranslation();
  const itemName = localized(lang, item.name);
  const abilityName = ability.name ? localized(lang, ability.name) : "";
  const name = abilityName ? `${itemName} · ${abilityName}` : itemName;
  const { kind } = ability.limit;
  const limit = limitText(t, ability, item.count);
  // An unequipped item that could be equipped is tagged, and still used (ADR 0049 step 17).
  const stowed = item.equipState !== "" && !equipped(item);
  const refused = ability.usable === false;
  return (
    <li
      className="grid min-h-8 grid-cols-[minmax(0,1fr)_auto_5rem] items-center gap-3 rounded-lg px-3 py-1 hover:bg-raised"
      data-testid={`ability-${item.id}-${ability.id}`}
    >
      <div className="flex min-w-0 flex-col">
        <span className="text-body break-words">
          {name}
          {stowed && (
            <span
              className="ml-2 text-label text-muted-foreground"
              data-testid="ability-stowed"
            >
              {t("actions.items.notEquipped")}
            </span>
          )}
        </span>
        {refused && ability.reason && (
          <span
            className="text-label text-muted-foreground"
            data-testid="ability-reason"
          >
            {t(`actions.items.reason.${ability.reason}.${kind}`)}
          </span>
        )}
      </div>
      <span
        className="text-right text-body break-words text-muted-foreground tabular-nums"
        data-testid="ability-limit"
      >
        {limit}
      </span>
      <Button
        size="sm"
        className="w-full"
        disabled={rolling || refused}
        aria-label={t("actions.items.useLabel", { name })}
        onClick={onUse}
      >
        {t("actions.items.use")}
      </Button>
    </li>
  );
}
