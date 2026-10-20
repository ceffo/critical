import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowLeft, ArrowRight, UserPlus, X } from "@phosphor-icons/react";
import {
  createCharacter,
  fetchCreationChoices,
  checkDraft,
  previewCharacter,
} from "@/api/characters";
import { ApiError, OVER_BUDGET } from "@/api/http";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import {
  STEPS,
  blockedReason,
  classOf,
  stepOfField,
  type StepId,
} from "@/lib/creationFlow";
import { holdPath } from "@/lib/navigation";
import { formatHeight, formatWeight, unitsFor } from "@/lib/measures";
import { replyGate } from "@/lib/replyGate";
import { useLanguageNames } from "@/lib/useLanguageNames";
import { APP_NAME } from "@/lib/brand";
import { cn } from "@/lib/utils";
import { skillName } from "@/journal/words";
import {
  ABILITIES,
  type CreationOptions,
  type DraftChecks,
  type DraftChecksRequest,
} from "@/types/character";
import { CardHead, Note, StepDot, Wordmark } from "./kit";
import {
  EMPTY_DRAFT,
  buildBody,
  checksRequest,
  draftXp,
  settle,
  type FieldError,
  type DraftPatch,
  mergeDraft,
  type FlowDraft,
  type PreviewState,
  type StepProps,
} from "./flow";
import { ClassStep, ExtrasStep, RaceStep, ScoresStep } from "./steps";
import { IdentityStep, ReviewStep } from "./identity";
import { ProficienciesStep } from "./proficiencies";
import { useReviewWarnings, warnedFields } from "./warnings";

/** The server's field names for the flow's: the race is `race` in its errors. */
const ERROR_FIELD: Record<string, string> = { race: "raceCode" };

/** A server refusal as the flow's field error; anything else (a network error) as a message. An
 * invalid input's code is its message; an over-budget refusal (ADR 0037 step 10) keeps its code,
 * its message worded with the server's figures. */
function fieldErrorOf(err: unknown): FieldError {
  if (err instanceof ApiError && err.field) {
    const field = ERROR_FIELD[err.field] ?? err.field;
    const code = err.code === OVER_BUDGET ? OVER_BUDGET : err.message;
    return { field, code, message: err.message };
  }
  return {
    field: "",
    code: "",
    message: err instanceof Error ? err.message : String(err),
  };
}

/** How long the draft rests before the preview is asked again (typing a name asks once). */
const PREVIEW_DELAY_MS = 250;

/** True once the player entered anything: leaving then asks first. */
function isDirty(d: FlowDraft): boolean {
  return JSON.stringify(d) !== JSON.stringify(EMPTY_DRAFT);
}

/**
 * The creation route (ADR 0034 steps 2 to 4): a header band (brand, title, Quitter), the step list,
 * the step's card (its content scrolls inside it, Retour / Suivant at its foot with the reason Suivant
 * is blocked) and the character filling in. The values live here, so going back and forth keeps
 * them; leaving with anything entered asks first. Créer sends what was entered; the server checks
 * it all and a refusal opens its step with the field's error. Rules are the server's: the web
 * computes no score, HP, level or verdict.
 */
