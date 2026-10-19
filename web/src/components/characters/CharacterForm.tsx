import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { UserPlus } from "@phosphor-icons/react";
import { createCharacter, fetchCreationChoices } from "@/api/characters";
import { ApiError } from "@/api/http";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import { Button } from "@/components/ui/button";
import { parseWhole } from "@/lib/wholeNumber";
import { toBody, type Draft } from "@/lib/characterDetails";
import { skillName } from "@/lib/logSummary";
import { cn } from "@/lib/utils";
import {
  ABILITIES,
  type CreationOptions,
  type NewCharacter,
  type ValueRange,
} from "@/types/character";

type NumberField = (typeof ABILITIES)[number] | "hpMax" | "xp";
type Field =
  | NumberField
  | "name"
  | "class"
  | "raceCode"
  | "strExceptional"
  | "skillArmor"
  | "skillPoints"
  | `skillPoints.${string}`
  | `details.${keyof Draft}`;

/** The details the form takes, in the card's order (the card's edit checks them the same way). */
const DETAIL_FIELDS: (keyof Draft)[] = [
  "age",
  "height",
  "weight",
  "hair",
  "eyes",
];

const NO_DETAILS: Draft = {
  alignment: "",
  age: "",
  sex: "",
  height: "",
  weight: "",
  hair: "",
  eyes: "",
  languages: "",
};

/** The server's field names for the form's: the race is `race` in its errors. */
const ERROR_FIELD: Record<string, Field> = { race: "raceCode" };

interface FieldError {
  field: Field;
  code: string;
  /** The server's message, shown when the web has no wording for the code. */
  message?: string;
}

/**
 * The new character form (ADR 0029 step 7.2, ADR 0030 step 1): the class (the ones the server can
 * create), the race (the ones the class admits, DMG Table 7), the name, the six ability scores as
 * on the sheet, the max HP and the starting XP; a thief its skill points and the armour worn for its
 * skills (ADR 0030 step 17: the percentages are the server's); and, optional, the details the
 * character card shows (alignment, age, languages...), which the server checks as the card's edit. Level, THAC0 and saves are never entered: the server
 * derives them. It starts with no equipment. The server checks every value and refuses with the
 * field; the form words its code. On success the page reloads on the new character, now current.
 */
