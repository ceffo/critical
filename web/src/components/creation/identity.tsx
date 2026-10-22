import { useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import {
  DiceFive,
  PencilSimple,
  Prohibit,
  Warning,
} from "@phosphor-icons/react";
import { rollIdentity } from "@/api/characters";
import { ApiError } from "@/api/http";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import { Button } from "@/components/ui/button";
import { ALIGNMENT_CODES } from "@/lib/characterDetails";
import { hasSpellbook, type StepId, type StepProps } from "./flow";
import { useSpellName } from "@/i18n/spellName";
import { heightField, unitsFor, weightField } from "@/lib/measures";
import { replyGate } from "@/lib/replyGate";
import { skillName } from "@/journal/words";
import { useLanguageNames } from "@/lib/useLanguageNames";
import { cn } from "@/lib/utils";
import {
  ABILITIES,
  type CreationOptions,
  type IdentityFigure,
  type IdentityFigureRange,
  type IdentityRollFigure,
  type SaveNumbers,
} from "@/types/character";
import { Group, Note } from "./kit";
import { useReviewWarnings, useShow, useWarningText } from "./warnings";
import { radioKeys } from "./radioKeys";

type Figure = IdentityFigure;

/** A refusal under a field, worded. */
function FieldError({ id, text }: { id: string; text: string | null }) {
  if (!text) return null;
  return (
    <p
      id={`${id}-error`}
      role="alert"
      className="text-label text-destructive"
      data-testid="field-error"
    >
      {text}
    </p>
  );
}

/** The race's dice and range of a figure, for the sex of a height or weight; undefined when unknown. */
function rangeOf(
  options: CreationOptions,
  race: string,
  sex: string,
  figure: Figure,
): IdentityFigureRange | undefined {
  const row = options.raceIdentity?.find((r) => r.race === race);
  if (!row) return undefined;
  if (figure === "age") return row.age;
  return row[figure].find((f) => f.sex === sex);
}

// --- Step 5: identity ---------------------------------------------------------------------

/**
 * The identity (ADR 0034 step 3.5): name, alignment as a 3x3 grid (the ones the class refuses marked;
 * picking one shows the refusal and blocks Suivant), sex (the languages are the proficiency step's,
 * ADR 0034 step 3.5 as amended by ADR 0037 step 18), height, weight and age each with the server's roll, range and dice, hair and eyes. Every verdict is
 * the server's: the class's list from the options, the warnings from the draft's answer.
 */
export function IdentityStep(props: StepProps) {
  const { options, draft, update, errorFor, answer } = props;
  const { t } = useTranslation();
  // The alignments the server's answer allows; before it, every one is offered and none refused.
  const allowed = answer.latest?.alignments ?? ALIGNMENT_CODES;
  const className = t(`details.classes.${draft.cls}`, {
    defaultValue: draft.cls,
  }).toLocaleLowerCase();
  const nameError = errorFor("name");
  const alignmentError = errorFor("details.alignment");
  const refused =
    draft.details.alignment !== "" &&
    !allowed.includes(draft.details.alignment);
  const alignmentKeys = radioKeys(
    ALIGNMENT_CODES,
    draft.details.alignment,
    (alignment) => update({ details: { alignment } }),
  );
  const sexKeys = radioKeys(options.sexes, draft.details.sex, (sex) =>
    update({ details: { sex } }),
  );

  return (
    <div
      className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)] gap-x-6 gap-y-5"
      data-testid="new-character-details"
    >
      <div className="col-span-2 flex max-w-[32rem] min-w-0 flex-col gap-1">
        <label htmlFor="new-character-name" className={labelClass}>
          {t("characters.name")}
        </label>
        <input
          id="new-character-name"
          aria-invalid={nameError ? true : undefined}
          aria-describedby={nameError ? "new-character-name-error" : undefined}
          className={cn(fieldClass, nameError && "border-destructive")}
          autoComplete="off"
          maxLength={options.maxNameLength * 2}
          value={draft.name}
          onChange={(e) => update({ name: e.target.value })}
        />
        <FieldError id="new-character-name" text={nameError} />
      </div>

      <Group title={t("details.fields.alignment")} testId="alignment-grid">
        <div
          role="radiogroup"
          aria-label={t("details.fields.alignment")}
          className="grid grid-cols-3 gap-1.5"
          onKeyDown={alignmentKeys.onKeyDown}
        >
          {ALIGNMENT_CODES.map((code) => {
            const ok = allowed.includes(code);
            const on = draft.details.alignment === code;
            const name = t(`details.alignments.${code}`);
            return (
              <button
                key={code}
                type="button"
                role="radio"
                aria-checked={on}
                tabIndex={alignmentKeys.tabIndex(code)}
                aria-label={
                  ok
                    ? name
                    : t("creation.identity.alignmentRefusedHint", {
                        name,
                        className,
                      })
                }
                onClick={() => update({ details: { alignment: code } })}
                className={cn(
                  "flex min-h-8 min-w-0 items-center gap-1.5 rounded-md px-2 py-1 text-left text-body outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                  on && !ok
                    ? "bg-raised text-danger shadow-[inset_0_0_0_1.5px_var(--danger)]"
                    : on
                      ? "chip-chosen"
                      : ok
                        ? "chip hover:bg-raised-hover"
                        : "text-ink-muted shadow-[inset_0_0_0_1px_var(--border)] hover:bg-raised-hover",
                )}
              >
                {!ok && (
                  <Prohibit
                    aria-hidden
                    weight="bold"
                    className="size-3.5 shrink-0"
                  />
                )}
                <span className="min-w-0 break-words">{name}</span>
              </button>
            );
          })}
        </div>
        {refused ? (
          <Note kind="refusal" testId="alignment-refusal">
            {t("creation.identity.alignmentRefused", {
              alignment: t(
                `details.alignments.${draft.details.alignment}`,
              ).toLocaleLowerCase(),
              className,
              count: allowed.length,
            })}
          </Note>
        ) : (
          <FieldError
            id="new-character-details.alignment"
            text={alignmentError}
          />
        )}
      </Group>

      <Group title={t("details.fields.sex")}>
        <div
          role="radiogroup"
          aria-label={t("details.fields.sex")}
          className="flex flex-wrap gap-1"
          onKeyDown={sexKeys.onKeyDown}
        >
          {options.sexes.map((code) => (
            <button
              key={code}
              type="button"
              role="radio"
              aria-checked={draft.details.sex === code}
              tabIndex={sexKeys.tabIndex(code)}
              onClick={() => update({ details: { sex: code } })}
              className={cn(
                "flex min-h-7 items-center rounded-md px-3 py-1 text-body outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                draft.details.sex === code
                  ? "chip-chosen"
                  : "chip hover:bg-raised-hover",
              )}
            >
              {t(`details.sexes.${code}`)}
            </button>
          ))}
        </div>
        <p className="text-label text-ink-muted">
          {t("creation.identity.sexLead")}
        </p>
        <FieldError
          id="new-character-details.sex"
          text={errorFor("details.sex")}
        />
      </Group>

      <Group title={t("creation.identity.build")} className="col-span-2">
        <div className="grid grid-cols-[repeat(auto-fill,minmax(15rem,1fr))] gap-4">
          {(["height", "weight", "age"] as const).map((f) => (
            <RollField key={f} figure={f} {...props} />
          ))}
        </div>
      </Group>

      {(["hair", "eyes"] as const).map((f) => {
        const field = `details.${f}`;
        const text = errorFor(field);
        return (
          <div key={f} className="flex min-w-0 flex-col gap-1">
            <label htmlFor={`new-character-${field}`} className={labelClass}>
              {t(`details.fields.${f}`)}
            </label>
            <input
              id={`new-character-${field}`}
              aria-invalid={text ? true : undefined}
              className={cn(fieldClass, text && "border-destructive")}
              autoComplete="off"
              value={draft.details[f]}
              onChange={(e) => update({ details: { [f]: e.target.value } })}
            />
            <FieldError id={`new-character-${field}`} text={text} />
          </div>
        );
      })}
    </div>
  );
}

