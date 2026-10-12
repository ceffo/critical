import { useEffect } from "react";
import { Header } from "@/components/dashboard/Header";
import { ActionArea } from "@/components/actions/ActionArea";
import { ResultCard } from "@/components/dashboard/ResultCard";
import { QuickHp } from "@/components/dashboard/QuickHp";
import { CombatLog } from "@/components/dashboard/CombatLog";
import { Notices } from "@/components/dashboard/Notices";
import { useCharacterStore } from "@/stores/character";
import { useTableStore } from "@/stores/table";

/**
 * Round-centric dashboard (docs/frontend.md, "Dashboard Layout"): persistent header,
 * then act (action area) → see the result (result card) → record/fix (HP entry + combat log).
 */
function App() {
  const loadCharacter = useCharacterStore((s) => s.loadCharacter);
  const refresh = useTableStore((s) => s.refresh);

  useEffect(() => {
    void loadCharacter();
    void refresh();
  }, [loadCharacter, refresh]);

  return (
    <div className="flex h-screen flex-col overflow-hidden bg-background">
      <Header />
      <main className="grid min-h-0 flex-1 grid-cols-[minmax(0,1fr)_minmax(0,380px)_minmax(0,420px)] gap-3 p-3">
        <ActionArea />
        <ResultCard />
        <div className="flex min-h-0 min-w-0 flex-col gap-3">
          <QuickHp />
          <CombatLog />
        </div>
      </main>
      <Notices />
    </div>
  );
}

export default App;
