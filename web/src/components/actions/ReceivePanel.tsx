import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { searchSpells } from "@/api/spells_db";
import { resources } from "@/i18n";
import { spellId, spellName } from "@/i18n/spellName";
import { useSpellStore } from "@/stores/spells";
import { useTableStore } from "@/stores/table";
import { captionClass, fieldClass } from "@/components/dashboard/fieldStyles";

interface Candidate {
  id: string;
  name: string;
  level?: number;
  cls?: string;
}

/** Lowercase without accents, for matching typed text against names. */
function fold(s: string): string {
  return s.normalize("NFD").replace(/[̀-ͯ]/g, "").toLowerCase();
}

/** Spells whose English or French catalogue name contains the query (the spell database is English only). */
function catalogueMatches(q: string): Candidate[] {
  const en = resources.en.spells as Record<string, string>;
  const fr = resources.fr.spells as Record<string, string>;
  const needle = fold(q);
  return Object.keys(en)
    .filter((id) => fold(en[id]).includes(needle) || fold(fr[id] ?? "").includes(needle))
    .map((id) => ({ id, name: en[id] }));
}

/** "Spell cast on me": find a spell by name (en/fr), enter the caster's level, apply it. No slot is used. */
export function ReceivePanel() {
  const { t } = useTranslation();
  const receiveSpell = useSpellStore((s) => s.receiveSpell);
  const { run, busy } = useTableStore();
  const [query, setQuery] = useState("");
  const [level, setLevel] = useState("");
  const [found, setFound] = useState<Candidate[]>([]);

  useEffect(() => {
    const q = query.trim();
    // Below two letters the hint shows instead of the (stale) results.
    if (q.length < 2) return;
    let live = true;
    const timer = setTimeout(() => {
      searchSpells(q)
        .catch(() => [])
        .then((rows) => {
          if (!live) return;
          const byId = new Map<string, Candidate>();
          for (const c of catalogueMatches(q)) byId.set(c.id, c);
          for (const r of rows) {
            const id = spellId(r.name);
            if (!byId.has(id) || byId.get(id)?.level === undefined)
              byId.set(id, { id, name: r.name, level: r.level, cls: r.class });
          }
          setFound([...byId.values()].slice(0, 12));
        });
    }, 200);
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [query]);

  const casterLevel = Number(level);
  const levelOk = Number.isInteger(casterLevel) && casterLevel >= 1 && casterLevel <= 40;

  return (
    <section
      className="flex flex-col gap-2 rounded-lg border p-3"
      aria-label={t("spellSlots.receiveTitle")}
    >
      <h3 className="text-sm font-semibold">{t("spellSlots.receiveTitle")}</h3>
      <div className="grid grid-cols-[1fr_8rem] gap-2">
        <label className="flex min-w-0 flex-col gap-1">
          <span className={captionClass}>{t("spellSlots.receiveSearchLabel")}</span>
          <input
            className={fieldClass}
            type="search"
            value={query}
            placeholder={t("spellSlots.receiveSearch")}
            onChange={(e) => setQuery(e.target.value)}
          />
        </label>
        <label className="flex min-w-0 flex-col gap-1">
          <span className={captionClass}>{t("spellSlots.casterLevel")}</span>
          <input
            className={fieldClass}
            type="number"
            min={1}
            max={40}
            value={level}
            onChange={(e) => setLevel(e.target.value)}
          />
        </label>
      </div>
      {query.trim().length < 2 ? (
        <p className="text-xs text-muted-foreground">{t("spellSlots.receiveHint")}</p>
      ) : found.length === 0 ? (
        <p className="text-xs text-muted-foreground">{t("spellSlots.noMatch")}</p>
      ) : (
        <ul className="flex max-h-48 flex-col gap-1 overflow-y-auto">
          {found.map((c) => {
            const name = spellName(t, c.id, c.name);
            return (
              <li key={c.id} className="flex items-center justify-between gap-2 rounded px-2 py-1 hover:bg-muted/60">
                <span className="min-w-0 truncate text-sm">
                  {name}
                  {c.cls && (
                    <span className="text-muted-foreground">
                      {" "}
                      · {c.cls} {c.level}
                    </span>
                  )}
                </span>
                <Button
                  size="xs"
                  variant="secondary"
                  disabled={busy || !levelOk}
                  aria-label={t("spellSlots.receiveLabel", { name })}
                  onClick={() => void run(() => receiveSpell(c.id, casterLevel))}
                >
                  {t("spellSlots.receive")}
                </Button>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
