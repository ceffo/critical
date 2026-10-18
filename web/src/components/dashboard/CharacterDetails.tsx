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
  languageKey,
  toBody,
  toDraft,
  type Draft,
} from "@/lib/characterDetails";
import { fetchCreationOptions } from "@/api/characters";
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

/** A language as entered, or its translation when the catalogue has one. */
function useLanguageName() {
  const { t, i18n } = useTranslation();
  return (name: string) => {
    const key = `details.language.${languageKey(name)}`;
    return i18n.exists(key) ? t(key) : name;
  };
}

function DetailsView({
  character: c,
  onEdit,
}: {
  character: Character;
  onEdit: () => void;
}) {
  const { t, i18n } = useTranslation();
  const languageName = useLanguageName();
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
  const alignmentKnown = i18n.exists(`details.alignments.${c.alignment}`);
  const sexKnown = i18n.exists(`details.sexes.${c.sex}`);
  const alignmentWarning = c.warnings.some(
    (w) => w.code === "alignment_not_allowed" && w.field === "alignment",
  );
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
          {c.alignment === "" || !alignmentKnown ? (
            toPick
          ) : (
            <>
              {t(`details.alignments.${c.alignment}`)}
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
          {c.sex === "" || !sexKnown ? toPick : t(`details.sexes.${c.sex}`)}
        </Row>
        {TEXT_FIELDS.map((f) => (
          <Row key={f} label={t(`details.fields.${f}`)}>
            {text(c[f])}
          </Row>
        ))}
        <Row label={t("details.fields.languages")}>
          {c.languages.length === 0
            ? none
            : c.languages.map(languageName).join(", ")}
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
  useEffect(() => {
    let live = true;
    fetchCreationOptions().then(
      (o) => live && setOptions(o),
      () => undefined,
    );
    return () => {
      live = false;
    };
  }, []);
  const alignments =
    options?.classes.find((c) => c.class === character.class)?.alignments ?? [];
  const sexes = options?.sexes ?? [];
  const set = (k: keyof Draft) => (v: string) =>
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
    field: keyof Draft,
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
      <div className="grid grid-cols-2 gap-x-3 gap-y-2">
        {pick("alignment", alignments)}
        {input("age", { inputMode: "numeric" })}
        {pick("sex", sexes)}
        {input("height")}
        {input("weight")}
        {input("hair")}
        {input("eyes")}
      </div>
      {input("languages", { hint: t("details.languagesHint") })}
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
