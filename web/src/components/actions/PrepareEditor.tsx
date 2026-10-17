import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Minus, Plus } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useSpellName } from "@/i18n/spellName";
import {
  addOne,
  draftRows,
  levelBudgets,
  overBudget,
  removeOne,
  splitPrepared,
} from "@/lib/prepareDraft";
import { preparedDots } from "@/lib/slotDots";
import { cn } from "@/lib/utils";
import type { LevelLimit, SpellbookEntry } from "@/types/spells";
import { Dots } from "./Dots";
import { SpellDetailsDialog } from "./SpellDetailsDialog";

interface Props {
  spellbook: SpellbookEntry[];
  limits: LevelLimit[];
  /** Spell ids prepared today (duplicates allowed): the draft starts from them. */
  initial: string[];
  busy: boolean;
  /** Save the draft; the server enforces the limits and its error is shown by the table. */
  onSave: (ids: string[]) => void;
  onCancel: () => void;
}

/** A spell name that opens the details popup (ADR 0027 step 10.4), wrapping, never cut. */
function NameButton({ name, onOpen }: { name: string; onOpen: () => void }) {
  const { t } = useTranslation();
  return (
    <button
      type="button"
      className="min-w-0 flex-1 cursor-pointer rounded-sm text-left text-body break-words outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
      title={t("spellSlots.details", { name })}
      onClick={onOpen}
    >
      {name}
    </button>
  );
}

/**
 * The spell preparation, in place of the Spells tab's list (ADR 0027 step 11): today's preparation
 * as the starting draft, a level filter over the grimoire and the draft, − count + per spell, and per
 * level the dots of prepared versus possible. The budget is the server's count per level: the browser
 * only compares it with the draft's count to gate "Enregistrer"; the server still refuses an
 * over-budget save.
 */
