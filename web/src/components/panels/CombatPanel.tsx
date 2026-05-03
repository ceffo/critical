import { useState } from 'react';
import { useCombatStore } from '@/stores/combat';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';

const WEAPONS = [
  { id: 'longsword', name: 'Longsword (1d8)' },
  { id: 'shortbow', name: 'Shortbow (1d6)' },
];

export function CombatPanel() {
  const { lastResult, isRolling, rollAttack, error } = useCombatStore();
  const [weaponId, setWeaponId] = useState('longsword');
  const [targetAc, setTargetAc] = useState(5);

  const handleRoll = () => {
    rollAttack(weaponId, targetAc);
  };

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label htmlFor="weapon">Weapon</Label>
          <Select value={weaponId} onValueChange={(val) => setWeaponId(val || '')}>
            <SelectTrigger id="weapon">
              <SelectValue placeholder="Select weapon" />
            </SelectTrigger>
            <SelectContent>
              {WEAPONS.map((w) => (
                <SelectItem key={w.id} value={w.id}>
                  {w.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label htmlFor="targetAc">Target AC</Label>
          <Input
            id="targetAc"
            type="number"
            value={targetAc}
            onChange={(e) => setTargetAc(parseInt(e.target.value) || 0)}
          />
        </div>
      </div>

      <Button className="w-full" onClick={handleRoll} disabled={isRolling}>
        {isRolling ? 'Rolling...' : 'Roll Attack'}
      </Button>

      {error && (
        <div className="p-2 text-sm text-destructive bg-destructive/10 rounded">
          {error}
        </div>
      )}

      {lastResult && (
        <Card className={lastResult.hit ? 'border-green-500/50 bg-green-500/5' : 'border-red-500/50 bg-red-500/5'}>
          <CardHeader className="pb-2">
            <div className="flex justify-between items-center">
              <CardTitle className="text-2xl font-bold">
                {lastResult.hit ? (
                  <span className="text-green-600 dark:text-green-400">HIT</span>
                ) : (
                  <span className="text-red-600 dark:text-red-400">MISS</span>
                )}
                {lastResult.isCrit && <Badge variant="destructive" className="ml-2 animate-bounce">CRIT!</Badge>}
              </CardTitle>
              <div className="text-right">
                <div className="text-xs text-muted-foreground uppercase tracking-wider">Total Damage</div>
                <div className="text-3xl font-black">{lastResult.hit ? lastResult.totalDamage : 0}</div>
              </div>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-2 gap-2 text-sm">
              <div className="bg-muted/50 p-2 rounded">
                <div className="text-xs text-muted-foreground">D20 Roll</div>
                <div className="font-mono text-lg">{lastResult.d20Roll}</div>
              </div>
              <div className="bg-muted/50 p-2 rounded">
                <div className="text-xs text-muted-foreground">Crit Roll</div>
                <div className="font-mono text-lg">{lastResult.d100CritRoll}</div>
              </div>
            </div>

            <div className="space-y-1">
              <div className="text-xs font-semibold uppercase text-muted-foreground">Attack Breakdown</div>
              <div className="text-sm font-mono bg-background/50 p-2 rounded border">
                THAC0 {lastResult.baseThac0}
                {lastResult.modifiers.map((m, i) => (
                  <span key={i} className="text-blue-500"> - {m.value} ({m.source})</span>
                ))}
                {' = '}
                <span className="font-bold">{lastResult.effectiveThac0}</span>
                <br />
                Target AC {targetAc} → Need {lastResult.effectiveThac0 - targetAc}+
              </div>
            </div>

            {lastResult.hit && (
              <div className="space-y-1">
                <div className="text-xs font-semibold uppercase text-muted-foreground">Damage Breakdown</div>
                <div className="text-sm font-mono bg-background/50 p-2 rounded border">
                  {lastResult.damageBreakdown}
                  <div className="mt-1 text-[10px] text-muted-foreground">
                    Base: {lastResult.baseDamage} | Spells: x{lastResult.spellsBuff} | Weapon: +{lastResult.weaponBuff} | Multipliers: x{lastResult.multipliers} | STR: +{lastResult.strengthBonus}
                  </div>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
