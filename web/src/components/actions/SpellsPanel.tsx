import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useSpellStore } from "@/stores/spells";
import { useLogStore } from "@/stores/log";
import { useTableStore } from "@/stores/table";
import { useItemsStore } from "@/stores/items";
import { spellId, spellName } from "@/i18n/spellName";
import type { SpellSlot } from "@/types/spells";
import { cn } from "@/lib/utils";
import { PrepareEditor } from "./PrepareEditor";
import { ReceivePanel } from "./ReceivePanel";

interface SlotRow {
  /** The copy a cast uses: the first unused one (the first when all are used). */
  slot: SpellSlot & { index: number };
  blank: boolean;
  name: string;
  remaining: number;
}

/** One row per distinct prepared spell (copies counted), sorted by displayed name; blank Rary slots last. */
function groupSlots(
  list: (SpellSlot & { index: number })[],
  nameOf: (s: SpellSlot) => string,
  blankName: string,
): SlotRow[] {
  const rows: SlotRow[] = [];
  const byKey = new Map<string, SlotRow>();
  for (const slot of list) {
    const blank = Boolean(slot.extra && !slot.prepared_spell_id && !slot.prepared_spell_name);
    if (blank) {
      rows.push({ slot, blank, name: blankName, remaining: slot.used ? 0 : 1 });
      continue;
    }
    const key = slot.prepared_spell_id || slot.prepared_spell_name;
    const row = byKey.get(key);
    if (!row) {
      const r = { slot, blank, name: nameOf(slot), remaining: slot.used ? 0 : 1 };
      byKey.set(key, r);
      rows.push(r);
    } else if (!slot.used) {
      if (row.remaining === 0) row.slot = slot;
      row.remaining++;
    }
  }
  return rows.sort(
    (a, b) => Number(a.blank) - Number(b.blank) || a.name.localeCompare(b.name) || a.slot.index - b.slot.index,
  );
}

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
  // Rary's Mnemonic Enhancer: how its 3 spell levels split into slots.
  const [raryChoice, setRaryChoice] = useState("1,1,1");
  // Blank Rary's slots: the spell picked per slot index.
  // Chromatic Orb: the target AC and the gem spent (inventory gems other than the daily Power Gem).
  const [orbAc, setOrbAc] = useState("");
  const [orbGem, setOrbGem] = useState("");
  const gems = useItemsStore((s) => s.items).filter(
    (i) => i.type.toLowerCase() === "gem" && i.behaviour !== "daily" && i.count > 0,
  );
  const [picks, setPicks] = useState<Record<number, string>>({});
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
          initial={slots.filter((s) => !s.extra).map((s) => s.prepared_spell_id || spellId(s.prepared_spell_name))}
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
                      {t("spellSlots.preparedCount", { count: list.filter((s) => !s.extra).length, max: maxOf(level) })}
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
                {groupSlots(list, (slot) =>
                  spellName(
                    t,
                    slot.prepared_spell_id || spellId(slot.prepared_spell_name),
                    slot.prepared_spell_name,
                  ),
                  t("spellSlots.extraBlank"),
                ).map(({ slot, blank, name: baseName, remaining }) => {
                  // A spell prepared several times is one row with the copies left; casting uses one.
                  const name = remaining > 1 ? `${baseName} ×${remaining}` : baseName;
                  const isRary = slot.prepared_spell_id === "rarys_mnemonic_enhancer";
                  const isOrb = slot.prepared_spell_id === "chromatic_orb";
                  const orbAcValue = /^-?\d+$/.test(orbAc.trim()) ? Number(orbAc) : undefined;
                  const choices = info.spellbook.filter((s) => s.level === slot.level);
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
                      {blank && !slot.used && (
                        <select
                          className="h-7 min-w-0 flex-1 rounded border bg-background px-1 text-sm"
                          aria-label={t("spellSlots.extraPick")}
                          data-testid="extra-pick"
                          value={picks[slot.index] ?? ""}
                          onChange={(e) => setPicks({ ...picks, [slot.index]: e.target.value })}
                        >
                          <option value="">{t("spellSlots.extraPick")}</option>
                          {choices.map((s) => (
                            <option key={s.id} value={s.id}>
                              {spellName(t, s.id, s.name)}
                            </option>
                          ))}
                        </select>
                      )}
                      {isOrb && !slot.used && (
                        <>
                          <input
                            className="h-7 w-16 rounded border bg-background px-1 text-sm"
                            aria-label={t("spellSlots.orbAc")}
                            placeholder={t("spellSlots.orbAc")}
                            data-testid="orb-ac"
                            inputMode="numeric"
                            value={orbAc}
                            onChange={(e) => setOrbAc(e.target.value)}
                          />
                          {gems.length > 1 && (
                            <select
                              className="h-7 min-w-0 rounded border bg-background px-1 text-sm"
                              aria-label={t("spellSlots.orbGem")}
                              data-testid="orb-gem"
                              value={orbGem || gems[0].id}
                              onChange={(e) => setOrbGem(e.target.value)}
                            >
                              {gems.map((g) => (
                                <option key={g.id} value={g.id}>
                                  {g.name} ({g.count})
                                </option>
                              ))}
                            </select>
                          )}
                        </>
                      )}
                      {isRary && !slot.used && (
                        <select
                          className="h-7 rounded border bg-background px-1 text-sm"
                          aria-label={t("spellSlots.raryChoice")}
                          data-testid="rary-split"
                          value={raryChoice}
                          onChange={(e) => setRaryChoice(e.target.value)}
                        >
                          <option value="1,1,1">{t("spellSlots.rary111")}</option>
                          <option value="1,2">{t("spellSlots.rary12")}</option>
                          <option value="3">{t("spellSlots.rary3")}</option>
                        </select>
                      )}
                      <Button
                        size="xs"
                        variant={slot.used ? "ghost" : "secondary"}
                        disabled={slot.used || busy || (blank && !picks[slot.index]) || (isOrb && orbAcValue === undefined)}
                        aria-label={t("spellSlots.castLabel", { name: baseName })}
                        onClick={() =>
                          void run(() =>
                            castSpell(slot.index, {
                              onSelf: onAlly ? false : undefined,
                              spellId: blank ? picks[slot.index] : undefined,
                              extraSlots: isRary ? raryChoice.split(",").map(Number) : undefined,
                              targetAc: isOrb ? orbAcValue : undefined,
                              gemItemId: isOrb && gems.length > 1 ? orbGem || gems[0].id : undefined,
                            }),
                          )
                        }
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
