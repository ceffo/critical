import { useState, type FormEvent } from "react";
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
  toDraft,
  unsetDetails,
  toPatch,
  type Draft,
} from "@/lib/characterDetails";
import { useCreationOptions } from "@/lib/useCreationOptions";
import { IdentityRoll } from "@/components/dashboard/IdentityRoll";
import { heightField, unitsFor, weightField } from "@/lib/measures";
import { DetailRows } from "@/components/dashboard/DetailRows";
import { useCharacterStore } from "@/stores/character";
import type { ApiError } from "@/api/http";
import type { Character } from "@/types/character";
import { primaryEntry } from "@/lib/classEntries";
import { TakeClassDialog } from "@/components/dashboard/TakeClassDialog";
import { SwitchClassDialog } from "@/components/dashboard/SwitchClassDialog";
import { switchableEntries } from "@/lib/classSwitch";

/**
 * The character's name, opening a popover with the details: the name first (Rename lives here too,
 * ADR 0034 step 7.2), class and level read-only, the rest as the player entered it, switched to a
 * form by "Edit". The server validates every field and names the rejected one, whose error shows
 * under it.
 */
export function CharacterDetails({
  character,
}: {
  character: Character | null;
}) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(false);
  const [open, setOpen] = useState(false);
  const [taking, setTaking] = useState(false);
  const [switching, setSwitching] = useState(false);
  const name = character?.name || t("app.loading");

  return (
    <>
      <Popover
        open={open}
        onOpenChange={(next) => {
          setOpen(next);
          if (!next) setEditing(false);
        }}
      >
        <PopoverTrigger
          disabled={!character}
          title={t("details.open")}
          data-testid="details-trigger"
          className="hit-24-up block max-w-full break-words rounded-sm text-left text-key hover:text-amb-300 data-popup-open:text-amb-300"
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
                onTakeClass={() => {
                  setOpen(false);
                  setTaking(true);
                }}
                onSwitchClass={() => {
                  setOpen(false);
                  setSwitching(true);
                }}
              />
            )}
          </PopoverContent>
        )}
      </Popover>
      {character && (
        <TakeClassDialog
          character={character}
          open={taking}
          onOpenChange={setTaking}
        />
      )}
      {character && (
        <SwitchClassDialog
          character={character}
          open={switching}
          onOpenChange={setSwitching}
        />
      )}
    </>
  );
}

