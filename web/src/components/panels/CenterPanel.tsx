import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { CombatPanel } from './CombatPanel';

type Tab = 'Combat' | 'Spells';
const TABS: Tab[] = ['Combat', 'Spells'];

export function CenterPanel() {
  const [activeTab, setActiveTab] = useState<Tab>('Combat');

  return (
    <div className="flex flex-col h-full">
      <div className="flex gap-1 border-b px-3 pt-2 pb-0 shrink-0">
        {TABS.map((tab) => (
          <Button
            key={tab}
            variant={activeTab === tab ? 'default' : 'ghost'}
            size="sm"
            onClick={() => setActiveTab(tab)}
          >
            {tab}
          </Button>
        ))}
      </div>
      <div className="flex-1 p-3 overflow-y-auto">
        {activeTab === 'Combat' ? (
          <CombatPanel />
        ) : (
          <p className="text-sm text-muted-foreground">{activeTab} panel coming soon.</p>
        )}
      </div>
    </div>
  );
}
