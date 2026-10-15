import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { useSpellStore } from "@/stores/spells";
import { useLogStore } from "@/stores/log";
import { useTableStore } from "@/stores/table";
import { useItemsStore } from "@/stores/items";
import { spellId, useSpellName } from "@/i18n/spellName";
import type { SpellSlot } from "@/types/spells";
import { cn } from "@/lib/utils";
import { splitText } from "@/lib/slotSplit";
import { modeName } from "@/lib/spellMode";
import { planCast } from "@/lib/castInputs";
import { damageText } from "@/lib/spellDice";
import { exposureText } from "@/lib/spellMode";
import { PrepareEditor } from "./PrepareEditor";

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
    const blank = Boolean(
      slot.extra && !slot.prepared_spell_id && !slot.prepared_spell_name,
    );
    if (blank) {
      rows.push({ slot, blank, name: blankName, remaining: slot.used ? 0 : 1 });
      continue;
    }
    const key = slot.prepared_spell_id || slot.prepared_spell_name;
    const row = byKey.get(key);
    if (!row) {
      const r = {
        slot,
        blank,
        name: nameOf(slot),
        remaining: slot.used ? 0 : 1,
      };
      byKey.set(key, r);
      rows.push(r);
    } else if (!slot.used) {
      if (row.remaining === 0) row.slot = slot;
      row.remaining++;
    }
  }
  return rows.sort(
    (a, b) =>
      Number(a.blank) - Number(b.blank) ||
      a.name.localeCompare(b.name) ||
      a.slot.index - b.slot.index,
  );
}

/**
 * Today's prepared spells per level: cast (result in the result card), long rest, the preparation
 * editor, and spells cast on the Bard by the party.
 */