/**
 * A height, weight or age: the field in the language's units, Tirer (the server rolls the table's
 * dice, ADR 0032 step 5.3), the race's range and dice from the options, the roll's breakdown and the
 * server's range warning. A roll answered after the race or the sex changed, or after a newer roll,
 * is dropped; anything typed meanwhile in the other fields stays (the reply merges into the latest
 * details).
 */
function RollField({
  figure,
  options,
  draft,
  update,
  errorFor,
  answer,
}: StepProps & { figure: Figure }) {
  const { t, i18n } = useTranslation();
  const units = unitsFor(i18n.language);
  const show = useShow();
  const warningText = useWarningText();
  const [roll, setRoll] = useState<{
    r: IdentityRollFigure;
    race: string;
    sex: string;
  } | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [gate] = useState(replyGate);
  const latest = useRef({ race: draft.race, sex: draft.details.sex });
  useEffect(() => {
    latest.current = { race: draft.race, sex: draft.details.sex };
  }, [draft.race, draft.details.sex]);
  useEffect(() => () => gate.close(), [gate]);

  const needsSex = figure !== "age";
  const missing =
    draft.race === ""
      ? "race"
      : needsSex && draft.details.sex === ""
        ? "sex"
        : null;
  const range = rangeOf(options, draft.race, draft.details.sex, figure);
  const field = `details.${figure}`;
  const id = `new-character-${field}`;
  const text = errorFor(field);
  const warning = (answer.current?.warnings ?? []).find(
    (w) => w.code === `${figure}_outside_range`,
  );
  const shownRoll =
    roll &&
    roll.race === draft.race &&
    (!needsSex || roll.sex === draft.details.sex)
      ? roll.r
      : null;

  const doRoll = async () => {
    if (missing || busy) return;
    const race = draft.race;
    const sex = draft.details.sex;
    const current = gate.open();
    setBusy(true);
    setError(null);
    try {
      const r = (await rollIdentity(race, sex, [figure])).rolls[0];
      const now = latest.current;
      if (
        !current() ||
        !r ||
        now.race !== race ||
        (needsSex && now.sex !== sex)
      )
        return;
      setRoll({ r, race, sex });
      if (figure === "height")
        update({
          details: {
            height: heightField(r.value, units, i18n.language),
            rolledHeight: r.value,
          },
        });
      else if (figure === "weight")
        update({
          details: {
            weight: weightField(r.value, units, i18n.language),
            rolledWeight: r.value,
          },
        });
      else update({ details: { age: String(r.value) } });
    } catch (e) {
      if (!current()) return;
      setError(
        e instanceof ApiError && e.field
          ? t(`details.roll.errors.${e.message}`, { defaultValue: e.message })
          : e instanceof Error
            ? e.message
            : String(e),
      );
    } finally {
      if (current()) setBusy(false);
    }
  };

  const unit =
    figure === "age"
      ? t("details.roll.units.year")
      : figure === "weight"
        ? units === "metric"
          ? "kg"
          : "lb"
        : units === "metric"
          ? "cm"
          : "";
  const who =
    draft.race === ""
      ? ""
      : figure === "age"
        ? t(`details.races.${draft.race}`)
        : t("creation.identity.raceSex", {
            race: t(`details.races.${draft.race}`),
            sex: t(`details.sexes.${draft.details.sex}`).toLocaleLowerCase(),
          });

  return (
    <div
      className="flex min-w-0 flex-col gap-1"
      data-testid={`figure-${figure}`}
    >
      <label htmlFor={id} className={labelClass}>
        {t(`details.fields.${figure}`)}
      </label>
      <div className="flex items-center gap-2">
        <input
          id={id}
          aria-invalid={text ? true : undefined}
          className={cn(
            fieldClass,
            "w-24 tabular-nums",
            text && "border-destructive",
          )}
          autoComplete="off"
          inputMode={figure === "age" ? "numeric" : undefined}
          value={draft.details[figure]}
          onChange={(e) => update({ details: { [figure]: e.target.value } })}
        />
        {unit && <span className="text-body text-ink-muted">{unit}</span>}
        <Button
          type="button"
          variant="quiet"
          size="sm"
          disabled={missing !== null || busy}
          onClick={() => void doRoll()}
          title={t(
            missing
              ? `details.roll.need.${missing}`
              : `details.roll.hint.${figure}`,
          )}
          aria-label={t(
            missing
              ? `details.roll.need.${missing}`
              : `details.roll.hint.${figure}`,
          )}
          data-testid={`details-roll-${figure}`}
        >
          <DiceFive aria-hidden />
          {shownRoll ? t("creation.reroll") : t("creation.scores.roll")}
        </Button>
      </div>
      <span
        className="text-label text-ink-muted"
        data-testid={`range-${figure}`}
      >
        {range
          ? t("creation.identity.range", {
              who,
              min: show(figure, range.min, true),
              max: show(figure, range.max),
              count: range.count,
              sides: range.sides,
            })
          : missing
            ? t(`details.roll.need.${missing}`)
            : null}
      </span>
      {shownRoll && (
        <span
          className="text-label"
          data-testid={`details-roll-${figure}-breakdown`}
        >
          {t("details.roll.breakdown", {
            count: shownRoll.dice.length,
            sides: shownRoll.sides,
            dice: shownRoll.dice.join(" + "),
            base: shownRoll.base,
            total: shownRoll.total,
            unit: t(`details.roll.units.${shownRoll.unit}`),
          })}
        </span>
      )}
      {error && (
        <span role="alert" className="text-label text-danger">
          {error}
        </span>
      )}
      <FieldError id={id} text={text} />
      {warning && (
        <span
          className="flex items-start gap-1 text-label text-warning"
          data-testid={`${figure}-warning`}
        >
          <Warning
            aria-hidden
            weight="bold"
            className="mt-0.5 size-3.5 shrink-0"
          />
          {warningText(warning)}
        </span>
      )}
    </div>
  );
}

