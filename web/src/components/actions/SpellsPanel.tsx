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
import { splitText } from "@/lib/slotSplit";
import { PrepareEditor } from "./PrepareEditor";
import { ReceivePanel } from "./ReceivePanel";

interface SlotRow {
  /** The copy a cast uses: the first unused one (the first when all are used). */
  slot: SpellSlot & { index: number };
  blank: boolean;
  name: string;
  remaining: number;
}

/** One row per distinct prepared spell (copies counted), sorted by displayed name; blank extra slots last. */
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
  // What a cast asks of the player comes from the spellbook data (slot_splits, consume_item_type,
  // attack_roll), per slot index: the split of the extra slots granted, the target AC, the item spent.
  const [splits, setSplits] = useState<Record<number, string>>({});
  const [targetAcs, setTargetAcs] = useState<Record<number, string>>({});
  const [itemPicks, setItemPicks] = useState<Record<number, string>>({});
  const items = useItemsStore((s) => s.items);
  // Blank extra slots: the spell picked per slot index.
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
                  const choices = info.spellbook.filter((s) => s.level === slot.level);
                  const castId = blank ? picks[slot.index] : slot.prepared_spell_id;
                  const needs = info.spellbook.find((s) => s.id === castId);
                  const slotSplits = needs?.slot_splits ?? [];
                  const split = splits[slot.index] ?? slotSplits[0]?.join(",");
                  const acText = targetAcs[slot.index] ?? "";
                  const targetAc = /^-?\d+$/.test(acText.trim()) ? Number(acText) : undefined;
                  // Items a spell can spend: of its type, not a daily item (never spent by a spell), some left.
                  const spendable = needs?.consume_item_type
                    ? items.filter(
                        (i) =>
                          i.type.toLowerCase() === needs.consume_item_type!.toLowerCase() &&
                          i.behaviour !== "daily" &&
                          i.count > 0,
                      )
                    : [];
                  const itemId = itemPicks[slot.index] ?? spendable[0]?.id;
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
                      {needs?.attack_roll && !slot.used && (
                        <input
                          className="h-7 w-16 rounded border bg-background px-1 text-sm"
                          aria-label={t("spellSlots.targetAc")}
                          placeholder={t("spellSlots.targetAc")}
                          data-testid="cast-target-ac"
                          inputMode="numeric"
                          value={acText}
                          onChange={(e) => setTargetAcs({ ...targetAcs, [slot.index]: e.target.value })}
                        />
                      )}
                      {spendable.length > 1 && !slot.used && (
                        <select
                          className="h-7 min-w-0 rounded border bg-background px-1 text-sm"
                          aria-label={t("spellSlots.itemSpent")}
                          data-testid="cast-item"
                          value={itemId}
                          onChange={(e) => setItemPicks({ ...itemPicks, [slot.index]: e.target.value })}
                        >
                          {spendable.map((i) => (
                            <option key={i.id} value={i.id}>
                              {i.name} ({i.count})
                            </option>
                          ))}
                        </select>
                      )}
                      {slotSplits.length > 0 && !slot.used && (
                        <select
                          className="h-7 rounded border bg-background px-1 text-sm"
                          aria-label={t("spellSlots.splitChoice")}
                          data-testid="slot-split"
                          value={split}
                          onChange={(e) => setSplits({ ...splits, [slot.index]: e.target.value })}
                        >
                          {slotSplits.map((levels) => (
                            <option key={levels.join(",")} value={levels.join(",")}>
                              {splitText(t, levels)}
                            </option>
                          ))}
                        </select>
                      )}
                      <Button
                        size="xs"
                        variant={slot.used ? "ghost" : "secondary"}
                        disabled={
                          slot.used ||
                          busy ||
                          (blank && !picks[slot.index]) ||
                          (needs?.attack_roll === true && targetAc === undefined)
                        }
                        aria-label={t("spellSlots.castLabel", { name: baseName })}
                        onClick={() =>
                          void run(() =>
                            castSpell(slot.index, {
                              onSelf: onAlly ? false : undefined,
                              spellId: blank ? picks[slot.index] : undefined,
                              extraSlots: split ? split.split(",").map(Number) : undefined,
                              targetAc: needs?.attack_roll ? targetAc : undefined,
                              itemId: spendable.length > 1 ? itemId : undefined,
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
