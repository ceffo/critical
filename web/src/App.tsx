import { useEffect, useState, type ReactNode } from "react";
import { VersionBadge } from "@/components/shell/VersionBadge";
import { setReload } from "@/api/http";
import { useTranslation } from "react-i18next";
import { Header } from "@/components/dashboard/Header";
import { InventoryPanel } from "@/components/actions/InventoryPanel";
import { LookupPanel } from "@/components/actions/LookupPanel";
import { QuickHp } from "@/components/dashboard/QuickHp";
import { ChargeCards } from "@/components/dashboard/ChargeCards";
import { Saves } from "@/components/dashboard/Saves";
import { AcShield } from "@/components/ac/AcShield";
import { HpBar } from "@/components/dashboard/HpBar";
import { PspPanel } from "@/components/dashboard/PspPanel";
import { Identity } from "@/components/dashboard/Identity";
import { Notices } from "@/components/dashboard/Notices";
import { DiceComposer } from "@/components/dashboard/DiceComposer";
import { ActStage } from "@/components/shell/ActStage";
import { DatabaseBanner } from "@/components/shell/DatabaseBanner";
import { Rail } from "@/components/shell/Rail";
import { SettingsPopover } from "@/components/shell/SettingsPopover";
import { XpPanel } from "@/components/xp/XpPanel";
import type { Activity } from "@/lib/activities";
import { cn } from "@/lib/utils";
import { useTableStore } from "@/stores/table";
import { useLanguagesStore } from "@/stores/languages";
import { useSpellNamesStore } from "@/stores/spellNames";
import type { Slice } from "@/lib/stateKeys";

/**
 * One activity's stage. It mounts on its first visit and then stays mounted while hidden, so its
 * state (tab, forms, scroll) survives switching and an unvisited one costs nothing at startup
 * (ADR 0009 step 2, kept by ADR 0022 step 2.4).
 */
function Pane({
  id,
  active,
  visited,
  label,
  className,
  children,
}: {
  id: Activity;
  active: Activity;
  visited: ReadonlySet<Activity>;
  label: string;
  className?: string;
  children: ReactNode;
}) {
  if (!visited.has(id)) return null;
  return (
    <section
      aria-label={label}
      data-activity={id}
      hidden={active !== id}
      className={cn(
        "min-h-0 min-w-0 flex-1",
        active === id && "flex flex-col",
        className,
      )}
    >
      {children}
    </section>
  );
}

/**
 * The shell (ADR 0022 step 2, docs/frontend.md "Dashboard Layout"): one screen, no page scroll. On
 * the ground, padded 12 px, every zone is a card 12 px apart: the header card across the top, then
 * the character card on the left edge, the rail on the ground and the stage with the focused
 * activity, then the dice footer card across the bottom. The header, the character card and the
 * footer are always visible: together they show the last result, HP and the round with its effects
 * in every activity. Each zone scrolls on its own.
 */
/** What the page reads on start, and again once the database answers after an uncertain write. */
const SHEET: readonly Slice[] = [
  "character",
  "log",
  "hp",
  "buffs",
  "savedBuffs",
  "round",
  "strength",
  "items",
  "weapons",
  "ac",
  "slots",
  "xp",
];

function App() {
  const { t } = useTranslation();
  const refresh = useTableStore((s) => s.refresh);
  const loadSpellNames = useSpellNamesStore((s) => s.load);
  const loadLanguages = useLanguagesStore((s) => s.load);
  const [activity, setActivity] = useState<Activity>("act");
  const [visited, setVisited] = useState<ReadonlySet<Activity>>(
    () => new Set(["act"]),
  );
  const focus = (a: Activity) => {
    setActivity(a);
    setVisited((v) => (v.has(a) ? v : new Set(v).add(a)));
  };

  useEffect(() => {
    // The sheet; the skills and the spellbook are read by their panels once shown.
    const load = () =>
      Promise.all([refresh(SHEET), loadSpellNames(), loadLanguages()]);
    void load();
    // After a write the database may or may not have saved, the whole sheet is read again before
    // writes are allowed (ADR 0025 step 10.4).
    setReload(load);
    return () => setReload(null);
  }, [refresh, loadSpellNames, loadLanguages]);

  return (
    <div className="flex h-dvh flex-col gap-3 overflow-hidden bg-background p-3">
      <DatabaseBanner />
      <Header />
      <div className="flex min-h-0 flex-1 gap-3">
        <aside
          aria-label={t("shell.character")}
          className="zone-card flex w-[clamp(280px,21vw,384px)] shrink-0 flex-col gap-6 overflow-y-auto px-4 pb-3.5"
          data-testid="character"
        >
          <Identity
            onOpenXp={() => focus("xp")}
            onOpenSkills={() => focus("act")}
            onOpenSpells={() => focus("spells")}
          />
          <div className="flex flex-col gap-3">
            {/* The AC beside the HP (ADR 0035 item 4): smaller, no label, opening its breakdown. */}
            <HpBar
              aside={<AcShield onOpenInventory={() => focus("inventory")} />}
            />
            <QuickHp />
            <PspPanel />
            <ChargeCards />
          </div>
          <Saves />
        </aside>

        <Rail active={activity} onSelect={focus} />

        <main
          className="flex min-h-0 min-w-0 flex-1 flex-col"
          data-testid="stage"
        >
          <Pane
            id="act"
            active={activity}
            visited={visited}
            label={t("shell.activity.act")}
          >
            <ActStage onOpenInventory={() => focus("inventory")} />
          </Pane>
          <Pane
            id="spells"
            active={activity}
            visited={visited}
            label={t("shell.activity.spells")}
          >
            <LookupPanel />
          </Pane>
          <Pane
            id="xp"
            active={activity}
            visited={visited}
            label={t("shell.activity.xp")}
          >
            <XpPanel />
          </Pane>
          <Pane
            id="inventory"
            active={activity}
            visited={visited}
            label={t("shell.activity.inventory")}
          >
            <InventoryPanel />
          </Pane>
        </main>
      </div>
      <footer
        aria-label={t("shell.footer")}
        className="zone-card flex min-w-0 shrink-0 items-center gap-3 px-3 py-2"
        data-testid="dice-footer"
      >
        <SettingsPopover />
        <DiceComposer />
        <VersionBadge className="ml-auto" />
      </footer>
      <Notices />
    </div>
  );
}

export default App;
