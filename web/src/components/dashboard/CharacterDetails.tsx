import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
} from "@/components/ui/popover";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import {
  TEXT_FIELDS,
  ALIGNMENT_CODES,
  SEX_CODES,
  codeOrEmpty,
  hasAlignmentWarning,
  languageWarning,
  toBody,
  toDraft,
  type Draft,
} from "@/lib/characterDetails";
import { fetchCreationChoices, fetchCreationOptions } from "@/api/characters";
import { LanguagePicker } from "@/components/characters/LanguagePicker";
import { useLanguageNames } from "@/lib/useLanguageNames";
import { useCharacterStore } from "@/stores/character";
import type { ApiError } from "@/api/http";
import type { Character, CreationOptions } from "@/types/character";

/**
 * The character's name, opening a popover with the details: class and level read-only, the rest
 * as the player entered it, switched to a form by "Edit". The server validates every field and
 * names the rejected one, whose error shows under it.
 */
export function CharacterDetails({
  character,
}: {
  character: Character | null;
}) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(false);
  const name = character?.name || t("app.loading");

  return (
    <Popover onOpenChange={(open) => !open && setEditing(false)}>
      <PopoverTrigger
        disabled={!character}
        title={t("details.open")}
        data-testid="details-trigger"
        className="block max-w-full break-words rounded-sm text-left text-key outline-none hover:text-amb-300 focus-visible:ring-3 focus-visible:ring-ring/50 data-popup-open:text-amb-300"
      >
        {name}
      </PopoverTrigger>
      {character && (
        <PopoverContent
          side="right"
          align="start"
          className="flex w-96 max-w-[calc(100vw-2rem)] flex-col gap-3"
          aria-label={t("details.title")}
          data-testid="details-popover"
        >
          <PopoverTitle className="section-label">
            {t("details.title")}
          </PopoverTitle>
          {editing ? (
            <DetailsForm
              character={character}
              onDone={() => setEditing(false)}
            />
          ) : (
            <DetailsView
              character={character}
              onEdit={() => setEditing(true)}
            />
          )}
        </PopoverContent>
      )}
    </Popover>
  );
}

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid grid-cols-[6rem_minmax(0,1fr)] gap-2 text-body">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="min-w-0 break-words tabular-nums">{children}</dd>
    </div>
  );
}

/**
 * The language catalogue, for the names (ADR 0032 step 4): loaded once the card opens, tolerant of
 * a release that predates it (its codes then show as they are). `loading` until it answers.
 */
function useLanguageCatalogue(): {
  options: CreationOptions | null;
  loading: boolean;
} {
  const [state, setState] = useState<{
    options: CreationOptions | null;
    loading: boolean;
  }>({ options: null, loading: true });
  useEffect(() => {
    let live = true;
    fetchCreationOptions().then(
      (options) => live && setState({ options, loading: false }),
      () => live && setState({ options: null, loading: false }),
    );
    return () => {
      live = false;
    };
  }, []);
  return state;
}

function DetailsView({
  character: c,
  onEdit,
}: {
  character: Character;
  onEdit: () => void;
}) {
  const { t, i18n } = useTranslation();
  const catalogue = useLanguageCatalogue();
  const languageName = useLanguageNames(catalogue.options);
  const languagesOver = languageWarning(c);
  const none = (
    <span className="text-muted-foreground">{t("details.none")}</span>
  );
  const classKey = `details.classes.${c.class}`;
  const text = (v: string) => (v === "" ? none : v);
  // ADR 0032 step 8.2: a required value the character lacks is asked for, not shown as "not set".
  const toPick = (
    <span className="text-danger" data-testid="details-missing">
      {t("details.missing")}
    </span>
  );
  // A value that is not a code (an old release's free text) reads as missing, never as a raw key.
  const alignment = codeOrEmpty(c.alignment, ALIGNMENT_CODES);
  const sex = codeOrEmpty(c.sex, SEX_CODES);
  const alignmentWarning = hasAlignmentWarning(c);
  return (
    <>
      <dl className="flex flex-col gap-2">
        <Row label={t("details.class")}>
          {i18n.exists(classKey) ? t(classKey) : c.class}
        </Row>
        <Row label={t("details.level")}>{c.derived.realizedLevel}</Row>
        <Row label={t("details.raceCode")}>
          {c.raceCode === "" ? none : t(`details.races.${c.raceCode}`)}
        </Row>
        <Row label={t("details.fields.alignment")}>
          {alignment === "" ? (
            toPick
          ) : (
            <>
              {t(`details.alignments.${alignment}`)}
              {alignmentWarning && (
                <span
                  className="block text-label text-muted-foreground"
                  data-testid="details-alignment-warning"
                >
                  {t("details.warnings.alignment_not_allowed", {
                    className: i18n.exists(classKey) ? t(classKey) : c.class,
                  })}
                </span>
              )}
            </>
          )}
        </Row>
        <Row label={t("details.fields.age")}>
          {c.age === null ? none : c.age}
        </Row>
        <Row label={t("details.fields.sex")}>
          {sex === "" ? toPick : t(`details.sexes.${sex}`)}
        </Row>
        {TEXT_FIELDS.map((f) => (
          <Row key={f} label={t(`details.fields.${f}`)}>
            {text(c[f])}
          </Row>
        ))}
        <Row label={t("details.fields.languages")}>
          {c.languages.length === 0
            ? none
            : catalogue.loading
              ? "…"
              : c.languages.map(languageName).join(", ")}
          {languagesOver && (
            <span
              className="block text-label text-muted-foreground"
              data-testid="details-languages-warning"
            >
              {t("details.warnings.languages_over_limit", {
                n: languagesOver.count,
                limit: languagesOver.limit,
              })}
            </span>
          )}
        </Row>
      </dl>
      <Button type="button" className="self-end" onClick={onEdit}>
        {t("details.edit")}
      </Button>
    </>
  );
}

