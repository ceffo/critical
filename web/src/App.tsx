import { useEffect } from 'react';
import { LeftPanel } from '@/components/panels/LeftPanel';
import { CenterPanel } from '@/components/panels/CenterPanel';
import { RightPanel } from '@/components/panels/RightPanel';
import { useCharacterStore } from '@/stores/character';

function App() {
  const { character, loadCharacter } = useCharacterStore();

  useEffect(() => {
    loadCharacter();
  }, [loadCharacter]);

  return (
    <div className="flex flex-col h-screen overflow-hidden bg-background">
      {/* Top Header - Persistent Stats */}
      <header className="border-b bg-card px-4 py-2 flex items-center justify-between shrink-0">
        <div className="flex items-center gap-4">
          <h1 className="text-lg font-black tracking-tighter uppercase">{character?.name || 'Loading...'}</h1>
          <div className="flex items-center gap-1">
            <span className="text-[10px] font-bold text-muted-foreground uppercase">Level</span>
            <span className="text-sm font-black">{character?.level}</span>
          </div>
        </div>
        
        <div className="flex gap-4">
          {character && [
            { l: 'STR', v: character.str },
            { l: 'DEX', v: character.dex },
            { l: 'CON', v: character.con },
            { l: 'INT', v: character.int },
            { l: 'WIS', v: character.wis },
            { l: 'CHA', v: character.cha },
          ].map(a => (
            <div key={a.l} className="flex flex-col items-center leading-none">
              <span className="text-[8px] font-bold text-muted-foreground">{a.l}</span>
              <span className="text-sm font-black">{a.v}</span>
            </div>
          ))}
        </div>

        <div className="flex items-center gap-2">
           <div className="text-[10px] font-bold text-muted-foreground uppercase">THAC0</div>
           <div className="text-sm font-black">{character?.thac0Base}</div>
        </div>
      </header>

      <div className="flex-1 grid grid-cols-[280px_1fr_240px] overflow-hidden">
        <aside className="border-r overflow-hidden">
          <LeftPanel />
        </aside>
        <main className="overflow-hidden">
          <CenterPanel />
        </main>
        <aside className="border-l overflow-hidden">
          <RightPanel />
        </aside>
      </div>
    </div>
  );
}

export default App;
