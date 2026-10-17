import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useSpellName } from "@/i18n/spellName";
import { splitPrepared } from "@/lib/prepareDraft";
import { cn } from "@/lib/utils";
import type { LevelLimit, SpellbookEntry } from "@/types/spells";

interface Props {
  spellbook: SpellbookEntry[];
  limits: LevelLimit[];
  /** Spell ids prepared today (duplicates allowed). */
  initial: string[];
  busy: boolean;
  /** Save the draft; the server enforces the limits and its error is shown by the table. */
  onSave: (ids: string[]) => void;
  onCancel: () => void;
}

/**
 * Pick today's spells from the spellbook. The counts next to each level come
 * from the API's limits; whether a draft is allowed is decided by the server on save.
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

  const byId = useMemo(
    () => new Map(spellbook.map((s) => [s.id, s])),
    [spellbook],
  );
  const label = (s: SpellbookEntry) => spellName(s.id, s.name);

  const needle = query.trim().toLowerCase();
  const matches = spellbook.filter(
    (s) =>
      needle === "" ||
      s.name.toLowerCase().includes(needle) ||
      label(s).toLowerCase().includes(needle),
  );

  const levels = [
    ...new Set([
      ...limits.map((l) => l.level),
      ...spellbook.map((s) => s.level),
    ]),
  ].sort((a, b) => a - b);
  const maxOf = (level: number) =>
    limits.find((l) => l.level === level)?.max ?? 0;
  const draftEntries = draft.flatMap((id, index) => {
    const entry = byId.get(id);
    return entry ? [{ entry, index }] : [];
  });

  return (
    <div
      className="grid grid-cols-2 gap-3"
      role="form"
      aria-label={t("spellSlots.prepareTitle")}
    >
      <section
        className="flex min-w-0 flex-col gap-2 rounded-lg bg-raised p-3"
        aria-label={t("spellSlots.spellbook")}
      >
        <h3 className="section-label">{t("spellSlots.spellbook")}</h3>
        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t("spellSlots.search")}
          aria-label={t("spellSlots.searchLabel")}
        />
        <ul className="flex max-h-72 flex-col gap-1 overflow-y-auto">
          {matches.map((s) => (
            <li
              key={s.id}
              className="flex items-center justify-between gap-2 rounded px-2 py-1 hover:bg-raised-hover"
            >
              <span className="min-w-0 text-body break-words">
                <span className="mr-2 text-label text-muted-foreground">
                  {t("spellSlots.level", { level: s.level })}
                </span>
                {label(s)}
              </span>
              <Button
                size="xs"
                aria-label={t("spellSlots.add", { name: label(s) })}
                onClick={() => setDraft((d) => [...d, s.id])}
              >
                {t("spellSlots.addShort")}
              </Button>
            </li>
          ))}
          {matches.length === 0 && (
            <li className="px-2 py-1 text-body text-muted-foreground">
              {t("spellSlots.noMatch")}
            </li>
          )}
        </ul>
      </section>

      <section
        className="flex min-w-0 flex-col gap-2 rounded-lg bg-raised p-3"
        aria-label={t("spellSlots.draft")}
      >
        <h3 className="section-label">{t("spellSlots.draft")}</h3>
        {draftEntries.length === 0 && (
          <p className="text-body text-muted-foreground">
            {t("spellSlots.draftEmpty")}
          </p>
        )}
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
        {levels.map((level) => {
          const items = draftEntries.filter((d) => d.entry.level === level);
          if (items.length === 0 && maxOf(level) === 0) return null;
          return (
            <div key={level} data-testid={`draft-level-${level}`}>
              <div className="flex items-center justify-between">
                <h4 className="section-label">
                  {t("spellSlots.level", { level })}
                </h4>
                <Badge
                  className={cn(items.length > maxOf(level) && "text-danger")}
                >
                  {t("spellSlots.preparedCount", {
                    count: items.length,
                    max: maxOf(level),
                  })}
                </Badge>
              </div>
              <ul className="flex flex-col gap-1">
                {items.map(({ entry, index }) => (
                  <li
                    key={index}
                    className="flex items-center justify-between gap-2 rounded px-2 py-0.5 hover:bg-raised-hover"
                  >
                    <span className="min-w-0 text-body break-words">
                      {label(entry)}
                    </span>
                    <Button
                      size="xs"
                      variant="ghost"
                      aria-label={t("spellSlots.remove", {
                        name: label(entry),
                      })}
                      onClick={() =>
                        setDraft((d) => d.filter((_, i) => i !== index))
                      }
                    >
                      ×
                    </Button>
                  </li>
                ))}
              </ul>
            </div>
          );
        })}
        <div className="mt-auto flex justify-end gap-2 pt-2">
          <Button size="sm" onClick={onCancel} disabled={busy}>
            {t("spellSlots.cancel")}
          </Button>
          <Button
            size="sm"
            title={t("spellSlots.save")}
            aria-label={t("spellSlots.save")}
            onClick={() => onSave(draft)}
            disabled={busy}
          >
            {t("spellSlots.saveShort")}
          </Button>
        </div>
      </section>
    </div>
  );
}
