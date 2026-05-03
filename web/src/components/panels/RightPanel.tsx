import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';

export function RightPanel() {
  return (
    <div className="flex flex-col gap-3 p-3 overflow-y-auto h-full">
      <Card>
        <CardHeader>
          <CardTitle>Round Counter</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">Round tracking coming soon.</p>
        </CardContent>
      </Card>
    </div>
  );
}
