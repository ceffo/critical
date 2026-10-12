import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { useSpellStore } from "@/stores/spells";
import { spellId, spellName } from "@/i18n/spellName";
import type { SpellSlot } from "@/types/spells";

export function SpellPanel() {
  const { t } = useTranslation();
  const { slots, fetchSlots, castSpell, resetSlots, loading } = useSpellStore();

  useEffect(() => {
    fetchSlots();
  }, [fetchSlots]);

  const slotsByLevel = slots.reduce(
    (acc, slot, index) => {
      const level = slot.level;
      if (!acc[level]) acc[level] = [];
      acc[level].push({ ...slot, index });
      return acc;
    },
    {} as Record<number, (SpellSlot & { index: number })[]>,
  );

  const levels = Object.keys(slotsByLevel).map(Number).sort();

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center px-1">
        <h2 className="text-xl font-bold">{t("spellSlots.title")}</h2>
        <Button
          variant="outline"
          size="sm"
          onClick={resetSlots}
          disabled={loading}
        >
          {t("spellSlots.longRest")}
        </Button>
      </div>

      {levels.length === 0 && !loading && (
        <p className="text-sm text-muted-foreground">{t("spellSlots.none")}</p>
      )}

      {levels.map((level) => {
        const levelSlots = slotsByLevel[level];
        const usedCount = levelSlots.filter((s) => s.used).length;
        const totalCount = levelSlots.length;

        return (
          <Card key={level}>
            <CardHeader className="py-2 px-4 border-b">
              <div className="flex justify-between items-center">
                <CardTitle className="text-sm font-semibold">
                  {t("spellSlots.level", { level })}
                </CardTitle>
                <Badge
                  variant={
                    usedCount === totalCount ? "destructive" : "secondary"
                  }
                >
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
                      ? "bg-muted/50 border-muted opacity-60"
                      : "bg-card hover:bg-accent/50"
                  }`}
                >
                  <span
                    className={`text-sm ${slot.used ? "line-through text-muted-foreground" : "font-medium"}`}
                  >
                    {spellName(
                      t,
                      spellId(slot.prepared_spell_name),
                      slot.prepared_spell_name,
                    )}
                  </span>
                  <Button
                    size="sm"
                    variant={slot.used ? "ghost" : "default"}
                    className="h-7 px-3 text-xs"
                    disabled={slot.used || loading}
                    onClick={() => castSpell(slot.index)}
                  >
                    {slot.used ? t("spellSlots.used") : t("spellSlots.cast")}
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
