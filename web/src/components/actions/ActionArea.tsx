import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { useItemsStore } from "@/stores/items";
import { useSpellStore } from "@/stores/spells";
import { useLogStore } from "@/stores/log";
import { AttackPanel } from "./AttackPanel";
import { SpellsPanel } from "./SpellsPanel";
import { ItemsTab } from "./ItemsTab";
import { SkillsTab } from "./SkillsTab";
import { combatItems } from "@/lib/combatItems";

/**
 * Tabs of the Agir (act) workspace: Weapons, Spells (with the spells left), Items (with how many
 * there are), Skills (proficiency and bard skill checks). Inventory and spell lookup are workspaces of their own in the rail (App.tsx); saving
 * throws live in the Moi column.
 */
const TABS = ["weapons", "spells", "items", "skills"] as const;
type TabId = (typeof TABS)[number];

export function ActionArea({
  onOpenInventory,
}: {
  onOpenInventory: () => void;
}) {
  const { t } = useTranslation();
  const [active, setActive] = useState<TabId>("weapons");
  const spellsLeft = useSpellStore(
    (s) => s.slots.filter((slot) => !slot.used).length,
  );
  const itemCount = useItemsStore((s) => combatItems(s.items).length);
  const fetchSlots = useSpellStore((s) => s.fetchSlots);
  const logEntries = useLogStore((s) => s.entries);
  // The Spells tab shows the spells left before it is opened; a cast, a long rest or a void
  // (any new log state) can change them.
  useEffect(() => {
    void fetchSlots().catch(() => undefined);
  }, [fetchSlots, logEntries]);
  const counts: Partial<Record<TabId, number>> = {
    spells: spellsLeft,
    items: itemCount,
  };

  return (
    <section
      className="flex min-h-0 min-w-0 flex-col rounded-xl bg-card"
      aria-label={t("actions.label")}
    >
      <div
        role="tablist"
        aria-label={t("actions.label")}
        className="flex shrink-0 items-center gap-1 bg-neu-900/50 px-3 pt-2"
      >
        <span className="mr-3 text-body font-semibold">
          {t("shell.workspace.act")}
        </span>
        {TABS.map((id) => (
          <button
            key={id}
            role="tab"
            id={`tab-${id}`}
            aria-selected={active === id}
            aria-controls={`panel-${id}`}
            className={cn(
              "flex items-center gap-2 border-b-2 border-transparent px-3 py-2 text-body font-medium text-muted-foreground transition-colors hover:text-foreground",
              active === id && "border-tint text-foreground",
            )}
            onClick={() => setActive(id)}
          >
            {t(`actions.tabs.${id}`)}
            {counts[id] !== undefined && (
              <span
                aria-hidden
                className="text-label text-muted-foreground"
                data-testid={`tab-count-${id}`}
              >
                {counts[id]}
              </span>
            )}
          </button>
        ))}
      </div>
      <div
        role="tabpanel"
        id={`panel-${active}`}
        aria-labelledby={`tab-${active}`}
        className="min-h-0 flex-1 overflow-y-auto p-3"
      >
        {active === "weapons" && <AttackPanel />}
        {active === "spells" && <SpellsPanel />}
        {active === "items" && <ItemsTab onManage={onOpenInventory} />}
        {active === "skills" && <SkillsTab />}
      </div>
    </section>
  );
}
