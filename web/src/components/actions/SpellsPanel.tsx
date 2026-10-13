import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useSpellStore } from "@/stores/spells";
import { useLogStore } from "@/stores/log";
import { useTableStore } from "@/stores/table";
import { spellId, spellName } from "@/i18n/spellName";
import type { SpellSlot } from "@/types/spells";
import { cn } from "@/lib/utils";
import { PrepareEditor } from "./PrepareEditor";
import { ReceivePanel } from "./ReceivePanel";

/**
 * Today's prepared spells per level: cast (result in the result card), long rest, the preparation
 * editor, and spells cast on the Bard by the party.
 */
export function SpellsPanel() {
  const { t } = useTranslation();
  const { slots, info, fetchSlots, fetchInfo, castSpell, resetSlots, prepare, loading } =
    useSpellStore();
  const [editing, setEditing] = useState(false);
  // Buff spells that can go on someone else land on the Bard unless this is checked.
  const [onAlly, setOnAlly] = useState(false);
  const logEntries = useLogStore((s) => s.entries);
  const { run, busy } = useTableStore();

  useEffect(() => {
    void run(async () => {
      await Promise.all([fetchSlots(), fetchInfo()]);
    });
  }, [run, fetchSlots, fetchInfo]);

  // A void from the combat log can change the preparation behind this panel.
  useEffect(() => {
    void fetchSlots().catch(() => undefined);
  }, [logEntries, fetchSlots]);

  // Group by level for display, keeping each slot's index (the cast endpoint's id).
  const byLevel = new Map<number, (SpellSlot & { index: number })[]>();
  slots.forEach((slot, index) => {
    byLevel.set(slot.level, [...(byLevel.get(slot.level) ?? []), { ...slot, index }]);
  });
  const maxOf = (level: number) => info.limits.find((l) => l.level === level)?.max;
  const levels = [...new Set([...byLevel.keys(), ...info.limits.map((l) => l.level)])].sort(
    (a, b) => a - b,
  );

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <h2 className="text-base font-semibold">{t("spellSlots.title")}</h2>
        <div className="flex gap-2">
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setEditing(true)}
            disabled={busy || editing}
          >
            {t("spellSlots.prepare")}
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => void run(resetSlots)}
            disabled={busy}
          >
            {t("spellSlots.longRest")}
          </Button>
        </div>
      </div>
      {editing && (
        <PrepareEditor
          spellbook={info.spellbook}
          limits={info.limits}
          initial={slots.map((s) => s.prepared_spell_id || spellId(s.prepared_spell_name))}
          busy={busy}
          onCancel={() => setEditing(false)}
          onSave={(ids) =>
            void run(() => prepare(ids)).then((ok) => {
              if (ok) setEditing(false);
            })
          }
        />
      )}
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" checked={onAlly} onChange={(e) => setOnAlly(e.target.checked)} />
        {t("spellSlots.onAlly")}
      </label>
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
                <div className="flex items-center gap-1.5">
                  {maxOf(level) !== undefined && (
                    <span className="text-xs text-muted-foreground">
                      {t("spellSlots.preparedCount", { count: list.length, max: maxOf(level) })}
                    </span>
                  )}
                  <Badge
                    data-testid={`remaining-${level}`}
                    variant={list.length > 0 && left === 0 ? "destructive" : "secondary"}
                  >
                    {t("spellSlots.remaining", { left, total: list.length })}
                  </Badge>
                </div>
              </div>
              <ul className="flex flex-col gap-1 p-1.5">
                {list.map((slot) => {
                  const name = spellName(
                    t,
                    slot.prepared_spell_id || spellId(slot.prepared_spell_name),
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
                        onClick={() => void run(() => castSpell(slot.index, onAlly ? false : undefined))}
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
      <ReceivePanel />
    </div>
  );
}
