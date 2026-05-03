import { useEffect, useState } from 'react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { useCharacterStore } from '@/stores/character';
import { rollSavingThrow } from '@/api/roll';
import type { RollResult } from '@/types/combat';

export function CharacterPanel() {
  const { character, loadCharacter, loading } = useCharacterStore();
  const [lastResult, setLastResult] = useState<RollResult | null>(null);

  useEffect(() => {
    loadCharacter();
  }, [loadCharacter]);

  if (loading || !character) return <div className="p-4">Loading character...</div>;

  const handleRollSave = async (category: string) => {
    try {
      const res = await rollSavingThrow(category);
      setLastResult(res);
    } catch (e: unknown) {
      alert((e as Error).message);
    }
  };

  const attributes = [
    { label: 'STR', value: character.str },
    { label: 'DEX', value: character.dex },
    { label: 'CON', value: character.con },
    { label: 'INT', value: character.int },
    { label: 'WIS', value: character.wis },
    { label: 'CHA', value: character.cha },
  ];

  const savingThrows = [
    { id: 'para_poison', label: 'Paralyze/Poison', value: character.saveParaPoison },
    { id: 'rod_staff_wand', label: 'Rod/Staff/Wand', value: character.saveRodStaffWand },
    { id: 'petri_poly', label: 'Petrify/Polymorph', value: character.savePetriPoly },
    { id: 'breath', label: 'Breath Weapon', value: character.saveBreath },
    { id: 'spell', label: 'Spells', value: character.saveSpell },
  ];

  return (
    <div className="space-y-4">
      {/* Attributes Grid */}
      <div className="grid grid-cols-3 gap-2">
        {attributes.map((attr) => (
          <Card key={attr.label} className="bg-muted/50">
            <CardContent className="p-2 flex flex-col items-center">
              <span className="text-xs font-bold text-muted-foreground uppercase">{attr.label}</span>
              <span className="text-2xl font-black">{attr.value}</span>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Saving Throws List */}
      <Card>
        <CardHeader className="py-3">
          <CardTitle className="text-sm">Saving Throws</CardTitle>
        </CardHeader>
        <CardContent className="p-0 border-t">
          <ul className="divide-y">
            {savingThrows.map((save) => (
              <li key={save.id} className="flex items-center justify-between p-3 hover:bg-muted/30 transition-colors group">
                <div className="flex flex-col">
                  <span className="text-sm font-medium">{save.label}</span>
                  <span className="text-xs text-muted-foreground italic">Target: {save.value} or better</span>
                </div>
                <div className="flex items-center gap-3">
                  <Badge variant="outline" className="font-mono text-lg px-2">{save.value}</Badge>
                  <Button 
                    size="sm" 
                    variant="secondary"
                    onClick={() => handleRollSave(save.id)}
                    className="opacity-0 group-hover:opacity-100 transition-opacity"
                  >
                    Roll
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        </CardContent>
      </Card>

      {/* Roll Result Display */}
      {lastResult && (
        <Card className={`border-2 ${lastResult.hit ? 'border-green-500 bg-green-50/5' : 'border-red-500 bg-red-50/5'}`}>
          <CardContent className="p-3">
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs font-bold uppercase text-muted-foreground">Result</span>
              <Badge variant={lastResult.hit ? 'default' : 'destructive'} className="uppercase">
                {lastResult.hit ? 'Passed' : 'Failed'}
              </Badge>
            </div>
            <div className="text-center mb-2">
              <div className="text-5xl font-black font-mono tracking-tighter">
                {lastResult.d20Roll}
              </div>
              <div className="text-xs font-medium text-muted-foreground mt-1">
                Margin: {lastResult.hitMargin > 0 ? '+' : ''}{lastResult.hitMargin}
              </div>
            </div>
            <div className="bg-muted p-2 rounded text-[10px] font-mono break-all leading-tight">
              {lastResult.damageBreakdown}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
