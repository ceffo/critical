import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useSpellStore } from "@/stores/spells";
import { useTableStore } from "@/stores/table";
import { spellId, spellName } from "@/i18n/spellName";
import type { SpellSlot } from "@/types/spells";
import { cn } from "@/lib/utils";

/** Today's slots, cast and long rest. Preparation (qki.9) and cast effects (qki.10) replace this. */
export function SpellsPanel() {
  const { t } = useTranslation();
  const { slots, fetchSlots, castSpell, resetSlots, loading } = useSpellStore();
  const { run, busy } = useTableStore();

  useEffect(() => {
    void run(fetchSlots);
  }, [run, fetchSlots]);

  // Group by level for display, keeping each slot's index (the cast endpoint's id).
  const byLevel = new Map<number, (SpellSlot & { index: number })[]>();
  slots.forEach((slot, index) => {
    byLevel.set(slot.level, [...(byLevel.get(slot.level) ?? []), { ...slot, index }]);
  });
  const levels = [...byLevel.keys()].sort((a, b) => a - b);

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <h2 className="text-base font-semibold">{t("spellSlots.title")}</h2>
        <Button
          variant="outline"
          size="sm"
          onClick={() => void run(resetSlots)}
          disabled={busy}
        >
          {t("spellSlots.longRest")}
        </Button>
      </div>
      {levels.length === 0 && !loading && (
        <p className="text-sm text-muted-foreground">{t("spellSlots.none")}</p>
      )}
      <div className="grid grid-cols-2 gap-3">
        {levels.map((level) => {
          const list = byLevel.get(level) ?? [];
          const left = list.filter((s) => !s.used).length;
          return (
            <section
              key={level}
              className="min-w-0 rounded-lg border"
              aria-label={t("spellSlots.level", { level })}
            >
              <div className="flex items-center justify-between border-b px-3 py-1.5">
                <h3 className="text-sm font-semibold">
                  {t("spellSlots.level", { level })}
                </h3>
                <Badge variant={left === 0 ? "destructive" : "secondary"}>
                  {t("spellSlots.remaining", { left, total: list.length })}
                </Badge>
              </div>
              <ul className="flex flex-col gap-1 p-1.5">
                {list.map((slot) => {
                  const name = spellName(
                    t,
                    spellId(slot.prepared_spell_name),
                    slot.prepared_spell_name,
                  );
                  return (
                    <li
                      key={slot.index}
                      className={cn(
                        "flex items-center justify-between gap-2 rounded px-2 py-1",
                        slot.used ? "opacity-60" : "hover:bg-muted/60",
                      )}
                    >
                      <span
                        className={cn(
                          "min-w-0 truncate text-sm",
                          slot.used && "text-muted-foreground line-through",
                        )}
                      >
                        {name}
                      </span>
                      <Button
                        size="xs"
                        variant={slot.used ? "ghost" : "secondary"}
                        disabled={slot.used || busy}
                        aria-label={t("spellSlots.castLabel", { name })}
                        onClick={() => void run(() => castSpell(slot.index))}
                      >
                        {slot.used ? t("spellSlots.used") : t("spellSlots.cast")}
                      </Button>
                    </li>
                  );
                })}
              </ul>
            </section>
          );
        })}
      </div>
    </div>
  );
}
