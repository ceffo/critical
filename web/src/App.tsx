import { useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Header } from "@/components/dashboard/Header";
import { ActionArea } from "@/components/actions/ActionArea";
import { InventoryPanel } from "@/components/actions/InventoryPanel";
import { LookupPanel } from "@/components/actions/LookupPanel";
import { ResultCard } from "@/components/dashboard/ResultCard";
import { QuickHp } from "@/components/dashboard/QuickHp";
import { ChargeCards } from "@/components/dashboard/ChargeCards";
import { Saves } from "@/components/dashboard/Saves";
import { CombatLog } from "@/components/dashboard/CombatLog";
import { HpBar } from "@/components/dashboard/HpBar";
import { Identity } from "@/components/dashboard/Identity";
import { Notices } from "@/components/dashboard/Notices";
import { Rail } from "@/components/shell/Rail";
import { XpPanel } from "@/components/xp/XpPanel";
import type { Workspace } from "@/lib/workspaces";
import { cn } from "@/lib/utils";
import { useCharacterStore } from "@/stores/character";
import { useTableStore } from "@/stores/table";
import { useSpellNamesStore } from "@/stores/spellNames";
import { useXpStore } from "@/stores/xp";

/**
 * A workspace mounts on its first visit and then stays mounted while hidden, so its state (tab,
 * forms, scroll) survives switching and an unvisited one costs nothing at startup.
 */
function Pane({
  id,
  active,
  visited,
  label,
  className,
  children,
}: {
  id: Workspace;
  active: Workspace;
  visited: ReadonlySet<Workspace>;
  label: string;
  className?: string;
  children: ReactNode;
}) {
  if (!visited.has(id)) return null;
  return (
    <section
      aria-label={label}
      data-workspace={id}
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
 * Dashboard v3 shell (ADR 0009, docs/frontend.md "Dashboard Layout"): a full-width header region,
 * then rail | workspace | Fil (result + journal, starting about mid-screen, a separator on its left edge) | Moi (identity, HP, absorb, damage entry,
 * saves). Columns have clamped widths and scroll on their own; the page itself never scrolls.
 */
function App() {
  const { t } = useTranslation();
  const loadCharacter = useCharacterStore((s) => s.loadCharacter);
  const refresh = useTableStore((s) => s.refresh);
  const fetchXp = useXpStore((s) => s.fetchXp);
  const loadSpellNames = useSpellNamesStore((s) => s.load);
  const [workspace, setWorkspace] = useState<Workspace>("act");
  const [visited, setVisited] = useState<ReadonlySet<Workspace>>(
    () => new Set(["act"]),
  );
  const open = (w: Workspace) => {
    setWorkspace(w);
    setVisited((v) => (v.has(w) ? v : new Set(v).add(w)));
  };

  useEffect(() => {
    void loadCharacter();
    void refresh();
    void fetchXp();
    void loadSpellNames();
  }, [loadCharacter, refresh, fetchXp, loadSpellNames]);

  return (
    <div className="grid h-dvh grid-rows-[auto_minmax(0,1fr)] overflow-hidden bg-background">
      <Header />
      <div className="grid min-h-0 grid-cols-[clamp(68px,5vw,96px)_minmax(0,1fr)_minmax(340px,calc(50vw_-_clamp(300px,22vw,440px)_-_1.5rem))_clamp(300px,22vw,440px)] gap-3 py-3 pr-3 pl-2">
        <Rail active={workspace} onSelect={open} />

        <div className="flex min-h-0 min-w-0 flex-col" data-testid="workspace">
          <Pane
            id="act"
            active={workspace}
            visited={visited}
            label={t("shell.workspace.act")}
          >
            <ActionArea onOpenInventory={() => open("inventory")} />
          </Pane>
          <Pane
            id="inventory"
            active={workspace}
            visited={visited}
            label={t("shell.workspace.inventory")}
            className="overflow-y-auto"
          >
            <InventoryPanel />
          </Pane>
          <Pane
            id="spells"
            active={workspace}
            visited={visited}
            label={t("shell.workspace.spells")}
          >
            <LookupPanel />
          </Pane>
          <Pane
            id="xp"
            active={workspace}
            visited={visited}
            label={t("shell.workspace.xp")}
          >
            <XpPanel />
          </Pane>
        </div>

        <aside
          aria-label={t("shell.fil")}
          className="flex min-h-0 min-w-0 flex-col gap-3 border-l border-border pl-3"
          data-testid="fil"
        >
          <div className="flex max-h-[60%] min-h-0 shrink-0 flex-col">
            <ResultCard />
          </div>
          <CombatLog />
        </aside>

        <aside
          aria-label={t("shell.moi")}
          className="flex min-h-0 min-w-0 flex-col gap-3 overflow-y-auto rounded-xl bg-card p-3"
          data-testid="moi"
        >
          <Identity onOpenXp={() => open("xp")} />
          <HpBar />
          <ChargeCards />
          <QuickHp />
          <Saves />
        </aside>
      </div>
      <Notices />
    </div>
  );
}

export default App;
