import { useTranslation } from "react-i18next";
import { ClassWarningTag } from "@/components/ClassWarning";
import { itemNamer } from "@/lib/itemPowers";
import { useItemsStore } from "@/stores/items";
import type { ArmourClass } from "@/types/ac";

/**
 * The class warnings of the worn armour and shield (ADR 0033 step 20, ADR 0035 item 9): the piece's
 * name and the warning in amber, above the breakdown on the Inventaire's AC card and in the shield's
 * popover. Nothing when there is none.
 */
export function AcWarnings({ ac }: { ac: ArmourClass }) {
  const { i18n } = useTranslation();
  const items = useItemsStore((s) => s.items);
  if (ac.warnings.length === 0) return null;
  const named = itemNamer(items, i18n.language);
  return (
    <ul className="flex flex-col gap-1" data-testid="ac-warnings">
      {ac.warnings.map((w) => (
        <li key={`${w.itemId}-${w.code}`} className="flex flex-col">
          <span className="min-w-0 break-words text-body font-medium">
            {named(w.itemId) ?? w.itemId}
          </span>
          <ClassWarningTag code={w.code} />
        </li>
      ))}
    </ul>
  );
}
