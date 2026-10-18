import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Backpack,
  ListChecks,
  Skull,
  Sparkle,
  Sword,
  type Icon,
} from "@phosphor-icons/react";
import { cn } from "@/lib/utils";
import { useItemsStore } from "@/stores/items";
import { useSpellStore } from "@/stores/spells";
import { useLogStore } from "@/stores/log";
import { AttackPanel } from "./AttackPanel";
import { SpellsPanel } from "./SpellsPanel";
import { ItemsTab } from "./ItemsTab";
import { SkillsTab } from "./SkillsTab";
import { TurnUndeadPanel } from "./TurnUndeadPanel";
import { useCharacterStore } from "@/stores/character";
import { combatItems } from "@/lib/combatItems";

/**
 * The act panel of the Act activity (ADR 0022 step 2.5): tabs with icons and no panel title (the
 * rail names the activity): Weapons, Spells (with the spells left), Items (with how many there
 * are), Skills (proficiency and bard skill checks), and Undead for a class that turns them (ADR 0030
 * step 16). Inventory and spell lookup are activities of
 * their own in the rail (App.tsx); saving throws live in the character card.
 */
const TABS = ["weapons", "spells", "items", "skills", "turning"] as const;
type TabId = (typeof TABS)[number];

const TAB_ICONS: Record<TabId, Icon> = {
  weapons: Sword,
  spells: Sparkle,
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
  const [active, setActive] = useState<TabId>("weapons");
  const turns = useCharacterStore((s) => s.character?.derived.turning != null);
  const tabs = TABS.filter((id) => id !== "turning" || turns);
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
    // The focused card of the Act activity (ADR 0026 step 6.1), its tabs in the header band (6.2).
    <section
      className="stage-card flex min-h-0 min-w-0 flex-col"
      aria-label={t("actions.label")}
    >
      <div
        role="tablist"
        aria-label={t("actions.label")}
        className="card-band flex shrink-0 flex-wrap items-center gap-1 px-4"
      >
        {tabs.map((id) => {
          const TabIcon = TAB_ICONS[id];
          return (
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
            </button>
          );
        })}
      </div>
      <div
        role="tabpanel"
        id={`panel-${active}`}
        aria-labelledby={`tab-${active}`}
        className="min-h-0 flex-1 overflow-y-auto px-4 py-3.5"
      >
        {active === "weapons" && <AttackPanel />}
        {active === "spells" && <SpellsPanel />}
        {active === "items" && <ItemsTab onManage={onOpenInventory} />}
        {active === "skills" && <SkillsTab />}
        {active === "turning" && turns && <TurnUndeadPanel />}
      </div>
    </section>
  );
}
