import { useEffect } from 'react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { useBuffsStore } from '@/stores/buffs';
import { ChevronRight } from 'lucide-react';

export function RightPanel() {
  const { round, fetchRound, advanceRound } = useBuffsStore();

  useEffect(() => {
    fetchRound();
  }, [fetchRound]);

  const handleNextRound = async () => {
    const expired = await advanceRound();
    if (expired.length > 0) {
      // Simple alert for now as shadcn Toast is missing in ui/
      alert(`Expired buffs: ${expired.map(b => b.name).join(', ')}`);
    }
  };

  return (
    <div className="flex flex-col gap-3 p-3 overflow-y-auto h-full">
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base text-muted-foreground uppercase tracking-widest">Combat Round</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col items-center gap-4 py-6">
          <div className="text-6xl font-black font-mono">{round}</div>
          <Button size="lg" className="w-full gap-2" onClick={handleNextRound}>
            Next Round <ChevronRight className="h-4 w-4" />
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
