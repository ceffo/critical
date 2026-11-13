import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Backpack,
  Lightning,
  ListChecks,
  Skull,
  Sparkle,
  Sword,
  type Icon,
} from "@phosphor-icons/react";
import { cn } from "@/lib/utils";
import { Tabs, TabsList, TabsTab } from "@/components/ui/tabs";
import { useItemsStore } from "@/stores/items";
import { useSpellStore } from "@/stores/spells";
import { AttackPanel } from "./AttackPanel";
import { SpellsPanel } from "./SpellsPanel";
import { ItemsTab } from "./ItemsTab";
import { SkillsTab } from "./SkillsTab";
import { TurnUndeadPanel } from "./TurnUndeadPanel";
import { PowersTab } from "@/components/powers/tab/PowersTab";
import { useCharacterStore } from "@/stores/character";
import { abilityRows } from "@/lib/itemAbilities";
import { useActFeatures } from "@/lib/actFeatures";

/**
 * The act panel of the Act activity (ADR 0022 step 2.5): tabs with icons and no panel title (the
 * rail names the activity): Weapons, Spells (with the spells left; only for a character with spell slots or a per-day item, ADR 0022 steps 2.4 and 2.5), Powers for a character with known powers (ADR 0047), Items (with how many there
 * are), Skills (proficiency and bard skill checks), and Turning for a class that turns them (ADR 0030
 * step 16). Inventory and spell lookup are activities of
 * their own in the rail (App.tsx); saving throws live in the character card.
 */
const TABS = [
  "weapons",
  "spells",
  "powers",
  "items",
  "skills",
  "turning",
] as const;
type TabId = (typeof TABS)[number];

const TAB_ICONS: Record<TabId, Icon> = {
  weapons: Sword,
  spells: Sparkle,
  powers: Lightning,
  items: Backpack,
  skills: ListChecks,
  turning: Skull,
};

export function ActionArea({
  onOpenInventory,
}: {
  onOpenInventory: () => void;
}) {
  const { t } = useTranslation();
  const [chosen, setActive] = useState<TabId>("weapons");
  const select = (id: TabId) => setActive(id);
  const turns = useCharacterStore((s) => s.character?.derived.turning != null);
  // The Powers tab is for a character with known powers (a psionicist), as Turning is for a class that turns.
  const { hasPowers, showSpellsTab } = useActFeatures();
  const tabs = TABS.filter(
    (id) =>
      (id !== "turning" || turns) &&
      (id !== "powers" || hasPowers) &&
      (id !== "spells" || showSpellsTab),
  );
  // The tab set can shrink under the active tab (a per-day item sold, a level change): back to Weapons.
  const active: TabId = tabs.includes(chosen) ? chosen : "weapons";
  const spellsLeft = useSpellStore(
    (s) => s.slots.filter((slot) => !slot.used).length,
  );
  const itemCount = useItemsStore((s) => abilityRows(s.items).length);
  // The Spells tab shows the spells left before it is opened: the slots are read with the sheet,
  // and again by the action module when a write names them.
  const counts: Partial<Record<TabId, number>> = {
    spells: spellsLeft,
    items: itemCount,
  };

  return (
    // The focused card of the Act activity (ADR 0026 step 6.1), its tabs in the header band (6.2).
    <Tabs
      value={active}
      onValueChange={(id) => select(id as TabId)}
      render={<section />}
      className="stage-card flex min-h-0 min-w-0 flex-col"
      aria-label={t("actions.label")}
    >
      <TabsList
        aria-label={t("actions.label")}
        className="card-band flex shrink-0 flex-wrap items-center gap-1 px-4"
      >
        {tabs.map((id) => {
          const TabIcon = TAB_ICONS[id];
          return (
            <TabsTab
              key={id}
              value={id}
              id={`tab-${id}`}
              aria-controls={`panel-${id}`}
              className={cn(
                "flex items-center gap-2 border-b-2 border-transparent px-3 py-2 text-body font-medium text-muted-foreground transition-colors hover:text-foreground",
                active === id && "border-tint text-foreground",
              )}
            >
              <TabIcon
                aria-hidden
                weight={active === id ? "fill" : "regular"}
                className="size-4"
              />
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
            </TabsTab>
          );
        })}
      </TabsList>
      <div
        role="tabpanel"
        id={`panel-${active}`}
        aria-labelledby={`tab-${active}`}
        className="min-h-0 flex-1 overflow-y-auto px-4 py-3.5"
      >
        {active === "weapons" && <AttackPanel />}
        {active === "spells" && <SpellsPanel />}
        {active === "powers" && hasPowers && <PowersTab />}
        {active === "items" && <ItemsTab onManage={onOpenInventory} />}
        {active === "skills" && <SkillsTab />}
        {active === "turning" && turns && <TurnUndeadPanel />}
      </div>
    </Tabs>
  );
}
