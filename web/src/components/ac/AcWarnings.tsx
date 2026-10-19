import { useTranslation } from "react-i18next";
import { ClassWarningTag } from "@/components/ClassWarning";
import { itemNamer } from "@/lib/itemPowers";
import { useItemsStore } from "@/stores/items";
import type { AcWarning, ArmourClass } from "@/types/ac";

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
  // One entry per piece: its name once, then one tag per warning.
  const byPiece = new Map<string, AcWarning["code"][]>();
  for (const w of ac.warnings)
    byPiece.set(w.itemId, [...(byPiece.get(w.itemId) ?? []), w.code]);
  return (
    <ul className="flex flex-col gap-1" data-testid="ac-warnings">
      {[...byPiece].map(([itemId, codes]) => (
        <li
          key={itemId}
          className="flex flex-col"
          data-testid="ac-warning-piece"
        >
          <span className="min-w-0 break-words text-body font-medium">
            {named(itemId) ?? itemId}
          </span>
          {codes.map((code) => (
            <ClassWarningTag key={code} code={code} />
          ))}
        </li>
      ))}
    </ul>
  );
}