function DetailsForm({
  character,
  onDone,
}: {
  character: Character;
  onDone: () => void;
}) {
  const { t, i18n } = useTranslation();
  const save = useCharacterStore((s) => s.saveDetails);
  const [draft, setDraft] = useState<Draft>(() => toDraft(character));
  const [error, setError] = useState<{ field: string; code: string } | null>(
    null,
  );
  const [busy, setBusy] = useState(false);
  // The lists the picks are made from come from the server (ADR 0032 conventions): the class's
  // admitted alignments and the sexes.
  const [options, setOptions] = useState<CreationOptions | null>(null);
  const [optionsFailed, setOptionsFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let live = true;
    fetchCreationChoices().then(
      (o) => {
        if (!live) return;
        setOptions(o);
        setOptionsFailed(false);
      },
      () => live && setOptionsFailed(true),
    );
    return () => {
      live = false;
    };
  }, [attempt]);
  const alignments =
    options?.classes.find((c) => c.class === character.class)?.alignments ?? [];
  const sexes = options?.sexes ?? [];
  const set = (k: Exclude<keyof Draft, "languages">) => (v: string) =>
    setDraft((d) => ({ ...d, [k]: v }));

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (busy) return;
    const { body, ageInvalid } = toBody(draft);
    if (ageInvalid) {
      setError({ field: "age", code: "out_of_range" });
      return;
    }
    setBusy(true);
    const rejected: ApiError | null = await save(body);
    setBusy(false);
    if (rejected)
      setError({ field: rejected.field ?? "", code: rejected.message });
    else onDone();
  };

  const message = (field: string) => {
    if (error?.field !== field) return null;
    const fieldKey = `details.errors.${field}.${error.code}`;
    const key = i18n.exists(fieldKey)
      ? fieldKey
      : `details.errors.${error.code}`;
    return (
      <span role="alert" className="text-label text-danger">
        {i18n.exists(key)
          ? t(key, {
              className: i18n.exists(`details.classes.${character.class}`)
                ? t(`details.classes.${character.class}`)
                : character.class,
            })
          : error.code}
      </span>
    );
  };
  // A pick among the server's codes: an empty value is the "pick" prompt, kept while the draft
  // holds nothing (a character that lacks it); the stored value stays offered whatever the list.
  const pick = (field: "alignment" | "sex", codes: string[]) => {
    const offered = codes.includes(draft[field])
      ? codes
      : draft[field] === ""
        ? codes
        : [draft[field], ...codes];
    return (
      <div key={field} className="flex min-w-0 flex-col gap-1 text-body">
        <label
          htmlFor={`details-${field}`}
          className="text-label text-muted-foreground"
        >
          {t(`details.fields.${field}`)}
        </label>
        <select
          id={`details-${field}`}
          className={fieldClass}
          value={draft[field]}
          aria-invalid={error?.field === field}
          onChange={(e) => set(field)(e.target.value)}
        >
          {draft[field] === "" && <option value="">{t("details.pick")}</option>}
          {offered.map((code) => (
            <option key={code} value={code}>
              {t(`details.${field === "sex" ? "sexes" : "alignments"}.${code}`)}
            </option>
          ))}
        </select>
        {message(field)}
      </div>
    );
  };
  const input = (
    field: Exclude<keyof Draft, "languages">,
    props: { inputMode?: "numeric"; hint?: string } = {},
  ) => (
    <div key={field} className="flex min-w-0 flex-col gap-1 text-body">
      <label
        htmlFor={`details-${field}`}
        className="text-label text-muted-foreground"
      >
        {t(`details.fields.${field}`)}
      </label>
      <input
        id={`details-${field}`}
        className={fieldClass}
        value={draft[field]}
        inputMode={props.inputMode}
        aria-invalid={error?.field === field}
        onChange={(e) => set(field)(e.target.value)}
      />
      {props.hint && (
        <span className="text-label text-muted-foreground">{props.hint}</span>
      )}
      {message(field)}
    </div>
  );

  return (
    <form onSubmit={(e) => void submit(e)} className="flex flex-col gap-3">
      {optionsFailed && (
        <div
          role="alert"
          data-testid="details-options-error"
          className="flex flex-wrap items-center gap-2 text-label text-danger"
        >
          {t("details.optionsError")}
          <Button
            type="button"
            variant="ghost"
            onClick={() => {
              setOptionsFailed(false);
              setAttempt((n) => n + 1);
            }}
          >
            {t("details.optionsRetry")}
          </Button>
        </div>
      )}
      <div className="grid grid-cols-2 gap-x-3 gap-y-2">
        {pick("alignment", alignments)}
        {input("age", { inputMode: "numeric" })}
        {pick("sex", sexes)}
        {input("height")}
        {input("weight")}
        {input("hair")}
        {input("eyes")}
      </div>
      {options && (options.languages ?? []).length > 0 && (
        <div className="flex min-w-0 flex-col gap-1 text-body">
          <span className="text-label text-muted-foreground">
            {t("details.fields.languages")}
          </span>
          <LanguagePicker
            idPrefix="details-languages"
            options={options}
            race={character.raceCode}
            picked={draft.languages}
            onChange={(languages) => setDraft((d) => ({ ...d, languages }))}
          />
          {message("languages")}
        </div>
      )}
      <div className="flex justify-end gap-2">
        <Button type="button" variant="ghost" onClick={onDone}>
          {t("details.cancel")}
        </Button>
        <Button type="submit" disabled={busy}>
          {busy ? t("details.saving") : t("details.save")}
        </Button>
      </div>
    </form>
  );
}
