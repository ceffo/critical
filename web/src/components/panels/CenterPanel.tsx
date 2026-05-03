import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { CombatPanel } from './CombatPanel';
import { SpellPanel } from './SpellPanel';

import { CharacterPanel } from './CharacterPanel';

type Tab = 'Combat' | 'Spells' | 'Character';
const TABS: Tab[] = ['Combat', 'Spells', 'Character'];

export function CenterPanel() {
  const [activeTab, setActiveTab] = useState<Tab>('Combat');

  return (
    <div className="flex flex-col h-full">
      <div className="flex gap-1 border-b px-3 pt-2 pb-0 shrink-0 overflow-x-auto scrollbar-none">
        {TABS.map((tab) => (
          <Button
            key={tab}
            variant={activeTab === tab ? 'default' : 'ghost'}
            className={activeTab === tab ? 'bg-primary text-primary-foreground rounded-t-lg rounded-b-none' : 'rounded-t-lg rounded-b-none'}
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
        ) : activeTab === 'Spells' ? (
          <SpellPanel />
        ) : (
          <CharacterPanel />
        )}
      </div>
    </div>
  );
}