export function SpellsPanel() {
  const { t } = useTranslation();
  const spellName = useSpellName();
  const {
    slots,
    info,
    fetchSlots,
    fetchInfo,
    castSpell,
    resetSlots,
    prepare,
    loading,
  } = useSpellStore();
  const [editing, setEditing] = useState(false);
  // Buff spells that can go on someone else land on the Bard unless this is checked.
  const [onAlly, setOnAlly] = useState(false);
  // What a cast asks of the player comes from the spellbook data (slot_splits, consume_item_type,
  // attack_roll), per slot index: the split of the extra slots granted, the target AC, the item spent.
  const [splits, setSplits] = useState<Record<number, string>>({});
  const [targetAcs, setTargetAcs] = useState<Record<number, string>>({});
  const [itemPicks, setItemPicks] = useState<Record<number, string>>({});
  // Spells with cast-time modes: the mode key picked per slot index (none: the first).
  const [modePicks, setModePicks] = useState<Record<number, string>>({});
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

  // The items a spell can spend are served with the spellbook: ask again when the inventory
  // changes (an item edited or restocked, a cast or a void in the log).
  const items = useItemsStore((s) => s.items);
  useEffect(() => {
    void fetchInfo().catch(() => undefined);
  }, [items, logEntries, fetchInfo]);

  // Group by level for display, keeping each slot's index (the cast endpoint's id).
  const byLevel = new Map<number, (SpellSlot & { index: number })[]>();
  slots.forEach((slot, index) => {
    byLevel.set(slot.level, [
      ...(byLevel.get(slot.level) ?? []),
      { ...slot, index },
    ]);
  });
  const maxOf = (level: number) =>
    info.limits.find((l) => l.level === level)?.max;
  const levels = [
    ...new Set([...byLevel.keys(), ...info.limits.map((l) => l.level)]),
  ].sort((a, b) => a - b);

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <label className="flex items-center gap-2 text-body">
          <input
            type="checkbox"
            checked={onAlly}
            onChange={(e) => setOnAlly(e.target.checked)}
          />
          {t("spellSlots.onAlly")}
        </label>
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
          initial={slots
            .filter((s) => !s.extra)
            .map((s) => s.prepared_spell_id || spellId(s.prepared_spell_name))}
          busy={busy}
          onCancel={() => setEditing(false)}
          onSave={(ids) =>
            void run(() => prepare(ids)).then((ok) => {
              if (ok) setEditing(false);
            })
          }
        />
      )}
      {levels.length === 0 && !loading && (
        <p className="text-body text-muted-foreground">
          {t("spellSlots.none")}
        </p>
      )}
      <div className="grid grid-cols-2 gap-3">
        {levels.map((level) => {
          const list = byLevel.get(level) ?? [];
          const left = list.filter((s) => !s.used).length;
          return (
            <section
              key={level}
              className="min-w-0 rounded-lg bg-neu-900/40"
              aria-label={t("spellSlots.level", { level })}
            >
              <div className="flex items-center justify-between px-3 pt-2 pb-1">
                <h3 className="text-title">
                  {t("spellSlots.level", { level })}
                </h3>
                <div className="flex items-center gap-2">
                  {maxOf(level) !== undefined && (
                    <span className="text-label text-muted-foreground">
                      {t("spellSlots.preparedCount", {
                        count: list.filter((s) => !s.extra).length,
                        max: maxOf(level),
                      })}
                    </span>
                  )}
                  <span className="flex items-center gap-1" aria-hidden>
                    {list.map((_, i) => (
                      <span
                        key={i}
                        className={cn(
                          "size-2 rounded-full",
                          i < left ? "bg-magic" : "bg-neu-700",
                        )}
                      />
                    ))}
                  </span>
                  <span
                    data-testid={`remaining-${level}`}
                    className={cn(
                      "text-label",
                      list.length > 0 && left === 0
                        ? "text-danger"
                        : "text-muted-foreground",
                    )}
                  >
                    {t("spellSlots.remaining", { left, total: list.length })}
                  </span>
                </div>
              </div>
              <ul className="flex flex-col gap-1 p-1.5">
                {groupSlots(
                  list,
                  (slot) =>
                    spellName(
                      slot.prepared_spell_id ||
                        spellId(slot.prepared_spell_name),
                      slot.prepared_spell_name,
                    ),
                  t("spellSlots.extraBlank"),
                ).map(({ slot, blank, name: baseName, remaining }) => {
                  // A spell prepared several times is one row with the copies left; casting uses one.
                  const name =
                    remaining > 1 ? `${baseName} ×${remaining}` : baseName;
                  const choices = info.spellbook.filter(
                    (s) => s.level === slot.level,
                  );
                  const plan = planCast(slot, blank, info.spellbook, {
                    pick: picks[slot.index],
                    split: splits[slot.index],
                    acText: targetAcs[slot.index],
                    itemPick: itemPicks[slot.index],
                    mode: modePicks[slot.index],
                  });
                  const {
                    needs,
                    slotSplits,
                    split,
                    spendable,
                    itemId,
                    modes,
                    mode,
                  } = plan;
                  const acText = targetAcs[slot.index] ?? "";
                  // The damage a cast rolls at the Bard's level, as the server serves it: the picked
                  // mode's for a spell with modes.
                  const entry = blank
                    ? undefined
                    : info.spellbook.find(
                        (s) =>
                          s.id ===
                          (slot.prepared_spell_id ||
                            spellId(slot.prepared_spell_name)),
                      );
                  const damage = blank
                    ? undefined
                    : mode
                      ? mode.damage
                      : entry?.damage;
                  // A lasting area's damage per round of exposure (ADR 0008 step 16), rolled from
                  // the cast card.
                  const exposure = entry?.exposure;
                  return (
                    <li
                      key={slot.index}
                      className={cn(
                        "flex min-h-7 items-center justify-between gap-2 rounded px-2 py-1",
                        slot.used ? "opacity-60" : "hover:bg-muted/60",
                      )}
                    >
                      <span className="flex min-w-0 flex-col leading-tight">
                        <span
                          className={cn(
                            "min-w-0 truncate text-body",
                            slot.used && "text-muted-foreground line-through",
                          )}
                        >
                          {name}
                        </span>
                        {damage && (
                          <span
                            className="min-w-0 truncate text-label text-muted-foreground tabular-nums"
                            data-testid="slot-dice"
                          >
                            {damageText(t, damage)}
                          </span>
                        )}
                        {exposure && entry && (
                          <span
                            className="min-w-0 truncate text-label text-muted-foreground tabular-nums"
                            data-testid="slot-exposure"
                          >
                            {exposureText(t, entry.id, exposure)}
                          </span>
                        )}
                      </span>
                      {blank && !slot.used && (
                        <select
                          className="h-7 min-w-0 flex-1 rounded border bg-background px-1 text-body"
                          aria-label={t("spellSlots.extraPick")}
                          data-testid="extra-pick"
                          value={picks[slot.index] ?? ""}
                          onChange={(e) =>
                            setPicks({ ...picks, [slot.index]: e.target.value })
                          }
                        >
                          <option value="">{t("spellSlots.extraPick")}</option>
                          {choices.map((s) => (
                            <option key={s.id} value={s.id}>
                              {spellName(s.id, s.name)}
                            </option>
                          ))}
                        </select>
                      )}
                      {needs?.attack_roll && !slot.used && (
                        <input
                          className="h-7 w-16 rounded border bg-background px-1 text-body"
                          aria-label={t("spellSlots.targetAc")}
                          placeholder={t("spellSlots.targetAc")}
                          data-testid="cast-target-ac"
                          inputMode="numeric"
                          value={acText}
                          onChange={(e) =>
                            setTargetAcs({
                              ...targetAcs,
                              [slot.index]: e.target.value,
                            })
                          }
                        />
                      )}
                      {spendable.length > 1 && !slot.used && (
                        <select
                          className="h-7 min-w-0 rounded border bg-background px-1 text-body"
                          aria-label={t("spellSlots.itemSpent")}
                          data-testid="cast-item"
                          value={itemId}
                          onChange={(e) =>
                            setItemPicks({
                              ...itemPicks,
                              [slot.index]: e.target.value,
                            })
                          }
                        >
                          {spendable.map((i) => (
                            <option key={i.id} value={i.id}>
                              {i.name} ({i.count})
                            </option>
                          ))}
                        </select>
                      )}
                      {modes.length > 0 && needs && !slot.used && (
                        <select
                          className="h-7 min-w-0 rounded border bg-background px-1 text-body"
                          aria-label={t("spellSlots.modeChoice")}
                          data-testid="slot-mode"
                          value={mode?.key}
                          onChange={(e) =>
                            setModePicks({
                              ...modePicks,
                              [slot.index]: e.target.value,
                            })
                          }
                        >
                          {modes.map((m) => (
                            <option key={m.key} value={m.key}>
                              {modeName(t, needs.id, m.key)}
                            </option>
                          ))}
                        </select>
                      )}
                      {slotSplits.length > 0 && !slot.used && (
                        <select
                          className="h-7 rounded border bg-background px-1 text-body"
                          aria-label={t("spellSlots.splitChoice")}
                          data-testid="slot-split"
                          value={split}
                          onChange={(e) =>
                            setSplits({
                              ...splits,
                              [slot.index]: e.target.value,
                            })
                          }
                        >
                          {slotSplits.map((levels) => (
                            <option
                              key={levels.join(",")}
                              value={levels.join(",")}
                            >
                              {splitText(t, levels)}
                            </option>
                          ))}
                        </select>
                      )}
                      <Button
                        size="xs"
                        variant={slot.used ? "ghost" : "secondary"}
                        disabled={busy || plan.blocked}
                        aria-label={t("spellSlots.castLabel", {
                          name: baseName,
                        })}
                        onClick={() =>
                          void run(() =>
                            castSpell(slot.index, {
                              onSelf: onAlly ? false : undefined,
                              ...plan.options,
                            }),
                          )
                        }
                      >
                        {slot.used
                          ? t("spellSlots.used")
                          : t("spellSlots.cast")}
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
