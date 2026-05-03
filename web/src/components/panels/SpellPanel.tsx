import { useEffect } from 'react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { useSpellStore } from '@/stores/spells';
import type { SpellSlot } from '@/types/spells';

export function SpellPanel() {
  const { slots, fetchSlots, castSpell, resetSlots, loading } = useSpellStore();

  useEffect(() => {
    fetchSlots();
  }, [fetchSlots]);

  const slotsByLevel = slots.reduce((acc, slot, index) => {
    const level = slot.level;
    if (!acc[level]) acc[level] = [];
    acc[level].push({ ...slot, index });
    return acc;
  }, {} as Record<number, (SpellSlot & { index: number })[]>);

  const levels = Object.keys(slotsByLevel).map(Number).sort();

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center px-1">
        <h2 className="text-xl font-bold">Spell Slots</h2>
        <Button variant="outline" size="sm" onClick={resetSlots} disabled={loading}>
          Long Rest (Reset)
        </Button>
      </div>

      {levels.length === 0 && !loading && (
        <p className="text-sm text-muted-foreground">No spell slots available.</p>
      )}

      {levels.map((level) => {
        const levelSlots = slotsByLevel[level];
        const usedCount = levelSlots.filter(s => s.used).length;
        const totalCount = levelSlots.length;

        return (
          <Card key={level}>
            <CardHeader className="py-2 px-4 border-b">
              <div className="flex justify-between items-center">
                <CardTitle className="text-sm font-semibold">Level {level}</CardTitle>
                <Badge variant={usedCount === totalCount ? "destructive" : "secondary"}>
                  {totalCount - usedCount} / {totalCount}
                </Badge>
              </div>
            </CardHeader>
            <CardContent className="p-2 space-y-1">
              {levelSlots.map((slot) => (
                <div 
                  key={slot.index} 
                  className={`flex justify-between items-center p-2 rounded border transition-colors ${
                    slot.used 
                      ? 'bg-muted/50 border-muted opacity-60' 
                      : 'bg-card hover:bg-accent/50'
                  }`}
                >
                  <span className={`text-sm ${slot.used ? 'line-through text-muted-foreground' : 'font-medium'}`}>
                    {slot.prepared_spell_name}
                  </span>
                  <Button 
                    size="sm" 
                    variant={slot.used ? "ghost" : "default"}
                    className="h-7 px-3 text-xs"
                    disabled={slot.used || loading}
                    onClick={() => castSpell(slot.index)}
                  >
                    {slot.used ? "Used" : "Cast"}
                  </Button>
                </div>
              ))}
            </CardContent>
          </Card>
        );
      })}
    </div>
  );
}