function DetailsView({
  character: c,
  onEdit,
  onTakeClass,
  onSwitchClass,
}: {
  character: Character;
  onEdit: () => void;
  onTakeClass: () => void;
  onSwitchClass: () => void;
}) {
  const { t } = useTranslation();
  const { options } = useCreationOptions();
  const canSwitch = switchableEntries(c, options).length > 0;
  // Only what is set shows; one link to the form stands for every field left out.
  const hasUnset = unsetDetails(c).length > 0;
  return (
    <>
      <DetailRows character={c} withIdentity />
      <div className="flex flex-wrap justify-end gap-2">
        <Button
          type="button"
          variant="ghost"
          onClick={onTakeClass}
          data-testid="take-class-open"
        >
          {t("takeClass.open")}
        </Button>
        {canSwitch && (
          <Button
            type="button"
            variant="ghost"
            onClick={onSwitchClass}
            title={t("switchClass.openTitle")}
            aria-label={t("switchClass.openTitle")}
            data-testid="switch-class-open"
          >
            {t("switchClass.open")}
          </Button>
        )}
        {hasUnset && (
          <Button
            type="button"
            variant="ghost"
            onClick={onEdit}
            data-testid="details-set"
          >
            {t("details.set")}
          </Button>
        )}
        <Button type="button" onClick={onEdit}>
          {t("details.edit")}
        </Button>
      </div>
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
  const [draft, setDraft] = useState<Draft>(() =>
    toDraft(character, i18n.language),
  );
  const [name, setName] = useState(character.name);
  const [error, setError] = useState<{ field: string; code: string } | null>(
    null,
  );
  const [busy, setBusy] = useState(false);
  const primary = primaryEntry(character.classes)?.class ?? "";
  // The lists the picks are made from come from the server (ADR 0032 conventions): the class's
  // admitted alignments and the sexes.
  const { options, failed: optionsFailed, retry } = useCreationOptions(true);
  const alignments =
    options?.classes.find((c) => c.class === primary)?.alignments ?? [];
  const sexes = options?.sexes ?? [];
  const set = (k: Exclude<keyof Draft, "languages">) => (v: string) =>
    setDraft((d) => ({ ...d, [k]: v }));

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (busy) return;
    const { body, ageInvalid, invalid } = toPatch(
      draft,
      i18n.language,
      character,
    );
    if (ageInvalid) {
      setError({ field: "age", code: "out_of_range" });
      return;
    }
    if (invalid.length > 0) {
      setError({ field: invalid[0], code: "not_a_number" });
      return;
    }
    setBusy(true);
    const rejected: ApiError | null = await save(body, name);
    setBusy(false);
    if (rejected)
      setError({ field: rejected.field ?? "", code: rejected.message });
    else onDone();
  };

  const message = (field: string) => {
    if (error?.field !== field) return null;
    // The name's refusals are the rename's, worded as on the roster (ADR 0029 step 7.3).
    if (field === "name")
      return (
        <span role="alert" className="text-label text-danger">
          {t(`characters.errors.${error.code}`, {
            max: options?.maxNameLength,
            defaultValue: error.code,
          })}
        </span>
      );
    const fieldKey = `details.errors.${field}.${error.code}`;
    const key = i18n.exists(fieldKey)
      ? fieldKey
      : `details.errors.${error.code}`;
    return (
      <span role="alert" className="text-label text-danger">
        {i18n.exists(key)
          ? t(key, {
              className: i18n.exists(`details.classes.${primary}`)
                ? t(`details.classes.${primary}`)
                : primary,
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
  const units = unitsFor(i18n.language);
  // A rolled figure fills its field, in the units the player types in.
  const filled = {
    age: (v: number) => setDraft((d) => ({ ...d, age: String(v) })),
    height: (v: number) =>
      setDraft((d) => ({
        ...d,
        height: heightField(v, units, i18n.language),
        rolledHeight: v,
      })),
    weight: (v: number) =>
      setDraft((d) => ({
        ...d,
        weight: weightField(v, units, i18n.language),
        rolledWeight: v,
      })),
  };
  const input = (
    field: Exclude<keyof Draft, "languages">,
    props: {
      inputMode?: "numeric";
      hint?: string;
      roll?: "age" | "height" | "weight";
    } = {},
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
      {props.roll && (
        <IdentityRoll
          figure={props.roll}
          raceCode={character.raceCode}
          sex={draft.sex}
          onRolled={filled[props.roll]}
        />
      )}
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
          <Button type="button" variant="ghost" onClick={retry}>
            {t("details.optionsRetry")}
          </Button>
        </div>
      )}
      <div className="grid grid-cols-2 gap-x-3 gap-y-2">
        <div className="col-span-2 flex min-w-0 flex-col gap-1 text-body">
          <label
            htmlFor="details-name"
            className="text-label text-muted-foreground"
          >
            {t("characters.name")}
          </label>
          <input
            id="details-name"
            autoComplete="off"
            className={fieldClass}
            value={name}
            aria-invalid={error?.field === "name"}
            onChange={(e) => setName(e.target.value)}
          />
          {message("name")}
        </div>
        {pick("alignment", alignments)}
        {input("age", { inputMode: "numeric", roll: "age" })}
        {pick("sex", sexes)}
        {input("height", {
          hint: t(`details.unitHint.height.${units}`),
          roll: "height",
        })}
        {input("weight", {
          hint: t(`details.unitHint.weight.${units}`),
          roll: "weight",
        })}
        {input("hair")}
        {input("eyes")}
      </div>
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
