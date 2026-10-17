import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { CaretDown, Moon } from "@phosphor-icons/react";
import { localized } from "@/i18n/localized";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useSpellStore } from "@/stores/spells";
import { useLogStore } from "@/stores/log";
import { useTableStore } from "@/stores/table";
import { useItemsStore } from "@/stores/items";
import { spellId, useSpellName } from "@/i18n/spellName";
import type { CastTarget, SpellSlot } from "@/types/spells";
import { cn } from "@/lib/utils";
import { splitText } from "@/lib/slotSplit";
import { modeName } from "@/lib/spellMode";
import { planCast } from "@/lib/castInputs";
import { slotDots } from "@/lib/slotDots";
import { Dots } from "./Dots";
import type { CastOptions } from "@/api/spells";
import { PrepareEditor } from "./PrepareEditor";
import { SpellDetailsDialog } from "./SpellDetailsDialog";
import { fieldClass } from "@/components/dashboard/fieldStyles";

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

/** The level's slot dots (ADR 0027 step 10.3). */
function SlotDots({
  level,
  slots,
  max,
}: {
  level: number;
  slots: SpellSlot[];
  max?: number;
}) {
  const { t } = useTranslation();
  const { dots, available, used, empty } = slotDots(slots, max);
  return (
    <Dots
      dots={dots}
      label={t("spellSlots.dots", { available, used, empty })}
      testId={`dots-${level}`}
    />
  );
}

/**
 * "Lancer" as a split button (ADR 0027 step 10.1): the main part casts with the server's default
 * target (no `onSelf`); the arrow offers the other target the server serves for the spell
 * (`cast_target.alternate`), and shows only when the server serves one.
 */
