import { LeftPanel } from '@/components/panels/LeftPanel';
import { CenterPanel } from '@/components/panels/CenterPanel';
import { RightPanel } from '@/components/panels/RightPanel';

function App() {
  return (
    <div className="grid grid-cols-[280px_1fr_240px] h-screen overflow-hidden bg-background">
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
  );
}

export default App;
