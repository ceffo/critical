import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Check, Plus, X } from "@phosphor-icons/react";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import { Button } from "@/components/ui/button";
import { useSpellName } from "@/i18n/spellName";
import { fold } from "@/lib/catalogue";
import type { SpellOption, StartingSpellbook } from "@/types/character";
import { Note } from "./kit";
import { draftSpellbook, spellbookState, type StepProps } from "./flow";

/**
 * The creation's spellbook step (ADR 0030 step 20.7), a mage's or a bard's: the spells the DM gave,
 * ticked without a learn roll. The limits and the count per spell level are the draft's answer
 * (`spellbook`, read at the starting level); the catalogue offers the wizard spells of the levels
 * the answer allows, and a spell the server refuses shows its refusal on its row. A mage's Read
 * Magic and Detect Magic come pre-ticked from the options and may be removed. Never blocks Suivant.
 */
export function SpellbookStep(props: StepProps) {
  const { options, draft, update, errorFor, answer } = props;
  const { t, i18n } = useTranslation();
  const name = useSpellName();
  const [query, setQuery] = useState("");
  const book = answer.latest?.spellbook ?? null;
  const state = spellbookState(answer.latest);
  const spells = draftSpellbook(options, draft);
  const levelOf = new Map(
    (options.wizardSpells ?? []).map((s) => [s.id, s.level]),
  );
  const set = (next: string[]) => update({ spellbook: next });
  const className = t(`details.classes.${draft.cls}`, {
    defaultValue: draft.cls,
  }).toLocaleLowerCase(i18n.language);
  const budgetError = errorFor("spellbook");

  return (
    <div className="flex min-w-0 flex-col gap-4" data-testid="spellbook-step">
      {book && <Limits book={book} />}
      {state === "wait" && (
        <Note kind="info" testId="spellbook-wait">
          {t("creation.spellbook.wait")}
        </Note>
      )}
      {state === "none" && (
        <Note kind="info" testId="spellbook-none">
          {t("creation.spellbook.noLevel", { className })}
        </Note>
      )}
      {budgetError && <Note kind="refusal">{budgetError}</Note>}
      <div className="grid grid-cols-[repeat(auto-fit,minmax(18rem,1fr))] items-start gap-5">
        <section
          aria-labelledby="spellbook-picks-title"
          className="flex min-w-0 flex-col gap-2"
        >
          <h2
            id="spellbook-picks-title"
            className="group-heading section-label px-2.5"
          >
            {t("creation.spellbook.picked", { count: spells.length })}
          </h2>
          {spells.length === 0 ? (
            <p className="px-2.5 text-body break-words text-ink-muted">
              {t("creation.spellbook.empty")}
            </p>
          ) : (
            <ul className="flex flex-col gap-1" data-testid="spellbook-picks">
              {spells.map((id) => (
                <PickedSpell
                  key={id}
                  name={name(id)}
                  level={levelOf.get(id)}
                  error={errorFor(`spellbook.${id}`)}
                  onRemove={() => set(spells.filter((s) => s !== id))}
                  testId={`spellbook-pick-${id}`}
                />
              ))}
            </ul>
          )}
        </section>
        {book && book.levels.length > 0 && (
          <section
            aria-labelledby="spellbook-catalogue-title"
            className="flex min-w-0 flex-col gap-2"
            data-testid="spellbook-catalogue"
          >
            <h2
              id="spellbook-catalogue-title"
              className="group-heading section-label px-2.5"
            >
              {t("creation.spellbook.catalogue")}
            </h2>
            <div className="flex flex-col gap-1 px-2.5">
              <label htmlFor="spellbook-search" className={labelClass}>
                {t("creation.spellbook.search")}
              </label>
              <input
                id="spellbook-search"
                type="search"
                className={fieldClass}
                autoComplete="off"
                placeholder={t("creation.spellbook.searchPlaceholder")}
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                data-testid="spellbook-search"
              />
            </div>
            {book.levels.map((l) => (
              <LevelGroup
                key={l.level}
                level={l.level}
                count={l.count}
                book={book}
                rows={matching(
                  options.wizardSpells ?? [],
                  l.level,
                  query,
                  name,
                )}
                picked={spells}
                onAdd={(id) => set([...spells, id])}
              />
            ))}
          </section>
        )}
      </div>
    </div>
  );
}