function CastButton({
  name,
  disabled,
  target,
  onCast,
}: {
  name: string;
  disabled: boolean;
  target?: CastTarget;
  onCast: (onSelf?: boolean) => void;
}) {
  const { t } = useTranslation();
  return (
    <span className="flex shrink-0">
      <Button
        size="lg"
        variant="main"
        className={cn(target && "rounded-r-none")}
        disabled={disabled}
        aria-label={t("spellSlots.castLabel", { name })}
        title={t("spellSlots.castLabel", { name })}
        onClick={() => onCast()}
      >
        {t("spellSlots.cast")}
      </Button>
      {target && (
        <DropdownMenu>
          <DropdownMenuTrigger
            disabled={disabled}
            render={
              <Button
                size="icon-lg"
                variant="main"
                className="w-6 rounded-l-none border-l-on-accent/40"
                aria-label={t("spellSlots.castTarget", { name })}
                title={t("spellSlots.castTarget", { name })}
              />
            }
          >
            <CaretDown aria-hidden />
          </DropdownMenuTrigger>
          <DropdownMenuContent>
            <DropdownMenuItem
              onClick={() => onCast(target.alternate === "self")}
            >
              {target.alternate === "self"
                ? t("spellSlots.castOnSelf")
                : t("spellSlots.castOnAlly")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </span>
  );
}

/**
 * Spells tab of the Act panel (ADR 0027 step 10). Today's prepared spells per level, each level's
 * slots as dots, the levels in two text columns when the panel is wide enough; a row is the spell's
 * name (opening its details) and its cast button. Prepare and Long rest on top.
 */
export function SpellsPanel() {
  const { t, i18n } = useTranslation();
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
  // The spell whose details popup is open.
  const [details, setDetails] = useState<{
    id: string;
    name: string;
    level: number;
  } | null>(null);
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

  // Group by level for display, keeping each slot's index (the cast endpoint's id). Only the levels
  // with a slot show (ADR 0027 step 10.5).
  const byLevel = new Map<number, (SpellSlot & { index: number })[]>();
  slots.forEach((slot, index) => {
    byLevel.set(slot.level, [
      ...(byLevel.get(slot.level) ?? []),
      { ...slot, index },
    ]);
  });
  const maxOf = (level: number) =>
    info.limits.find((l) => l.level === level)?.max;
  const levels = [...byLevel.keys()].sort((a, b) => a - b);

  // "Préparer" turns the tab into the editor, in the same space (ADR 0027 step 11.1).
  if (editing)
    return (
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
    );

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-end gap-2">
        <Button
          size="sm"
          title={t("spellSlots.prepare")}
          aria-label={t("spellSlots.prepare")}
          onClick={() => setEditing(true)}
          disabled={busy}
        >
          {t("spellSlots.prepareShort")}
        </Button>
        <Button
          size="sm"
          title={t("spellSlots.longRestLabel")}
          aria-label={t("spellSlots.longRestLabel")}
          onClick={() => void run(resetSlots)}
          disabled={busy}
        >
          <Moon aria-hidden />
          {t("spellSlots.longRest")}
        </Button>
      </div>
      {levels.length === 0 && !loading && (
        <p className="text-body text-muted-foreground">
          {t("spellSlots.none")}
        </p>
      )}
      {/* The levels flow in two text columns, 24 px apart, once the panel is wide enough (ADR 0022
          step 3.2); a level never splits across them. */}
      <div className="@container">
        <div className="gap-x-6 @lg:columns-2">
          {levels.map((level) => {
            const list = byLevel.get(level) ?? [];
            return (
              <section
                key={level}
                className="mb-3 min-w-0 break-inside-avoid"
                aria-label={t("spellSlots.level", { level })}
              >
                <div className="flex flex-wrap items-center gap-x-3 gap-y-1 px-2 pb-1">
                  <h3 className="section-label">
                    {t("spellSlots.level", { level })}
                  </h3>
                  <span className="ml-auto">
                    <SlotDots level={level} slots={list} max={maxOf(level)} />
                  </span>
                </div>
                <ul className="flex flex-col gap-0.5">
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
                    const cast = (onSelf?: boolean) =>
                      void run(() =>
                        castSpell(slot.index, {
                          ...plan.options,
                          onSelf,
                        } satisfies CastOptions),
                      );
                    return (
                      <li
                        key={slot.index}
                        className={cn(
                          "flex min-h-9 flex-wrap items-center gap-x-2 gap-y-1 rounded-md px-2 py-0.5",
                          !slot.used && "hover:bg-raised",
                        )}
                      >
                        {/* A blank extra slot's picker stands for its name (below). */}
                        {blank && !slot.used ? null : blank ? (
                          <span className="min-w-32 flex-1 text-body break-words text-muted-foreground">
                            {name}
                          </span>
                        ) : (
                          // The name opens the spell's details (ADR 0027 step 10.4).
                          <button
                            type="button"
                            className={cn(
                              "min-w-32 flex-1 cursor-pointer rounded-sm text-left text-body break-words outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50",
                              slot.used && "text-muted-foreground line-through",
                            )}
                            title={t("spellSlots.details", { name: baseName })}
                            onClick={() =>
                              setDetails({
                                id:
                                  slot.prepared_spell_id ||
                                  spellId(slot.prepared_spell_name),
                                name: baseName,
                                level: slot.level,
                              })
                            }
                          >
                            {name}
                          </button>
                        )}
                        {blank && !slot.used && (
                          <select
                            className={cn(
                              fieldClass,
                              "w-auto min-w-32 flex-1 px-1",
                            )}
                            aria-label={t("spellSlots.extraPick")}
                            data-testid="extra-pick"
                            value={picks[slot.index] ?? ""}
                            onChange={(e) =>
                              setPicks({
                                ...picks,
                                [slot.index]: e.target.value,
                              })
                            }
                          >
                            <option value="">
                              {t("spellSlots.extraPick")}
                            </option>
                            {choices.map((s) => (
                              <option key={s.id} value={s.id}>
                                {spellName(s.id, s.name)}
                              </option>
                            ))}
                          </select>
                        )}
                        {/* The cast inputs stay with the cast button: they wrap under the name together. */}
                        <span className="ml-auto flex flex-wrap items-center justify-end gap-2">
                          {needs?.attack_roll && !slot.used && (
                            <input
                              className={cn(fieldClass, "w-16 px-1")}
                              aria-label={t("spellSlots.targetAc")}
                              placeholder={t("attack.acShort")}
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
                              className={cn(fieldClass, "w-auto px-1")}
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
                                  {localized(i18n.language, i.name)} ({i.count})
                                </option>
                              ))}
                            </select>
                          )}
                          {modes.length > 0 && needs && !slot.used && (
                            <select
                              className={cn(fieldClass, "w-auto px-1")}
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
                              className={cn(
                                fieldClass,
                                "w-auto max-w-full px-1",
                              )}
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
                          {slot.used ? (
                            <span className="px-2 text-label text-muted-foreground">
                              {t("spellSlots.used")}
                            </span>
                          ) : (
                            <CastButton
                              name={baseName}
                              disabled={busy || plan.blocked}
                              target={plan.target}
                              onCast={cast}
                            />
                          )}
                        </span>
                      </li>
                    );
                  })}
                </ul>
              </section>
            );
          })}
        </div>
      </div>
      <SpellDetailsDialog spell={details} onClose={() => setDetails(null)} />
    </div>
  );
}