export function CharacterForm({ onCancel }: { onCancel?: () => void }) {
  const { t } = useTranslation();
  const [options, setOptions] = useState<CreationOptions | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [name, setName] = useState("");
  const [cls, setCls] = useState("");
  const [race, setRace] = useState("");
  const [numbers, setNumbers] = useState<Record<NumberField, string>>({
    str: "",
    dex: "",
    con: "",
    int: "",
    wis: "",
    cha: "",
    hpMax: "",
    xp: "0",
  });
  // A warrior's 18/xx (ADR 0030 step 9), blank for none; offered at Strength 18.
  const [strExceptional, setStrExceptional] = useState("");
  // ADR 0030 steps 1 and 17: a thief's points by skill and the armour worn for its skills.
  const [skillPoints, setSkillPoints] = useState<Record<string, string>>({});
  const [skillArmor, setSkillArmor] = useState("leather");
  const [details, setDetails] = useState<Draft>(NO_DETAILS);
  const [error, setError] = useState<FieldError | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let live = true;
    fetchCreationChoices().then(
      (o) => {
        if (!live) return;
        setOptions(o);
        const first = o.classes[0];
        if (first) {
          setCls(first.class);
          setRace(first.races[0] ?? "");
        }
      },
      (e: unknown) => live && setLoadError(String((e as Error).message ?? e)),
    );
    return () => {
      live = false;
    };
  }, [attempt]);

  const classOption = options?.classes.find((c) => c.class === cls);
  const races = classOption?.races ?? [];
  const alignments = classOption?.alignments ?? [];
  const skills = classOption?.skills ?? [];
  const armors = classOption?.skillArmors ?? [];
  const offersExceptional =
    !!classOption?.exceptionalStrengthRaces.includes(race) &&
    parseWhole(numbers.str) === 18;
  const rangeOf = (f: NumberField): ValueRange | undefined =>
    options
      ? f === "hpMax"
        ? options.hpMax
        : f === "xp"
          ? options.xp
          : options.ability
      : undefined;

  const wording = (e: FieldError): string => {
    if (e.field.startsWith("details.")) {
      const detail = e.field.slice("details.".length);
      return t(
        [`details.errors.${detail}.${e.code}`, `details.errors.${e.code}`],
        {
          className: t(`details.classes.${cls}`, { defaultValue: cls }),
          defaultValue: e.message ?? e.code,
        },
      );
    }
    const r =
      e.field === "name"
        ? { max: options?.maxNameLength }
        : e.field === "strExceptional"
          ? options?.strExceptional
          : e.field.startsWith("skillPoints.")
            ? options?.skillPoints
            : e.field in numbers
              ? rangeOf(e.field as NumberField)
              : undefined;
    const key = `characters.errors.${e.code}`;
    const className = t(`details.classes.${cls}`, { defaultValue: cls });
    return t(key, {
      ...r,
      className,
      defaultValue: e.message ?? e.code,
    });
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const body: NewCharacter = {
      name,
      class: cls,
      raceCode: race,
      str: null,
      dex: null,
      con: null,
      int: null,
      wis: null,
      cha: null,
      hpMax: null,
      xp: null,
      strExceptional: 0,
    };
    for (const f of Object.keys(numbers) as NumberField[]) {
      const v = parseWhole(numbers[f]);
      if (Number.isNaN(v)) {
        setError({ field: f, code: "out_of_range" });
        return;
      }
      body[f] = v;
    }
    if (offersExceptional && strExceptional.trim() !== "") {
      const v = parseWhole(strExceptional);
      if (Number.isNaN(v) || v === null) {
        setError({ field: "strExceptional", code: "out_of_range" });
        return;
      }
      body.strExceptional = v;
    }
    if (skills.length > 0) {
      body.skillArmor = skillArmor;
      body.skillPoints = {};
      for (const skill of skills) {
        const raw = skillPoints[skill] ?? "";
        if (raw.trim() === "") continue;
        const v = parseWhole(raw);
        if (Number.isNaN(v) || v === null) {
          setError({ field: `skillPoints.${skill}`, code: "out_of_range" });
          return;
        }
        body.skillPoints[skill] = v;
      }
    }
    const { body: detailsBody, ageInvalid } = toBody(details);
    if (ageInvalid) {
      setError({ field: "details.age", code: "out_of_range" });
      return;
    }
    body.details = detailsBody;
    setBusy(true);
    setError(null);
    try {
      await createCharacter(body);
      globalThis.location.assign("/");
    } catch (err) {
      setBusy(false);
      if (err instanceof ApiError && err.field) {
        setError({
          field: ERROR_FIELD[err.field] ?? (err.field as Field),
          code: err.message,
          message: err.message,
        });
      } else {
        setError({
          field: "name",
          code: "",
          message: err instanceof Error ? err.message : String(err),
        });
      }
    }
  };

  const errorFor = (f: Field) =>
    error?.field === f ? (
      <p
        id={`new-character-${f}-error`}
        role="alert"
        className="text-label text-destructive"
        data-testid="field-error"
      >
        {wording(error)}
      </p>
    ) : null;

  const fieldProps = (f: Field) => ({
    id: `new-character-${f}`,
    "aria-invalid": error?.field === f ? true : undefined,
    "aria-describedby":
      error?.field === f ? `new-character-${f}-error` : undefined,
    className: cn(fieldClass, error?.field === f && "border-destructive"),
  });

  const number = (f: NumberField, label: string, hint?: string) => {
    const r = rangeOf(f);
    return (
      <div className="flex min-w-0 flex-col gap-1">
        <label htmlFor={`new-character-${f}`} className={labelClass}>
          {label}
        </label>
        <input
          {...fieldProps(f)}
          inputMode="numeric"
          title={
            hint ??
            (r ? t("characters.range", { min: r.min, max: r.max }) : undefined)
          }
          value={numbers[f]}
          onChange={(e) => {
            setNumbers((n) => ({ ...n, [f]: e.target.value }));
            setError(null);
          }}
        />
        {errorFor(f)}
      </div>
    );
  };

  const detail = (f: keyof Draft, hint?: string) => {
    const field: Field = `details.${f}`;
    return (
      <div key={f} className="flex min-w-0 flex-col gap-1">
        <label
          htmlFor={`new-character-${field}`}
          className={cn(labelClass, "break-words")}
        >
          {t(`details.fields.${f}`)}
        </label>
        <input
          {...fieldProps(field)}
          autoComplete="off"
          inputMode={f === "age" ? "numeric" : undefined}
          value={details[f]}
          onChange={(e) => {
            setDetails((d) => ({ ...d, [f]: e.target.value }));
            setError(null);
          }}
        />
        {hint && (
          <span className="text-label text-muted-foreground">{hint}</span>
        )}
        {errorFor(field)}
      </div>
    );
  };

  // ADR 0032 steps 2 and 3: alignment and sex are required codes, picked from the server's lists
  // (the class's admitted alignments, the sexes).
  const pick = (f: "alignment" | "sex", codes: string[]) => {
    const field: Field = `details.${f}`;
    return (
      <div key={f} className="flex min-w-0 flex-col gap-1">
        <label
          htmlFor={`new-character-${field}`}
          className={cn(labelClass, "break-words")}
        >
          {t(`details.fields.${f}`)}
        </label>
        <select
          {...fieldProps(field)}
          value={details[f]}
          onChange={(e) => {
            setDetails((d) => ({ ...d, [f]: e.target.value }));
            setError(null);
          }}
        >
          <option value="">{t("details.pick")}</option>
          {codes.map((code) => (
            <option key={code} value={code}>
              {t(`details.${f === "sex" ? "sexes" : "alignments"}.${code}`)}
            </option>
          ))}
        </select>
        {errorFor(field)}
      </div>
    );
  };

  if (loadError)
    return (
      <div role="alert" className="flex flex-col items-start gap-2">
        <p className="text-body text-danger">{t("details.optionsError")}</p>
        <Button
          type="button"
          variant="ghost"
          onClick={() => {
            setLoadError(null);
            setAttempt((n) => n + 1);
          }}
        >
          {t("details.optionsRetry")}
        </Button>
      </div>
    );
  if (!options)
    return <p className="text-body text-muted-foreground">{t("loading")}</p>;

  return (
    <form
      noValidate
      onSubmit={(e) => void submit(e)}
      className="flex flex-col gap-4"
      aria-label={t("characters.formTitle")}
      data-testid="character-form"
    >
      <div className="flex min-w-0 flex-col gap-1">
        <label htmlFor="new-character-name" className={labelClass}>
          {t("characters.name")}
        </label>
        <input
          {...fieldProps("name")}
          autoComplete="off"
          maxLength={options.maxNameLength * 2}
          value={name}
          onChange={(e) => {
            setName(e.target.value);
            setError(null);
          }}
        />
        {errorFor("name")}
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="flex min-w-0 flex-col gap-1">
          <label htmlFor="new-character-class" className={labelClass}>
            {t("details.class")}
          </label>
          <select
            {...fieldProps("class")}
            value={cls}
            onChange={(e) => {
              const next = e.target.value;
              setCls(next);
              const allowed =
                options.classes.find((c) => c.class === next)?.races ?? [];
              if (!allowed.includes(race)) setRace(allowed[0] ?? "");
              // The class limits the alignment (ADR 0032 step 2): a pick it no longer admits is cleared.
              const admitted =
                options.classes.find((c) => c.class === next)?.alignments ?? [];
              setDetails((d) =>
                admitted.includes(d.alignment) ? d : { ...d, alignment: "" },
              );
              setError(null);
            }}
          >
            {options.classes.map((c) => (
              <option key={c.class} value={c.class}>
                {t(`details.classes.${c.class}`, { defaultValue: c.class })}
              </option>
            ))}
          </select>
          {errorFor("class")}
        </div>
        <div className="flex min-w-0 flex-col gap-1">
          <label htmlFor="new-character-raceCode" className={labelClass}>
            {t("characters.race")}
          </label>
          <select
            {...fieldProps("raceCode")}
            value={race}
            onChange={(e) => {
              setRace(e.target.value);
              setError(null);
            }}
          >
            {races.map((r) => (
              <option key={r} value={r}>
                {t(`details.races.${r}`)}
              </option>
            ))}
          </select>
          {errorFor("raceCode")}
        </div>
      </div>

      <fieldset className="flex min-w-0 flex-col gap-2">
        <legend className={cn(labelClass, "mb-1")}>
          {t("characters.abilities")}
        </legend>
        <p className="text-label text-muted-foreground">
          {t("characters.abilitiesLead")}
        </p>
        <div className="grid grid-cols-3 gap-3 sm:grid-cols-6">
          {ABILITIES.map((a) =>
            number(
              a,
              t(`attr.${a}`),
              t("characters.abilityHint", {
                name: t(`abilityCheck.names.${a}`),
                min: options.ability.min,
                max: options.ability.max,
              }),
            ),
          )}
        </div>
        {offersExceptional && (
          <div className="flex min-w-0 flex-col gap-1 sm:w-1/3">
            <label
              htmlFor="new-character-strExceptional"
              className={labelClass}
            >
              {t("characters.strExceptional")}
            </label>
            <input
              {...fieldProps("strExceptional")}
              inputMode="numeric"
              title={t("characters.strExceptionalHint", {
                min: options.strExceptional.min,
                max: options.strExceptional.max,
              })}
              value={strExceptional}
              onChange={(e) => {
                setStrExceptional(e.target.value);
                setError(null);
              }}
            />
            {errorFor("strExceptional")}
          </div>
        )}
      </fieldset>

      {skills.length > 0 && (
        <fieldset
          className="flex min-w-0 flex-col gap-2"
          data-testid="thief-skills-entry"
        >
          <legend className={cn(labelClass, "mb-1")}>
            {t("characters.skillPoints")}
          </legend>
          <p className="text-label text-muted-foreground">
            {t("characters.skillPointsLead")}
          </p>
          <div className="flex min-w-0 flex-col gap-1 sm:w-1/2">
            <label htmlFor="new-character-skillArmor" className={labelClass}>
              {t("characters.skillArmor")}
            </label>
            <select
              {...fieldProps("skillArmor")}
              value={skillArmor}
              onChange={(e) => {
                setSkillArmor(e.target.value);
                setError(null);
              }}
            >
              {armors.map((a) => (
                <option key={a} value={a}>
                  {t(`characters.armors.${a}`, { defaultValue: a })}
                </option>
              ))}
            </select>
            {errorFor("skillArmor")}
          </div>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            {skills.map((skill) => {
              const f: Field = `skillPoints.${skill}`;
              return (
                <div key={skill} className="flex min-w-0 flex-col gap-1">
                  <label
                    htmlFor={`new-character-${f}`}
                    className={cn(labelClass, "break-words")}
                  >
                    {skillName(t, skill)}
                  </label>
                  <input
                    {...fieldProps(f)}
                    inputMode="numeric"
                    title={t("characters.range", {
                      min: options.skillPoints.min,
                      max: options.skillPoints.max,
                    })}
                    value={skillPoints[skill] ?? ""}
                    onChange={(e) => {
                      setSkillPoints((p) => ({
                        ...p,
                        [skill]: e.target.value,
                      }));
                      setError(null);
                    }}
                  />
                  {errorFor(f)}
                </div>
              );
            })}
          </div>
          {errorFor("skillPoints")}
        </fieldset>
      )}

      <div className="grid grid-cols-2 gap-3">
        {number("hpMax", t("characters.hpMax"))}
        {number("xp", t("characters.xp"))}
      </div>
      <p className="text-label text-muted-foreground">
        {t("characters.derivedLead")}
      </p>

      <fieldset
        className="flex min-w-0 flex-col gap-2"
        data-testid="new-character-details"
      >
        <legend className={cn(labelClass, "mb-1")}>
          {t("characters.details")}
        </legend>
        <p className="text-label text-muted-foreground">
          {t("characters.detailsLead")}
        </p>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {pick("alignment", alignments)}
          {pick("sex", options?.sexes ?? [])}
          {DETAIL_FIELDS.map((f) => detail(f))}
        </div>
        {detail("languages", t("details.languagesHint"))}
      </fieldset>

      <div className="flex flex-wrap items-center justify-end gap-2">
        {onCancel && (
          <Button type="button" variant="quiet" onClick={onCancel}>
            {t("characters.cancel")}
          </Button>
        )}
        <Button
          type="submit"
          variant="main"
          disabled={busy}
          aria-label={t("characters.createHint")}
          title={t("characters.createHint")}
        >
          <UserPlus aria-hidden />
          {t("characters.create")}
        </Button>
      </div>
    </form>
  );
}