export function CreationFlow({ onLeave }: { onLeave: () => void }) {
  const { t, i18n } = useTranslation();
  const [options, setOptions] = useState<CreationOptions | null>(null);
  const [loadError, setLoadError] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [draft, setDraft] = useState<FlowDraft>(EMPTY_DRAFT);
  const [step, setStep] = useState<StepId>("class");
  const [reached, setReached] = useState(0);
  const [error, setError] = useState<FieldError | null>(null);
  const [busy, setBusy] = useState(false);
  const [confirmLeave, setConfirmLeave] = useState(false);

  useEffect(() => {
    document.title = `${t("characters.newTitle")} · ${APP_NAME}`;
  }, [t]);

  useEffect(() => {
    let live = true;
    fetchCreationChoices().then(
      (o) => live && setOptions(o),
      () => live && setLoadError(true),
    );
    return () => {
      live = false;
    };
  }, [attempt]);

  const dirty = isDirty(draft);
  // Closing the tab or reloading with values entered asks the browser's question; leaving through
  // Créer or the confirmed Quitter does not.
  const going = useRef(false);
  useEffect(() => {
    if (!dirty) return;
    const warn = (e: BeforeUnloadEvent) => {
      if (!going.current) e.preventDefault();
    };
    addEventListener("beforeunload", warn);
    return () => removeEventListener("beforeunload", warn);
  }, [dirty]);
  // The browser's Back or Forward stays in the document (no beforeunload): with values entered it
  // asks the same question as Quitter instead of dropping them.
  useEffect(() => {
    if (!dirty) return;
    return holdPath(() => {
      if (!going.current) setConfirmLeave(true);
    });
  }, [dirty]);

  // Every change makes a new revision; a request's reply is applied only if none came since.
  const revision = useRef(0);
  const watch = () => {
    const at = revision.current;
    return () => revision.current === at;
  };
  const update = (patch: DraftPatch) => {
    revision.current += 1;
    setDraft((d) => {
      const next = mergeDraft(d, patch);
      return settle(next, draftXp(options, next));
    });
    setError(null);
  };

  const xp = draftXp(options, draft);

  // The server's preview of the draft on the identity step and the review (ADR 0034): the warnings
  // and the derived figures. Every change of the draft asks again after a short rest; a reply to an
  // older draft (or one landing after the player left these steps) is dropped by the gate.
  const [answered, setPreview] = useState<{
    key: string;
    state: PreviewState;
  } | null>(null);
  const [previewGate] = useState(replyGate);
  const built = options ? buildBody(options, draft, xp, i18n.language) : null;
  const body = built && "body" in built ? built.body : null;
  // An alignment the class's list leaves out is already shown refused (from the options); the
  // preview is asked without it, so the step still gets the server's other warnings meanwhile.
  const allowedAlignments = classOf(options, draft.cls)?.alignments ?? [];
  const bodyKey = body
    ? JSON.stringify(
        body.details?.alignment !== undefined &&
          !allowedAlignments.includes(body.details.alignment)
          ? { ...body, details: { ...body.details, alignment: undefined } }
          : body,
      )
    : "";
  // A reply belongs to the draft it answered: once the draft moves on (or becomes unreadable) the
  // old figures and the "no warning" verdict are not shown, whatever the reply's arrival.
  const preview: PreviewState =
    answered && answered.key === bodyKey && bodyKey !== ""
      ? answered.state
      : { data: null, error: null };
  const previewing = step === "identity" || step === "review";
  useEffect(() => {
    if (!previewing || bodyKey === "") return;
    const current = previewGate.open();
    const timer = setTimeout(() => {
      previewCharacter(JSON.parse(bodyKey)).then(
        (data) =>
          current() &&
          setPreview({ key: bodyKey, state: { data, error: null } }),
        (err: unknown) =>
          current() &&
          setPreview({
            key: bodyKey,
            state: { data: null, error: fieldErrorOf(err) },
          }),
      );
    }, PREVIEW_DELAY_MS);
    return () => {
      clearTimeout(timer);
      previewGate.close();
    };
  }, [previewing, bodyKey, previewGate]);

  // The server's verdicts on the class and skills steps (ADR 0034): the classes whose ability
  // minimums the scores miss, and the skill point budget with what is left. The last answer stays
  // shown while a newer one is asked for, so the figures do not blink as the player types.
  const [answeredChecks, setChecks] = useState<{
    cls: string;
    data: DraftChecks;
  } | null>(null);
  // An answer about another class (the player picked again meanwhile) is never shown.
  const checks = answeredChecks?.cls === draft.cls ? answeredChecks.data : null;
  const [checksGate] = useState(replyGate);
  const checking =
    step === "class" || step === "extras" || step === "proficiencies";
  const checksKey = options
    ? JSON.stringify(checksRequest(options, draft, xp))
    : "";
  useEffect(() => {
    if (!checking || checksKey === "") return;
    const current = checksGate.open();
    const timer = setTimeout(() => {
      const asked = JSON.parse(checksKey) as DraftChecksRequest;
      checkDraft(asked).then(
        (data) => current() && setChecks({ cls: asked.class, data }),
        // An older server has no checks: the steps show what the options carry.
        () => current() && setChecks(null),
      );
    }, PREVIEW_DELAY_MS);
    return () => {
      clearTimeout(timer);
      checksGate.close();
    };
  }, [checking, checksKey, checksGate]);

  const className = t(`details.classes.${draft.cls}`, {
    defaultValue: draft.cls,
  });

  const wording = (e: FieldError): string => {
    if (e.field === "" || e.code === OVER_BUDGET) return e.message ?? "";
    // A pick's refusal (ADR 0037 step 12): the proficiency editor's wording of its code.
    if (e.field.startsWith("nonweapon"))
      return t(`proficiencies.errors.${e.code}`, {
        defaultValue: t("proficiencies.errors.refused"),
      });
    if (e.field.startsWith("details.")) {
      const detail = e.field.slice("details.".length);
      return t(
        [`details.errors.${detail}.${e.code}`, `details.errors.${e.code}`],
        { className, defaultValue: e.message ?? e.code },
      );
    }
    const range =
      e.field === "name"
        ? { max: options?.maxNameLength }
        : e.field === "strExceptional"
          ? options?.strExceptional
          : e.field.startsWith("skillPoints.")
            ? options?.skillPoints
            : e.field === "hpMax"
              ? options?.hpMax
              : e.field === "xp"
                ? options?.xp
                : (ABILITIES as readonly string[]).includes(e.field)
                  ? options?.ability
                  : undefined;
    return t(`characters.errors.${e.code}`, {
      ...range,
      className,
      defaultValue: e.message ?? e.code,
    });
  };
  // Créer's refusal first, else the preview's (the identity step shows it at its field).
  const errorFor = (field: string) =>
    error?.field === field
      ? wording(error)
      : // A value the web cannot read (a weight typed as "heavy") shows at its field at once.
        !error && built && "error" in built && built.error.field === field
        ? wording(built.error)
        : !error && preview.error?.field === field
          ? wording(preview.error)
          : null;

  const index = STEPS.indexOf(step);
  const go = (to: StepId) => {
    setStep(to);
    setReached((r) => Math.max(r, STEPS.indexOf(to)));
  };

  const submit = async () => {
    if (!options || !built) return;
    if ("error" in built) {
      setError(built.error);
      go(stepOfField(built.error.field));
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await createCharacter(built.body);
      going.current = true;
      globalThis.location.assign("/");
    } catch (err) {
      setBusy(false);
      const e = fieldErrorOf(err);
      setError(e);
      // A refusal opens the step that holds its field (ADR 0034); a network error stays here.
      if (e.field !== "") go(stepOfField(e.field));
    }
  };

  const leaveNow = () => {
    going.current = true;
    onLeave();
  };
  const leave = () => (dirty ? setConfirmLeave(true) : leaveNow());

  if (loadError || !options)
    return (
      <main className="flex h-app items-center justify-center px-3 pt-3">
        {loadError ? (
          <div role="alert" className="flex flex-col items-start gap-2">
            <p className="text-body text-danger">{t("details.optionsError")}</p>
            <Button
              variant="quiet"
              onClick={() => {
                setLoadError(false);
                setAttempt((n) => n + 1);
              }}
            >
              {t("details.optionsRetry")}
            </Button>
          </div>
        ) : (
          <p className="text-body text-muted-foreground">{t("loading")}</p>
        )}
      </main>
    );

  const props: StepProps = {
    options,
    draft,
    update,
    error,
    errorFor,
    xp,
    watch,
    preview,
    checks,
    body,
    wording,
    language: i18n.language,
  };
  // A value the web cannot read blocks the step that holds its field, where it is shown; the
  // review and Créer send the player back to that step.
  const unreadable =
    built && "error" in built && stepOfField(built.error.field) === step;
  const blocked =
    blockedReason(step, {
      cls: draft.cls,
      race: draft.race,
      xp,
      scores: draft.scores,
      hpMax: draft.hpMax,
      name: draft.name,
      alignment: draft.details.alignment,
      sex: draft.details.sex,
      alignments: classOf(options, draft.cls)?.alignments ?? [],
    }) ?? (unreadable ? "unreadable" : null);
  const last = index === STEPS.length - 1;
  const lead =
    step === "race" && draft.cls
      ? t("creation.race.lead", { className: className.toLocaleLowerCase() })
      : step === "extras" && draft.cls
        ? t("creation.extras.lead", {
            className: className.toLocaleLowerCase(),
          })
        : t(`creation.${step}.lead`);

  return (
    <div
      className="mx-auto flex h-app max-w-[1680px] flex-col gap-3 overflow-hidden px-3 pt-3"
      data-screen="create-character"
    >
      <header className="zone-card flex shrink-0 items-center gap-4 px-4 py-2">
        <Wordmark />
        <span className="text-title">{t("characters.newTitle")}</span>
        <span className="flex-1" />
        <Button
          variant="quiet"
          onClick={leave}
          aria-label={t("creation.leaveHint")}
          title={t("creation.leaveHint")}
        >
          <X aria-hidden />
          {t("creation.leave")}
        </Button>
      </header>
      <div className="flex min-h-0 flex-1 gap-3">
        <StepList
          current={index}
          reached={reached}
          draft={draft}
          onGo={(i) => go(STEPS[i]!)}
        />
        <main
          className="stage-card flex min-w-0 flex-1 flex-col"
          data-testid="creation-step"
          data-step={step}
        >
          <div className="card-band flex shrink-0 flex-col gap-0.5 px-5 py-3">
            <span className="text-label text-ink-muted">
              {t("creation.stepOf", { n: index + 1, of: STEPS.length })}
            </span>
            <h1 className="text-headline">{t(`creation.${step}.title`)}</h1>
            <p className="text-body text-ink-muted">{lead}</p>
          </div>
          {/* The scroller is its content's containing block (relative): an absolute box inside, such
              as a score's sr-only label, scrolls and clips with it instead of stretching the page. */}
          <div className="relative flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto px-5 py-4">
            {step === "class" && <ClassStep {...props} />}
            {step === "race" && <RaceStep {...props} />}
            {step === "scores" && <ScoresStep {...props} />}
            {step === "extras" && <ExtrasStep {...props} />}
            {step === "proficiencies" && <ProficienciesStep {...props} />}
            {step === "identity" && <IdentityStep {...props} />}
            {step === "review" && <ReviewStep {...props} go={go} />}
          </div>
          <div className="flex shrink-0 items-center gap-3 border-t border-border px-5 py-3">
            {index > 0 && (
              <Button
                variant="quiet"
                size="lg"
                onClick={() => go(STEPS[index - 1]!)}
                aria-label={t("creation.backHint")}
                title={t("creation.backHint")}
              >
                <ArrowLeft aria-hidden />
                {t("creation.back")}
              </Button>
            )}
            <span
              className="min-w-0 flex-1 text-right text-body text-ink-muted"
              data-testid="next-blocked"
            >
              {blocked ? (
                t(`creation.blocked.${blocked}`)
              ) : last ? (
                <ReviewFoot props={props} error={error} wording={wording} />
              ) : null}
            </span>
            {last ? (
              <Button
                variant="main"
                size="lg"
                disabled={busy}
                onClick={() => void submit()}
                aria-label={t("characters.createHint")}
                title={t("characters.createHint")}
              >
                <UserPlus aria-hidden />
                {t("characters.create")}
              </Button>
            ) : (
              <Button
                variant="main"
                size="lg"
                disabled={blocked !== null}
                onClick={() => go(STEPS[index + 1]!)}
                aria-label={t("creation.nextHint")}
                title={t("creation.nextHint")}
              >
                {t("creation.next")}
                <ArrowRight aria-hidden />
              </Button>
            )}
          </div>
        </main>
        <Building {...props} />
      </div>
      <Dialog open={confirmLeave} onOpenChange={setConfirmLeave}>
        <DialogContent
          className="max-w-[28rem] gap-4"
          data-testid="leave-confirm"
        >
          <DialogTitle className="section-label">
            {t("creation.leaveTitle")}
          </DialogTitle>
          <p className="text-body">{t("creation.leaveLead")}</p>
          <div className="flex flex-wrap justify-end gap-2">
            <Button
              variant="quiet"
              onClick={() => setConfirmLeave(false)}
              aria-label={t("creation.stayHint")}
              title={t("creation.stayHint")}
            >
              {t("creation.stay")}
            </Button>
            <Button
              variant="destructive"
              onClick={() => {
                setConfirmLeave(false);
                leaveNow();
              }}
              aria-label={t("creation.discardHint")}
              title={t("creation.discardHint")}
            >
              <X aria-hidden />
              {t("creation.discard")}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

/** What a done step shows under its name: the choice made. */
function useStepChoice(draft: FlowDraft): (step: StepId) => string | null {
  const { t } = useTranslation();
  return (step) => {
    switch (step) {
      case "class":
        return draft.cls
          ? `${t(`details.classes.${draft.cls}`, { defaultValue: draft.cls })} · ${levelText(t, draft)}`
          : null;
      case "race":
        return draft.race ? t(`details.races.${draft.race}`) : null;
      case "scores":
        return draft.scoreMode === "roll" && draft.roll
          ? t("creation.scores.methodN", { method: draft.roll.method.code })
          : t("creation.scores.typed");
      case "extras":
        return draft.hpMax
          ? t("creation.extras.hpN", { hp: draft.hpMax })
          : null;
      case "proficiencies":
        return draft.picks.length > 0
          ? t("creation.proficiencies.pickedN", { count: draft.picks.length })
          : null;
      case "identity":
        return draft.name.trim() || null;
      default:
        return null;
    }
  };
}

/** "niveau 5" for a picked level; the typed XP otherwise (the server derives the level from it). */
function levelText(
  t: (k: string, o?: Record<string, unknown>) => string,
  draft: FlowDraft,
): string {
  return draft.levelMode === "level"
    ? t("creation.level.levelLower", { level: draft.level })
    : t("creation.level.xpN", { xp: draft.xp });
}

/** The steps as a vertical list; a done one shows its choice and opens again on a click. */
function StepList({
  current,
  reached,
  draft,
  onGo,
}: {
  current: number;
  reached: number;
  draft: FlowDraft;
  onGo: (i: number) => void;
}) {
  const { t } = useTranslation();
  const choice = useStepChoice(draft);
  return (
    <nav
      className="zone-card flex w-[clamp(14rem,15vw,16rem)] shrink-0 flex-col"
      aria-label={t("creation.steps")}
      data-testid="creation-steps"
    >
      <CardHead>{t("creation.steps")}</CardHead>
      <ol className="flex min-h-0 flex-col gap-1 overflow-y-auto p-2">
        {STEPS.map((s, i) => {
          const done = i < Math.max(reached, current);
          const on = i === current;
          const open = i <= reached && !on;
          const shown = done && !on ? choice(s) : null;
          return (
            <li key={s}>
              <button
                type="button"
                disabled={!open}
                aria-current={on ? "step" : undefined}
                onClick={() => onGo(i)}
                className={cn(
                  "flex w-full items-start gap-3 rounded-md px-2 py-2 text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                  on && "selected",
                  open && "hover:bg-raised-hover",
                )}
              >
                <StepDot n={i + 1} done={done} on={on} />
                <span className="flex min-w-0 flex-col">
                  <span
                    className={cn(
                      "text-body break-words",
                      on ? "font-semibold" : done ? "" : "text-ink-muted",
                    )}
                  >
                    {t(`creation.${s}.name`)}
                  </span>
                  {shown && (
                    <span className="text-label break-words text-ink-muted">
                      {shown}
                    </span>
                  )}
                </span>
              </button>
            </li>
          );
        })}
      </ol>
    </nav>
  );
}

/** The review's foot: Créer's network error, else the preview's verdict ("1 avertissement, rien de
 * bloquant"). */
function ReviewFoot({
  props,
  error,
  wording,
}: {
  props: StepProps;
  error: FieldError | null;
  wording: (e: FieldError) => string;
}) {
  const { t } = useTranslation();
  const warnings = useReviewWarnings(props);
  if (error) return <span className="text-danger">{wording(error)}</span>;
  if (props.preview.error) return <>{t("creation.review.refusedFoot")}</>;
  if (!props.preview.data) return null;
  return warnings.length > 0 ? (
    <>{t("creation.review.warningsFoot", { count: warnings.length })}</>
  ) : (
    <>{t("creation.review.noWarning")}</>
  );
}

/** The character being built, filled step by step (the dashboard character card's width). */
function Building({ options, draft, xp, preview, body, language }: StepProps) {
  const { t } = useTranslation();
  const languageName = useLanguageNames(options);
  const units = unitsFor(language);
  const dash = <span className="text-ink-muted">—</span>;
  const line = [
    draft.cls
      ? t(`details.classes.${draft.cls}`, { defaultValue: draft.cls })
      : null,
    draft.race ? t(`details.races.${draft.race}`) : null,
    draft.cls ? levelText(t, { ...draft, xp }) : null,
  ].filter(Boolean);
  // The languages the server reads (the native one added) once it answered, else the ones picked.
  const languages = preview.data?.languages ?? draft.details.languages;
  const details = body?.details;
  const build = [
    details?.height_cm
      ? formatHeight(details.height_cm, units, language)
      : null,
    details?.weight_kg
      ? formatWeight(details.weight_kg, units, language)
      : null,
  ].filter(Boolean);
  const warned = preview.data
    ? warnedFields(
        t,
        preview.data.warnings,
        preview.data.derived.skillBudget.length,
      )
    : [];
  return (
    <aside
      className="zone-card flex w-[clamp(280px,21vw,384px)] shrink-0 flex-col overflow-y-auto"
      aria-label={t("creation.yourCharacter")}
      data-testid="creation-building"
    >
      <CardHead>{t("creation.yourCharacter")}</CardHead>
      <div className="flex flex-col gap-4 px-4 pt-3 pb-4">
        <div className="flex flex-col gap-0.5">
          <span className="text-title break-words">
            {draft.name.trim() || (
              <span className="text-ink-muted">{t("creation.noName")}</span>
            )}
          </span>
          <span className="text-body break-words text-ink-muted">
            {line.length > 0 ? line.join(" · ") : "—"}
          </span>
        </div>
        <div className="grid grid-cols-6 gap-1">
          {ABILITIES.map((a) => (
            <div
              key={a}
              className="flex flex-col items-center rounded-md bg-raised py-1"
            >
              <span className="text-label text-ink-muted">
                {t(`attr.${a}`)}
              </span>
              <span className="text-key tabular-nums">
                {draft.scores[a].trim() || dash}
              </span>
            </div>
          ))}
        </div>
        <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1.5 text-body">
          <dt className="text-ink-muted">{t("creation.extras.hp")}</dt>
          <dd>{draft.hpMax.trim() || dash}</dd>
          <dt className="text-ink-muted">{t("details.fields.alignment")}</dt>
          <dd className="break-words">
            {draft.details.alignment
              ? t(`details.alignments.${draft.details.alignment}`)
              : dash}
          </dd>
          <dt className="text-ink-muted">{t("details.fields.sex")}</dt>
          <dd>
            {draft.details.sex ? t(`details.sexes.${draft.details.sex}`) : dash}
          </dd>
          <dt className="text-ink-muted">{t("details.fields.languages")}</dt>
          <dd className="break-words">
            {languages.length > 0
              ? languages.map(languageName).join(", ")
              : dash}
          </dd>
          <dt className="text-ink-muted">{t("creation.proficiencies.name")}</dt>
          <dd className="break-words" data-testid="building-proficiencies">
            {draft.picks.length > 0
              ? draft.picks.map((p) => skillName(t, p.id)).join(", ")
              : dash}
          </dd>
          <dt className="text-ink-muted">{t("creation.heightWeight")}</dt>
          <dd className="break-words">
            {build.length > 0 ? build.join(" · ") : dash}
          </dd>
          <dt className="text-ink-muted">{t("details.fields.age")}</dt>
          <dd>
            {details?.age
              ? `${details.age} ${t("details.roll.units.year")}`
              : dash}
          </dd>
        </dl>
        {warned.length > 0 && (
          <Note kind="warning" testId="building-warnings">
            {t("creation.warningsOn", {
              count: warned.length,
              fields: warned.join(", "),
            })}
          </Note>
        )}
      </div>
    </aside>
  );
}
