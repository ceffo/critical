import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { MagicWand, X } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
} from "@/components/ui/popover";
import { NO_QUERY, searchSpells } from "@/api/spells_db";
import { localized } from "@/i18n/localized";
import { useSpellName } from "@/i18n/spellName";
import type { Spell } from "@/types/spells";
import { fieldClass } from "./fieldStyles";

const SEARCH_PAGE_SIZE = 30;

/**
 * The spell the next save is against (ADR 0006 step 10.2), on the saves' label line: a rare
 * choice, one click away (ADR 0027 step 1). Unset, a "Spell" button opens a name search (English
 * and French, the lookup's ranked search); set, it shows as a tag that clears it. The server decides
 * whether the character is immune; the panel only sends the id.
 */
export function SaveSpell({
  spellId,
  onChange,
  disabled,
}: {
  spellId: string | undefined;
  onChange: (id: string | undefined) => void;
  disabled: boolean;
}) {
  const { t, i18n } = useTranslation();
  const spellName = useSpellName();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [found, setFound] = useState<Spell[]>([]);

  useEffect(() => {
    const q = query.trim();
    // Below two letters the hint shows instead of the (stale) results.
    if (q.length < 2) return;
    let live = true;
    const timer = setTimeout(() => {
      searchSpells({ ...NO_QUERY, q }, 1, SEARCH_PAGE_SIZE)
        .catch(() => ({ items: [] as Spell[], total: 0 }))
        .then(({ items }) => {
          if (live) setFound(items);
        });
    }, 200);
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [query]);

  if (spellId) {
    const name = spellName(spellId, spellId);
    return (
      <span
        className="chip-chosen flex min-w-0 items-center gap-1 rounded-md py-0.5 pr-0.5 pl-2 text-label"
        data-testid="save-spell-tag"
      >
        <span className="min-w-0 break-words">
          {t("saves.spellTag", { name })}
        </span>
        <button
          type="button"
          className="rounded p-0.5 outline-none hover:bg-raised-hover focus-visible:ring-3 focus-visible:ring-ring/50"
          aria-label={t("saves.spellClear", { name })}
          title={t("saves.spellClear", { name })}
          disabled={disabled}
          onClick={() => onChange(undefined)}
          data-testid="save-spell-clear"
        >
          <X aria-hidden />
        </button>
      </span>
    );
  }

  const searching = query.trim().length >= 2;
  return (
    <Popover
      open={open}
      onOpenChange={(o) => {
        setOpen(o);
        if (!o) setQuery("");
      }}
    >
      <PopoverTrigger
        render={
          <Button
            size="xs"
            variant="ghost"
            disabled={disabled}
            title={t("saves.spellHint")}
            aria-label={t("saves.spellHint")}
          />
        }
        data-testid="save-spell"
      >
        <MagicWand />
        {t("saves.spell")}
      </PopoverTrigger>
      <PopoverContent
        side="bottom"
        align="end"
        className="flex w-[24rem] max-w-[calc(100vw-2rem)] flex-col gap-2"
        aria-label={t("saves.spellSearchLabel")}
        data-testid="save-spell-popover"
      >
        <PopoverTitle className="sr-only">
          {t("saves.spellSearchLabel")}
        </PopoverTitle>
        <label className="flex min-w-0 flex-col gap-1">
          <span className="text-label text-muted-foreground">
            {t("saves.spellSearchLabel")}
          </span>
          <input
            className={fieldClass}
            type="search"
            value={query}
            placeholder={t("spellSlots.receiveSearch")}
            onChange={(e) => setQuery(e.target.value)}
            autoFocus
            data-testid="save-spell-search"
          />
        </label>
        {!searching ? (
          <p className="text-body text-muted-foreground">
            {t("spellSlots.receiveHint")}
          </p>
        ) : found.length === 0 ? (
          <p className="text-body text-muted-foreground">
            {t("spellSlots.noMatch")}
          </p>
        ) : (
          <ul className="flex max-h-56 flex-col gap-0.5 overflow-y-auto">
            {found.map((c) => {
              const name = localized(i18n.language, c.name);
              return (
                <li key={c.id}>
                  <button
                    type="button"
                    className="w-full rounded px-2 py-1 text-left text-body outline-none hover:bg-raised-hover focus-visible:ring-3 focus-visible:ring-ring/50"
                    aria-label={t("saves.spellPick", { name })}
                    data-testid="save-spell-option"
                    onClick={() => {
                      onChange(c.id);
                      setOpen(false);
                      setQuery("");
                    }}
                  >
                    <span className="break-words">{name}</span>
                    <span className="text-muted-foreground tabular-nums">
                      {" "}
                      · {c.class} {c.level}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        )}
      </PopoverContent>
    </Popover>
  );
}
