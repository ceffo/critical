import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus, X } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
} from "@/components/ui/popover";
import { NO_QUERY, searchSpells } from "@/api/spells_db";
import { localized } from "@/i18n/localized";
import { modeText } from "@/lib/spellMode";
import { cn } from "@/lib/utils";
import { useBuffsStore } from "@/stores/buffs";
import { useSpellStore } from "@/stores/spells";
import { useTableStore } from "@/stores/table";
import type { Spell } from "@/types/spells";
import { AddBuffForm } from "./BuffForms";
import { fieldClass } from "./fieldStyles";

const SEARCH_PAGE_SIZE = 50;
const TABS = ["spell", "saved", "new"] as const;
type Tab = (typeof TABS)[number];

/**
 * "Add" on the effects timeline: one popover for the three ways an effect starts. A spell cast on
 * the Bard (its duration comes from the caster level, computed by the server), a saved effect, or a
 * new custom one. Each closes the popover once the server has the effect.
 */
export function AddEffect() {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [tab, setTab] = useState<Tab>("spell");

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={
          <Button size="xs" variant="ghost" aria-label={t("buffs.add")} />
        }
        data-testid="add-effect"
      >
        <Plus />
        {t("buffs.addShort")}
      </PopoverTrigger>
      <PopoverContent
        side="bottom"
        align="start"
        className="flex w-[34rem] max-w-[calc(100vw-2rem)] flex-col gap-3"
        aria-label={t("buffs.add")}
        data-testid="add-effect-popover"
      >
        <PopoverTitle className="sr-only">{t("buffs.add")}</PopoverTitle>
        <div
          role="tablist"
          aria-label={t("buffs.add")}
          className="flex gap-1 rounded-lg bg-neu-900 p-1"
        >
          {TABS.map((id) => (
            <button
              key={id}
              type="button"
              role="tab"
              aria-selected={tab === id}
              className={cn(
                "flex-1 rounded-md px-2 py-1 text-body outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                tab === id
                  ? "bg-tint/15 font-medium text-amb-200"
                  : "text-muted-foreground hover:text-foreground",
              )}
              onClick={() => setTab(id)}
            >
              {t(`buffs.tab.${id}`)}
            </button>
          ))}
        </div>
        {tab === "spell" && <ReceiveSpell onDone={() => setOpen(false)} />}
        {tab === "saved" && <SavedEffects onDone={() => setOpen(false)} />}
        {tab === "new" && <AddBuffForm onDone={() => setOpen(false)} />}
      </PopoverContent>
    </Popover>
  );
}