/** The wizard spells of `level` whose name (in the UI language) matches the query, by name. */
function matching(
  all: SpellOption[],
  level: number,
  query: string,
  name: (id: string) => string,
): { id: string; name: string }[] {
  const q = fold(query.trim());
  return all
    .filter((s) => s.level === level)
    .map((s) => ({ id: s.id, name: name(s.id) }))
    .filter((s) => q === "" || fold(s.name).includes(q))
    .sort((a, b) => a.name.localeCompare(b.name));
}

/** The limits of step 20.1 at the starting level, as the server reads them. */
function Limits({ book }: { book: StartingSpellbook }) {
  const { t } = useTranslation();
  return (
    <p
      className="text-body break-words text-ink-muted tabular-nums"
      data-testid="spellbook-limits"
    >
      {[
        t("creation.spellbook.classMax", { level: book.classMaxSpellLevel }),
        t("creation.spellbook.intMax", { level: book.intMaxSpellLevel }),
        book.noMaximum
          ? t("creation.spellbook.noMaximum")
          : t("creation.spellbook.perLevel", {
              count: book.maxSpellsPerLevel,
            }),
      ].join(" · ")}
    </p>
  );
}

/** One ticked spell: its name and level, the server's refusal of it, and the button that removes it. */
function PickedSpell({
  name,
  level,
  error,
  onRemove,
  testId,
}: {
  name: string;
  level: number | undefined;
  error: string | null;
  onRemove: () => void;
  testId: string;
}) {
  const { t } = useTranslation();
  return (
    <li
      className="flex flex-col gap-1 rounded-md bg-raised px-2.5 py-1"
      data-testid={testId}
    >
      <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3">
        <span className="flex min-w-0 flex-col">
          <span className="text-body break-words">{name}</span>
          {level !== undefined && (
            <span className="text-label break-words text-ink-muted">
              {t("creation.spellbook.levelN", { level })}
            </span>
          )}
        </span>
        <Button
          type="button"
          size="icon-xs"
          aria-label={t("creation.spellbook.remove", { name })}
          title={t("creation.spellbook.remove", { name })}
          onClick={onRemove}
          data-testid="spellbook-remove"
        >
          <X aria-hidden />
        </Button>
      </div>
      {error && (
        <p role="alert" className="text-label break-words text-danger">
          {error}
        </p>
      )}
    </li>
  );
}

/** One spell level of the catalogue: its heading with the spells entered against Table 4's maximum,
 * and its spells, each a button that ticks it. */
function LevelGroup({
  level,
  count,
  book,
  rows,
  picked,
  onAdd,
}: {
  level: number;
  count: number;
  book: StartingSpellbook;
  rows: { id: string; name: string }[];
  picked: string[];
  onAdd: (id: string) => void;
}) {
  const { t } = useTranslation();
  const title = t("creation.spellbook.levelN", { level });
  const counted = book.noMaximum
    ? t("creation.spellbook.countNoMax", { count })
    : t("creation.spellbook.countOf", { count, max: book.maxSpellsPerLevel });
  return (
    <div
      className="flex flex-col gap-1"
      data-testid={`spellbook-level-${level}`}
    >
      <h3 className="flex flex-wrap items-baseline justify-between gap-x-3 px-2.5 text-label font-medium text-ink-muted">
        <span className="break-words">{title}</span>
        <span className="tabular-nums" data-testid="spellbook-level-count">
          {counted}
        </span>
      </h3>
      {rows.length === 0 ? (
        <p className="px-2.5 text-body break-words text-ink-muted">
          {t("creation.spellbook.noMatch")}
        </p>
      ) : (
        <ul className="flex flex-col gap-1" aria-label={title}>
          {rows.map((r) => {
            const has = picked.includes(r.id);
            const label = has
              ? t("creation.spellbook.alreadyPicked", { name: r.name })
              : t("creation.spellbook.add", { name: r.name });
            return (
              <li key={r.id}>
                <button
                  type="button"
                  className="flex min-h-7 w-full items-center gap-1.5 rounded-md bg-raised px-2.5 py-0.5 text-left text-body transition-colors outline-none hover:bg-raised-hover focus-visible:ring-3 focus-visible:ring-ring/50 disabled:text-ink-muted"
                  disabled={has}
                  aria-label={label}
                  title={label}
                  onClick={() => onAdd(r.id)}
                  data-testid={`spellbook-add-${r.id}`}
                >
                  {has ? (
                    <Check aria-hidden className="size-3.5 shrink-0" />
                  ) : (
                    <Plus aria-hidden className="size-3.5 shrink-0" />
                  )}
                  <span className="min-w-0 break-words">{r.name}</span>
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
