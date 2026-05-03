import { useEffect, useState } from 'react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { useHpStore } from '@/stores/hp';

function HpBar({ current, max }: { current: number; max: number }) {
  const pct = max > 0 ? Math.max(0, Math.min(100, (current / max) * 100)) : 0;
  const color =
    pct > 50 ? 'bg-green-500' : pct > 25 ? 'bg-yellow-400' : 'bg-red-500';
  return (
    <div>
      <div className="flex justify-between text-sm font-mono mb-1">
        <span>{current} / {max}</span>
        <span>{Math.round(pct)}%</span>
      </div>
      <div className="w-full h-4 bg-muted rounded overflow-hidden">
        <div
          className={`h-full transition-all duration-300 ${color}`}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}

export function LeftPanel() {
  const { currentHp, maxHp, events, fetchHp, appendEvent } = useHpStore();

  const [damageInput, setDamageInput] = useState('');
  const [damageSource, setDamageSource] = useState('');
  const [healInput, setHealInput] = useState('');
  const [healSource, setHealSource] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    fetchHp();
  }, [fetchHp]);

  async function handleDamage() {
    const delta = parseInt(damageInput, 10);
    if (isNaN(delta) || delta <= 0 || !damageSource.trim()) return;
    setLoading(true);
    try {
      await appendEvent(-delta, damageSource.trim());
      setDamageInput('');
      setDamageSource('');
    } finally {
      setLoading(false);
    }
  }

  async function handleHeal() {
    const delta = parseInt(healInput, 10);
    if (isNaN(delta) || delta <= 0 || !healSource.trim()) return;
    setLoading(true);
    try {
      await appendEvent(delta, healSource.trim());
      setHealInput('');
      setHealSource('');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex flex-col gap-3 p-3 overflow-y-auto h-full">
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Hit Points</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <HpBar current={currentHp} max={maxHp} />

          {/* Damage entry */}
          <div className="flex flex-col gap-1">
            <label className="text-xs text-muted-foreground uppercase tracking-wide">Damage</label>
            <div className="flex gap-1">
              <input
                type="number"
                min="1"
                placeholder="Amount"
                value={damageInput}
                onChange={(e) => setDamageInput(e.target.value)}
                className="w-16 px-2 py-1 text-sm border rounded bg-background"
              />
              <input
                type="text"
                placeholder="Source"
                value={damageSource}
                onChange={(e) => setDamageSource(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleDamage()}
                className="flex-1 px-2 py-1 text-sm border rounded bg-background"
              />
            </div>
            <Button
              size="sm"
              variant="destructive"
              disabled={loading || !damageInput || !damageSource}
              onClick={handleDamage}
            >
              Apply Damage
            </Button>
          </div>

          {/* Heal entry */}
          <div className="flex flex-col gap-1">
            <label className="text-xs text-muted-foreground uppercase tracking-wide">Heal</label>
            <div className="flex gap-1">
              <input
                type="number"
                min="1"
                placeholder="Amount"
                value={healInput}
                onChange={(e) => setHealInput(e.target.value)}
                className="w-16 px-2 py-1 text-sm border rounded bg-background"
              />
              <input
                type="text"
                placeholder="Source"
                value={healSource}
                onChange={(e) => setHealSource(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleHeal()}
                className="flex-1 px-2 py-1 text-sm border rounded bg-background"
              />
            </div>
            <Button
              size="sm"
              disabled={loading || !healInput || !healSource}
              onClick={handleHeal}
            >
              Apply Heal
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Recent events */}
      {events.length > 0 && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Recent Events</CardTitle>
          </CardHeader>
          <CardContent>
            <ul className="flex flex-col gap-1">
              {events.map((e) => (
                <li key={e.id} className="flex justify-between text-sm">
                  <span className="text-muted-foreground truncate max-w-[140px]">{e.source}</span>
                  <span className={e.delta < 0 ? 'text-red-500' : 'text-green-500'}>
                    {e.delta > 0 ? '+' : ''}{e.delta}
                  </span>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
