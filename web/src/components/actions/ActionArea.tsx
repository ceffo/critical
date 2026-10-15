import { useState, type ComponentType } from "react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { AttackPanel } from "./AttackPanel";
import { SpellsPanel } from "./SpellsPanel";
import { SavesPanel } from "./SavesPanel";

/**
 * Tabs of the Agir (act) workspace, one panel component each. Inventory and spell lookup are
 * workspaces of their own in the rail (App.tsx).
 */
const TABS = [
  { id: "attack", Panel: AttackPanel },
  { id: "spells", Panel: SpellsPanel },
  { id: "saves", Panel: SavesPanel },
] as const satisfies readonly { id: string; Panel: ComponentType }[];

type TabId = (typeof TABS)[number]["id"];

export function ActionArea() {
  const { t } = useTranslation();
  const [active, setActive] = useState<TabId>("attack");
  const { Panel } = TABS.find((tab) => tab.id === active) ?? TABS[0];

  return (
    <section
      className="flex min-h-0 min-w-0 flex-col rounded-xl border bg-card"
      aria-label={t("actions.label")}
    >
      <div
        role="tablist"
        aria-label={t("actions.label")}
        className="flex shrink-0 gap-1 border-b px-2 pt-2"
      >
        {TABS.map(({ id }) => (
          <button
            key={id}
            role="tab"
            id={`tab-${id}`}
            aria-selected={active === id}
            aria-controls={`panel-${id}`}
            className={cn(
              "-mb-px rounded-t-lg border border-transparent px-3 py-1.5 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground",
              active === id && "border-border border-b-card bg-card text-foreground",
            )}
            onClick={() => setActive(id)}
          >
            {t(`actions.tabs.${id}`)}
          </button>
        ))}
      </div>
      <div
        role="tabpanel"
        id={`panel-${active}`}
        aria-labelledby={`tab-${active}`}
        className="min-h-0 flex-1 overflow-y-auto p-4"
      >
        <Panel />
      </div>
    </section>
  );
}