// --- Step 6: review -----------------------------------------------------------------------

const SAVE_KEYS: [keyof SaveNumbers, string][] = [
  ["paraPoison", "para_poison"],
  ["rodStaffWand", "rod_staff_wand"],
  ["petriPoly", "petri_poly"],
  ["breath", "breath"],
  ["spell", "spell"],
];

/** One block of the review, with its edit button (opens its step). */
function ReviewBlock({
  title,
  step,
  go,
  children,
}: {
  title: string;
  step: StepId;
  go: (s: StepId) => void;
  children: ReactNode;
}) {
  const { t } = useTranslation();
  return (
    <section
      className="flex min-w-0 flex-col gap-2 rounded-lg bg-raised p-3"
      data-testid={`review-${step}`}
    >
      <div className="flex items-center justify-between gap-2">
        <h3 className="section-label">{title}</h3>
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={() => go(step)}
          aria-label={t("creation.review.editHint", { step: title })}
          title={t("creation.review.editHint", { step: title })}
        >
          <PencilSimple aria-hidden />
        </Button>
      </div>
      {children}
    </section>
  );
}

function Dl({ rows }: { rows: [string, ReactNode, string?][] }) {
  return (
    <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1 text-body">
      {rows.map(([k, v, testId]) => (
        <div key={k} className="contents">
          <dt className="text-ink-muted">{k}</dt>
          <dd className="break-words" data-testid={testId}>
            {v}
          </dd>
        </div>
      ))}
    </dl>
  );
}