/** A spell cast on the Bard: find it by name (en/fr), enter the caster's level, apply it. No slot is used. */
function ReceiveSpell({ onDone }: { onDone: () => void }) {
  const { t, i18n } = useTranslation();
  const receiveSpell = useSpellStore((s) => s.receiveSpell);
  const { run, busy } = useTableStore();
  const [query, setQuery] = useState("");
  const [level, setLevel] = useState("");
  const [found, setFound] = useState<Spell[]>([]);
  /** Matches the API has beyond the page shown. */
  const [more, setMore] = useState(0);
  /** Spells with cast-time modes: the mode the caster picked, per spell id (none: the first). */
  const [modes, setModes] = useState<Record<string, string>>({});

  const casterLevel = Number(level);
  const levelOk =
    Number.isInteger(casterLevel) && casterLevel >= 1 && casterLevel <= 40;
  // The modes' dice are served at the caster level typed, the one the receive rolls at.
  const searchLevel = levelOk ? casterLevel : undefined;

  useEffect(() => {
    const q = query.trim();
    // Below two letters the hint shows instead of the (stale) results.
    if (q.length < 2) return;
    let live = true;
    const timer = setTimeout(() => {
      // The lookup's ranked search: English and French names, exact > prefix > contains.
      searchSpells({ ...NO_QUERY, q }, 1, SEARCH_PAGE_SIZE, searchLevel)
        .catch(() => ({ items: [] as Spell[], total: 0 }))
        .then(({ items, total }) => {
          if (!live) return;
          setFound(items);
          setMore(Math.max(0, total - items.length));
        });
    }, 200);
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [query, searchLevel]);

  const searching = query.trim().length >= 2;

  async function apply(id: string) {
    if (await run(() => receiveSpell(id, casterLevel, modes[id]))) onDone();
  }

  return (
    <section
      className="flex flex-col gap-2"
      aria-label={t("spellSlots.receiveTitle")}
    >
      <div className="grid grid-cols-[1fr_8rem] gap-2">
        <label className="flex min-w-0 flex-col gap-1">
          <span className="text-label text-muted-foreground">
            {t("spellSlots.receiveSearchLabel")}
          </span>
          <input
            className={fieldClass}
            type="search"
            value={query}
            placeholder={t("spellSlots.receiveSearch")}
            onChange={(e) => setQuery(e.target.value)}
            autoFocus
          />
        </label>
        <label className="flex min-w-0 flex-col gap-1">
          <span className="text-label text-muted-foreground">
            {t("spellSlots.casterLevel")}
          </span>
          <input
            className={`${fieldClass} tabular-nums`}
            type="number"
            min={1}
            max={40}
            value={level}
            onChange={(e) => setLevel(e.target.value)}
          />
        </label>
      </div>
      {!searching ? (
        <p className="text-body text-muted-foreground">
          {t("spellSlots.receiveHint")}
        </p>
      ) : found.length === 0 ? (
        <p className="text-body text-muted-foreground">
          {t("spellSlots.noMatch")}
        </p>
      ) : (
        <ul className="flex max-h-56 flex-col gap-1 overflow-y-auto">
          {found.map((c) => {
            const name = localized(i18n.language, c.name);
            return (
              <li
                key={c.id}
                className="flex items-center justify-between gap-2 rounded px-2 py-1 hover:bg-muted/60"
              >
                <span className="min-w-0 truncate text-body">
                  {name}
                  <span className="text-muted-foreground tabular-nums">
                    {" "}
                    · {c.class} {c.level}
                  </span>
                </span>
                {c.modes && c.modes.length > 0 && (
                  <select
                    className="h-7 min-w-0 rounded border bg-background px-1 text-body"
                    aria-label={t("spellSlots.modeChoiceFor", { name })}
                    data-testid="receive-mode"
                    value={modes[c.id] ?? c.modes[0].key}
                    onChange={(e) =>
                      setModes({ ...modes, [c.id]: e.target.value })
                    }
                  >
                    {c.modes.map((m) => (
                      <option key={m.key} value={m.key}>
                        {modeText(t, c.id, m)}
                      </option>
                    ))}
                  </select>
                )}
                <Button
                  size="xs"
                  variant="secondary"
                  disabled={busy || !levelOk}
                  aria-label={t("spellSlots.receiveLabel", { name })}
                  onClick={() => void apply(c.id)}
                >
                  {t("spellSlots.receive")}
                </Button>
              </li>
            );
          })}
        </ul>
      )}
      {searching && more > 0 && (
        <p
          className="text-body text-muted-foreground tabular-nums"
          data-testid="receive-more"
        >
          {t("spellSlots.receiveMore", { count: more })}
        </p>
      )}
    </section>
  );
}

/** The effects saved for reuse: add one as it was saved, or delete it (active effects stay). */
function SavedEffects({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation();
  const { activateBuff, deleteSavedBuff, saved } = useBuffsStore();
  const { run, busy } = useTableStore();

  if (saved.length === 0) {
    return (
      <p className="text-body text-muted-foreground">{t("buffs.savedNone")}</p>
    );
  }
  return (
    <ul
      className="flex max-h-64 flex-col gap-1 overflow-y-auto"
      aria-label={t("buffs.saved")}
    >
      {saved.map((s) => (
        <li
          key={s.id}
          data-testid="saved-buff"
          className="flex items-center justify-between gap-2 rounded px-2 py-1 hover:bg-muted/60"
        >
          <span className="flex min-w-0 flex-col leading-tight">
            <span className="truncate text-body">{s.name}</span>
            {s.note && (
              <span className="truncate text-label text-muted-foreground">
                {s.note}
              </span>
            )}
          </span>
          <span className="flex shrink-0 items-center gap-1">
            <Button
              size="xs"
              variant="secondary"
              disabled={busy}
              aria-label={t("buffs.addSaved", { name: s.name })}
              onClick={() =>
                void run(() =>
                  activateBuff({
                    name: s.name,
                    duration: s.duration,
                    charges: s.charges,
                    effects: s.effects,
                    note: s.note,
                  }),
                ).then((ok) => ok && onDone())
              }
            >
              {t("buffs.addShort")}
            </Button>
            <Button
              size="icon-xs"
              variant="ghost"
              disabled={busy}
              aria-label={t("buffs.deleteSaved", { name: s.name })}
              title={t("buffs.deleteSaved", { name: s.name })}
              onClick={() => void run(() => deleteSavedBuff(s.id))}
            >
              <X />
            </Button>
          </span>
        </li>
      ))}
    </ul>
  );
}
