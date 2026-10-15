import { useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Header } from "@/components/dashboard/Header";
import { ActionArea } from "@/components/actions/ActionArea";
import { InventoryPanel } from "@/components/actions/InventoryPanel";
import { LookupPanel } from "@/components/actions/LookupPanel";
import { ResultCard } from "@/components/dashboard/ResultCard";
import { QuickHp } from "@/components/dashboard/QuickHp";
import { CombatLog } from "@/components/dashboard/CombatLog";
import { HpBar } from "@/components/dashboard/HpBar";
import { Identity } from "@/components/dashboard/Identity";
import { Notices } from "@/components/dashboard/Notices";
import { Rail } from "@/components/shell/Rail";
import type { Workspace } from "@/lib/workspaces";
import { cn } from "@/lib/utils";
import { useCharacterStore } from "@/stores/character";
import { useTableStore } from "@/stores/table";

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
      className={cn("min-h-0 min-w-0 flex-1", active === id && "flex flex-col", className)}
    >
      {children}
    </section>
  );
}

/**
 * Dashboard v3 shell (ADR 0009, docs/frontend.md "Dashboard Layout"): a full-width header region,
 * then rail | workspace | Fil (result + journal) | Moi (identity, HP). Columns have clamped widths
 * and scroll on their own; the page itself never scrolls.
 */
function App() {
  const { t } = useTranslation();
  const loadCharacter = useCharacterStore((s) => s.loadCharacter);
  const refresh = useTableStore((s) => s.refresh);
  const [workspace, setWorkspace] = useState<Workspace>("act");
  const [visited, setVisited] = useState<ReadonlySet<Workspace>>(() => new Set(["act"]));
  const open = (w: Workspace) => {
    setWorkspace(w);
    setVisited((v) => (v.has(w) ? v : new Set(v).add(w)));
  };

  useEffect(() => {
    void loadCharacter();
    void refresh();
  }, [loadCharacter, refresh]);

  return (
    <div className="grid h-dvh grid-rows-[auto_minmax(0,1fr)] overflow-hidden bg-background">
      <Header />
      <div className="grid min-h-0 grid-cols-[clamp(68px,5vw,96px)_minmax(0,1fr)_clamp(340px,26vw,520px)_clamp(300px,22vw,440px)] gap-3 py-3 pr-3 pl-1.5">
        <Rail active={workspace} onSelect={open} />

        <div className="flex min-h-0 min-w-0 flex-col" data-testid="workspace">
          <Pane id="act" active={workspace} visited={visited} label={t("shell.workspace.act")}>
            <ActionArea />
          </Pane>
          <Pane
            id="inventory"
            active={workspace}
            visited={visited}
            label={t("shell.workspace.inventory")}
            className="overflow-y-auto rounded-xl bg-card p-4"
          >
            <InventoryPanel />
          </Pane>
          <Pane
            id="spells"
            active={workspace}
            visited={visited}
            label={t("shell.workspace.spells")}
            className="rounded-xl bg-card p-4"
          >
            <LookupPanel />
          </Pane>
        </div>

        <aside
          aria-label={t("shell.fil")}
          className="flex min-h-0 min-w-0 flex-col gap-3"
        >
          <div className="flex max-h-[60%] min-h-0 shrink-0 flex-col">
            <ResultCard />
          </div>
          <CombatLog />
        </aside>

        <aside
          aria-label={t("shell.moi")}
          className="flex min-h-0 min-w-0 flex-col gap-4 overflow-y-auto rounded-xl bg-card p-4"
        >
          <Identity />
          <HpBar />
          <QuickHp />
        </aside>
      </div>
      <Notices />
    </div>
  );
}

export default App;