/**
 * The review (ADR 0034 step 3.6): the warnings on top, then one block per step with an edit button,
 * every value as it will be sent (heights and weights from the body, so what shows is what is
 * stored) and the server's derived figures (THAC0, saves, skills) from the draft's answer; Créer is
 * at the card's foot. The draft's first refusal is shown here; Créer opens the field's step on the
 * creation's.
 */
export function ReviewStep(props: StepProps & { go: (s: StepId) => void }) {
  const { options, draft, answer, refusals, body, go, xp, wording } = props;
  const { t } = useTranslation();
  const name = useLanguageNames(options);
  const show = useShow();
  const warnings = useReviewWarnings(props);
  const spellName = useSpellName();
  const read = answer.current?.derived ? answer.current : null;
  const d = read?.derived;
  const refusal = refusals[0] ?? answer.failed;
  const pending = <span className="text-ink-muted">…</span>;
  const dash = <span className="text-ink-muted">—</span>;
  const className = t(`details.classes.${draft.cls}`, {
    defaultValue: draft.cls,
  });
  const details = body?.details;
  // A new character enters its points in the first skill set its class grants.
  const skills = d?.skillSets[0]?.skills ?? [];
  const warnedField = (f: string) => read?.warnings.some((w) => w.field === f);
  const fromRoll =
    draft.hpRoll && draft.hpMax.trim() === String(draft.hpRoll.total)
      ? draft.hpRoll.levels.map((l) => l.gain).join(" + ")
      : null;
  const measure = (f: "height" | "weight", v: number | null | undefined) =>
    v === null || v === undefined ? (
      dash
    ) : (
      <span className={cn(warnedField(f) && "text-warning")}>{show(f, v)}</span>
    );

  return (
    <div className="flex flex-col gap-4" data-testid="review">
      {refusal && (
        <Note kind="refusal" testId="review-refusal">
          {wording(refusal)}
        </Note>
      )}
      {warnings.map((w) => (
        <Note key={w} kind="warning" testId="review-warning">
          {w}
        </Note>
      ))}
      <div className="grid grid-cols-[repeat(auto-fill,minmax(18rem,1fr))] gap-3">
        <ReviewBlock
          title={t("creation.review.classRace")}
          step="class"
          go={go}
        >
          <Dl
            rows={[
              [t("characters.name"), draft.name.trim() || dash],
              [
                t("creation.class.name"),
                d
                  ? t("creation.review.classLevel", {
                      className,
                      level: d.realizedLevel,
                      xp: Number(xp).toLocaleString(props.language),
                    })
                  : pending,
                "review-class-level",
              ],
              [
                t("creation.race.name"),
                draft.race ? t(`details.races.${draft.race}`) : dash,
              ],
              [t("app.thac0"), d ? d.thac0 : pending, "review-thac0"],
            ]}
          />
          <span className="section-label pt-1">{t("saves.title")}</span>
          <Dl
            rows={SAVE_KEYS.map(([k, key]) => [
              t(`saves.categories.${key}`),
              d ? d.saves[k] : pending,
              `review-save-${key}`,
            ])}
          />
        </ReviewBlock>
        <ReviewBlock title={t("creation.scores.name")} step="scores" go={go}>
          <div className="grid grid-cols-6 gap-1">
            {ABILITIES.map((a) => (
              <div
                key={a}
                className="flex flex-col items-center rounded-md bg-surface py-1"
              >
                <span className="text-label text-ink-muted">
                  {t(`attr.${a}`)}
                </span>
                <span className="text-key tabular-nums">
                  {a === "str" && (body?.strExceptional ?? 0) > 0
                    ? `18/${String(body!.strExceptional % 100).padStart(2, "0")}`
                    : (body?.[a] ?? "—")}
                </span>
              </div>
            ))}
          </div>
          <span className="text-label text-ink-muted">
            {draft.scoreMode === "roll" && draft.roll
              ? t("creation.review.rolledMethod", {
                  method: draft.roll.method.code,
                })
              : t("creation.scores.typed")}
          </span>
        </ReviewBlock>
        <ReviewBlock title={t("creation.extras.name")} step="extras" go={go}>
          <Dl
            rows={[
              [
                t("creation.review.hpMax"),
                fromRoll
                  ? t("creation.review.hpRolled", {
                      hp: body?.hpMax,
                      gains: fromRoll,
                    })
                  : (body?.hpMax ?? dash),
                "review-hp",
              ],
              ...skills.map(
                (s) =>
                  [
                    skillName(t, s.skill),
                    t("creation.review.skillWithPoints", {
                      percent: s.percent,
                      count: body?.skillPoints?.[s.skill] ?? 0,
                    }),
                    `review-skill-${s.skill}`,
                  ] as [string, ReactNode, string],
              ),
            ]}
          />
        </ReviewBlock>
        {hasSpellbook(answer.latest) && (
          <ReviewBlock
            title={t("creation.spellbook.name")}
            step="spellbook"
            go={go}
          >
            <p className="text-body break-words" data-testid="review-spellbook">
              {body?.spellbook && body.spellbook.length > 0
                ? body.spellbook.map((id) => spellName(id)).join(", ")
                : t("creation.spellbook.empty")}
            </p>
          </ReviewBlock>
        )}
        <ReviewBlock
          title={t("creation.identity.name")}
          step="identity"
          go={go}
        >
          <Dl
            rows={[
              [
                t("details.fields.alignment"),
                draft.details.alignment
                  ? t(`details.alignments.${draft.details.alignment}`)
                  : dash,
              ],
              [
                t("details.fields.sex"),
                draft.details.sex
                  ? t(`details.sexes.${draft.details.sex}`)
                  : dash,
              ],
              [
                t("details.fields.languages"),
                read ? read.languages.map(name).join(", ") : pending,
                "review-languages",
              ],
              [
                t("details.fields.height"),
                measure("height", details?.height_cm),
                "review-height",
              ],
              [
                t("details.fields.weight"),
                measure("weight", details?.weight_kg),
                "review-weight",
              ],
              [
                t("details.fields.age"),
                details?.age ? (
                  <span className={cn(warnedField("age") && "text-warning")}>
                    {show("age", details.age)}
                  </span>
                ) : (
                  dash
                ),
                "review-age",
              ],
              [t("details.fields.hair"), draft.details.hair.trim() || dash],
              [t("details.fields.eyes"), draft.details.eyes.trim() || dash],
            ]}
          />
        </ReviewBlock>
      </div>
    </div>
  );
}
