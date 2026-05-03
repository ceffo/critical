import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';

export function LeftPanel() {
  return (
    <div className="flex flex-col gap-3 p-3 overflow-y-auto h-full">
      <Card>
        <CardHeader>
          <CardTitle>HP &amp; Buffs</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">HP ledger coming soon.</p>
        </CardContent>
      </Card>
    </div>
  );
}