export function PrepareEditor({
  spellbook,
  limits,
  initial,
  busy,
  onSave,
  onCancel,
}: Props) {
  const { t } = useTranslation();
  const spellName = useSpellName();
  // Ids no longer in the spellbook are shown as dropped and never sent (the API refuses them).
  const [{ kept, dropped }] = useState(() => splitPrepared(initial, spellbook));
  const [draft, setDraft] = useState<string[]>(kept);
  const [query, setQuery] = useState("");
  // The levels picked in the filter; none picked: every level.
  const [picked, setPicked] = useState<number[]>([]);
  const [details, setDetails] = useState<SpellbookEntry | null>(null);

  const byId = useMemo(
    () => new Map(spellbook.map((s) => [s.id, s])),
    [spellbook],
  );
  const label = (s: SpellbookEntry) => spellName(s.id, s.name);

  const budgets = levelBudgets(draft, spellbook, limits);
  const over = overBudget(budgets);
  // The filter offers the character's spell levels: those the server allows spells at.
  const filterLevels = budgets.filter((b) => b.max > 0).map((b) => b.level);
  const shown = budgets.filter(
    (b) => picked.length === 0 || picked.includes(b.level),
  );
  const toggle = (level: number) =>
    setPicked((p) =>
      p.includes(level) ? p.filter((l) => l !== level) : [...p, level],
    );

  const needle = query.trim().toLowerCase();
  const matches = (s: SpellbookEntry) =>
    needle === "" ||
    s.name.toLowerCase().includes(needle) ||
    label(s).toLowerCase().includes(needle);
  const byName = (a: SpellbookEntry, b: SpellbookEntry) =>
    label(a).localeCompare(label(b));
  const grimoire = shown
    .map(({ level }) => ({
      level,
      spells: spellbook
        .filter((s) => s.level === level && matches(s))
        .sort(byName),
    }))
    .filter((g) => g.spells.length > 0);
  const rows = draftRows(draft).flatMap(({ id, count }) => {
    const entry = byId.get(id);
    return entry ? [{ entry, count }] : [];
  });
  // Sorted by displayed name, as on the Spells tab.
  rows.sort((a, b) => byName(a.entry, b.entry));

  return (
    <div
      className="flex flex-col gap-3"
      role="form"
      aria-label={t("spellSlots.prepareTitle")}
    >
      <div className="flex flex-wrap items-center gap-2">
        <div
          role="group"
          aria-label={t("spellSlots.levelFilter")}
          className="flex flex-wrap items-center gap-1"
        >
          {filterLevels.map((level) => {
            const on = picked.includes(level);
            return (
              <button
                key={level}
                type="button"
                aria-pressed={on}
                aria-label={t("spellSlots.level", { level })}
                title={t("spellSlots.level", { level })}
                className={cn(
                  "h-7 w-7 text-body font-medium tabular-nums transition-colors",
                  on
                    ? "chip-chosen"
                    : "chip text-muted-foreground hover:text-foreground",
                )}
                onClick={() => toggle(level)}
              >
                {level}
              </button>
            );
          })}
        </div>
        <div className="ml-auto flex flex-wrap items-center justify-end gap-2">
          {over.length > 0 && (
            <span
              role="status"
              className="text-label text-danger"
              data-testid="over-budget"
            >
              {t("spellSlots.overBudget", {
                count: over.length,
                levels: over.join(", "),
              })}
            </span>
          )}
          <Button size="sm" onClick={onCancel} disabled={busy}>
            {t("spellSlots.cancel")}
          </Button>
          <Button
            size="sm"
            variant="main"
            title={t("spellSlots.save")}
            aria-label={t("spellSlots.save")}
            onClick={() => onSave(draft)}
            disabled={busy || over.length > 0}
          >
            {t("spellSlots.saveShort")}
          </Button>
        </div>
      </div>

      {dropped.length > 0 && (
        <div
          role="status"
          className="rounded border border-destructive/40 p-2 text-body"
          data-testid="stale-prepared"
        >
          <p className="font-medium">{t("spellSlots.staleTitle")}</p>
          <ul className="list-disc pl-5 text-muted-foreground">
            {dropped.map((id) => (
              <li key={id}>
                {id ? spellName(id, id) : t("spellSlots.staleUnknown")}
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* Two columns once the panel is wide enough (ADR 0022 step 3.2), stacked when narrow. */}
      <div className="@container">
        <div className="grid gap-3 @lg:grid-cols-2">
          <section
            className="flex min-h-64 min-w-0 flex-col gap-2 rounded-lg bg-raised p-3"
            aria-label={t("spellSlots.draft")}
          >
            <h3 className="section-label">{t("spellSlots.draft")}</h3>
            {shown.length === 0 && (
              <p className="text-body text-muted-foreground">
                {t("spellSlots.draftEmpty")}
              </p>
            )}
            {shown.map(({ level, count, max }) => {
              const items = rows.filter((r) => r.entry.level === level);
              return (
                <div key={level} data-testid={`draft-level-${level}`}>
                  <div className="flex flex-wrap items-center gap-x-3 gap-y-1 pb-1">
                    <h4 className="section-label">
                      {t("spellSlots.level", { level })}
                    </h4>
                    <span className="ml-auto">
                      <Dots
                        dots={preparedDots(count, max)}
                        label={t("spellSlots.preparedCount", { count, max })}
                        testId={`prepared-dots-${level}`}
                      />
                    </span>
                  </div>
                  <ul className="flex flex-col gap-0.5">
                    {items.map(({ entry, count: copies }) => (
                      <li
                        key={entry.id}
                        className="flex min-h-8 items-center gap-2 rounded-md px-2 hover:bg-raised-hover"
                      >
                        <NameButton
                          name={label(entry)}
                          onOpen={() => setDetails(entry)}
                        />
                        <span className="flex shrink-0 items-center gap-1">
                          <Button
                            size="icon-sm"
                            variant="ghost"
                            aria-label={t("spellSlots.remove", {
                              name: label(entry),
                            })}
                            title={t("spellSlots.remove", {
                              name: label(entry),
                            })}
                            onClick={() =>
                              setDraft((d) => removeOne(d, entry.id))
                            }
                          >
                            <Minus />
                          </Button>
                          <span
                            className="min-w-5 text-center font-semibold tabular-nums"
                            data-testid="prepared-count"
                          >
                            {copies}
                          </span>
                          <Button
                            size="icon-sm"
                            variant="ghost"
                            aria-label={t("spellSlots.addCopy", {
                              name: label(entry),
                            })}
                            title={t("spellSlots.addCopy", {
                              name: label(entry),
                            })}
                            onClick={() => setDraft((d) => addOne(d, entry.id))}
                          >
                            <Plus />
                          </Button>
                        </span>
                      </li>
                    ))}
                  </ul>
                </div>
              );
            })}
          </section>

          {/* Side by side, the grimoire is as tall as the preparation (it adds no height: 0 then
              the row's full height) and its list scrolls inside; stacked, the list has a cap. */}
          <section
            className="flex min-w-0 flex-col gap-2 rounded-lg bg-raised p-3 @lg:h-0 @lg:min-h-full"
            aria-label={t("spellSlots.spellbook")}
          >
            <h3 className="section-label">{t("spellSlots.spellbook")}</h3>
            <Input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t("spellSlots.search")}
              aria-label={t("spellSlots.searchLabel")}
            />
            <div className="flex max-h-80 min-h-0 flex-col gap-2 overflow-y-auto @lg:max-h-none @lg:flex-1">
              {grimoire.map(({ level, spells }) => (
                <div key={level} data-testid={`grimoire-level-${level}`}>
                  <h4 className="section-label px-2 pb-1">
                    {t("spellSlots.level", { level })}
                  </h4>
                  <ul className="flex flex-col gap-0.5">
                    {spells.map((s) => (
                      <li
                        key={s.id}
                        className="flex min-h-8 items-center gap-2 rounded-md px-2 hover:bg-raised-hover"
                      >
                        <NameButton
                          name={label(s)}
                          onOpen={() => setDetails(s)}
                        />
                        <Button
                          size="xs"
                          aria-label={t("spellSlots.add", { name: label(s) })}
                          title={t("spellSlots.add", { name: label(s) })}
                          onClick={() => setDraft((d) => addOne(d, s.id))}
                        >
                          {t("spellSlots.addShort")}
                        </Button>
                      </li>
                    ))}
                  </ul>
                </div>
              ))}
              {grimoire.length === 0 && (
                <p className="px-2 py-1 text-body text-muted-foreground">
                  {t("spellSlots.noMatch")}
                </p>
              )}
            </div>
          </section>
        </div>
      </div>
      <SpellDetailsDialog
        spell={
          details && {
            id: details.id,
            name: label(details),
            level: details.level,
          }
        }
        onClose={() => setDetails(null)}
      />
    </div>
  );
}
