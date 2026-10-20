import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowLeft, ArrowRight, UserPlus, X } from "@phosphor-icons/react";
import { createCharacter, draftCharacter } from "@/api/characters";
import { ApiError, OVER_BUDGET } from "@/api/http";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { holdPath } from "@/lib/navigation";
import { formatHeight, formatWeight, unitsFor } from "@/lib/measures";
import { replyGate } from "@/lib/replyGate";
import { useCreationOptions } from "@/lib/useCreationOptions";
import { useLanguageNames } from "@/lib/useLanguageNames";
import { APP_NAME } from "@/lib/brand";
import { cn } from "@/lib/utils";
import { skillName } from "@/journal/words";
import {
  ABILITIES,
  type CreationDraft,
  type CreationOptions,
} from "@/types/character";
import { CardHead, Note, StepDot, Wordmark } from "./kit";
import {
  EMPTY_DRAFT,
  STEPS,
  blockedReason,
  buildBody,
  classOf,
  draftSpellbook,
  draftXp,
  hasSpellbook,
  refusalError,
  settle,
  stepOfField,
  stepsFor,
  type DraftAnswer,
  type FieldError,
  type DraftPatch,
  mergeDraft,
  type FlowDraft,
  type StepId,
  type StepProps,
} from "./flow";
import { ClassStep, ExtrasStep, RaceStep, ScoresStep } from "./steps";
import { IdentityStep, ReviewStep } from "./identity";
import { ProficienciesStep } from "./proficiencies";
import { SpellbookStep } from "./spellbook";
import { useSpellName } from "@/i18n/spellName";
import { useSpellNamesStore } from "@/stores/spellNames";
import { isSpecialization } from "@/components/proficiencies/weapons";
import { useReviewWarnings, warnedFields } from "./warnings";

/** A server refusal as the flow's field error; anything else (a network error) as a message. An
 * invalid input's code is its message; an over-budget refusal (ADR 0037 step 10) keeps its code,
 * its message worded with the server's figures. */
function fieldErrorOf(err: unknown): FieldError {
  if (err instanceof ApiError && err.field) {
    const code = err.code === OVER_BUDGET ? OVER_BUDGET : err.message;
    return { field: err.field, code, message: err.message };
  }
  return {
    field: "",
    code: "",
    message: err instanceof Error ? err.message : String(err),
  };
}

/** How long the draft rests before it is asked again (typing a name asks once). */
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
  const { options, failed: loadError, retry } = useCreationOptions(true);
  const [draft, setDraft] = useState<FlowDraft>(EMPTY_DRAFT);
  const [step, setStep] = useState<StepId>("class");
  const [reached, setReached] = useState(0);
  const [error, setError] = useState<FieldError | null>(null);
  const [busy, setBusy] = useState(false);
  const [confirmLeave, setConfirmLeave] = useState(false);

  useEffect(() => {
    document.title = `${t("characters.newTitle")} · ${APP_NAME}`;
  }, [t]);
  // The starting spellbook names its spells by id (ADR 0030 step 20.7): the catalogue's names, read
  // once like the dashboard reads them.
  useEffect(() => {
    if (Object.keys(useSpellNamesStore.getState().names).length === 0)
      void useSpellNamesStore.getState().load();
  }, []);

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

  // The server's answer to the draft (ADR 0034, amendment of 2026-10-20: one draft endpoint): the
  // class minimums, the skill budget, the pools, the warnings, the derived figures and the refusals
  // a Créer would meet. Every change of the draft asks again after a short rest; a reply landing
  // after a newer change (or after the player left) is dropped by the gate.
  const built = options ? buildBody(options, draft, xp, i18n.language) : null;
  const body = built && !built.error ? built.body : null;
  // An alignment the class's list leaves out is already shown refused (from the options); the
  // draft is asked without it, so the step still gets the server's other warnings meanwhile.
  const allowedAlignments = classOf(options, draft.cls)?.alignments ?? [];
  const asked = built?.body;
  const draftKey = asked
    ? JSON.stringify(
        asked.details?.alignment !== undefined &&
          !allowedAlignments.includes(asked.details.alignment)
          ? { ...asked, details: { ...asked.details, alignment: undefined } }
          : asked,
      )
    : "";
  const [answered, setAnswered] = useState<{
    key: string;
    cls: string;
    data: CreationDraft | null;
    failed: FieldError | null;
  } | null>(null);
  const [draftGate] = useState(replyGate);
  useEffect(() => {
    if (draftKey === "") return;
    const current = draftGate.open();
    const timer = setTimeout(() => {
      const sent = JSON.parse(draftKey) as { class: string };
      draftCharacter(JSON.parse(draftKey)).then(
        (data) =>
          current() &&
          setAnswered({ key: draftKey, cls: sent.class, data, failed: null }),
        (err: unknown) =>
          current() &&
          setAnswered({
            key: draftKey,
            cls: sent.class,
            data: null,
            failed: fieldErrorOf(err),
          }),
      );
    }, PREVIEW_DELAY_MS);
    return () => {
      clearTimeout(timer);
      draftGate.close();
    };
  }, [draftKey, draftGate]);
  // A reply belongs to the draft it answered: once the draft moves on, its verdicts (the warnings,
  // the refusals, "no warning") are not shown, whatever the reply's arrival; its figures stay shown
  // for the same class until the next answer, so they do not blink as the player types. An answer
  // about another class is never shown.
  const fresh = answered !== null && answered.key === draftKey;
  const answer: DraftAnswer = {
    current: fresh ? answered.data : null,
    latest: answered?.cls === draft.cls ? answered.data : null,
    failed: fresh ? answered.failed : null,
  };
  // The refusals the creation would meet are shown as the former preview showed them (ADR 0034 step
  // 6 unchanged): once the draft was seen on the proficiency or identity step or the review, at its
  // field on any step, until the draft changes elsewhere; a value out of range typed on an earlier
  // step is the server's refusal at Créer.
  const previewing =
    step === "proficiencies" ||
    step === "spellbook" ||
    step === "identity" ||
    step === "review";
  const [previewedKey, setPreviewedKey] = useState("");
  if (previewing && previewedKey !== draftKey) setPreviewedKey(draftKey);
  const seen = previewedKey === draftKey;
  const refusals = seen
    ? (answer.current?.refusals ?? []).map(refusalError)
    : [];

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
    if (e.field === "weapon" || e.field.startsWith("weapon."))
      return t(`weaponProficiencies.errors.${e.code}`, {
        defaultValue: t("weaponProficiencies.errors.refused"),
      });
    // A starting spell's refusal (ADR 0030 step 20.7), worded with the draft's limits.
    if (e.field === "spellbook" || e.field.startsWith("spellbook.")) {
      const book = answer.latest?.spellbook;
      return t(`creation.spellbook.errors.${e.code}`, {
        className,
        classMax: book?.classMaxSpellLevel ?? 0,
        intMax: book?.intMaxSpellLevel ?? 0,
        max: book?.maxSpellsPerLevel ?? 0,
        defaultValue: t(`apiError.${e.code}`, {
          defaultValue: e.message ?? e.code,
        }),
      });
    }
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
  // Créer's refusal first, else the draft's (shown at its field).
  const errorFor = (field: string) => {
    if (error) return error.field === field ? wording(error) : null;
    // A value the web cannot read (a weight typed as "heavy") shows at its field at once.
    if (built?.error?.field === field) return wording(built.error);
    const refused = refusals.find((r) => r.field === field);
    return refused ? wording(refused) : null;
  };

  // The class's steps (the spellbook step for a mage or a bard only); `reached` counts in STEPS,
  // so a class picked again keeps what was reached.
  const steps = stepsFor(options, draft.cls);
  const index = steps.indexOf(step);
  const go = (to: StepId) => {
    setStep(to);
    setReached((r) => Math.max(r, STEPS.indexOf(to)));
  };

  const submit = async () => {
    if (!options || !built) return;
    if (built.error) {
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
            <Button variant="quiet" onClick={retry}>
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
    answer,
    refusals,
    body,
    wording,
    language: i18n.language,
  };
  // A value the web cannot read blocks the step that holds its field, where it is shown; the
  // review and Créer send the player back to that step.
  const unreadable =
    built?.error != null && stepOfField(built.error.field) === step;
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
  const last = index === steps.length - 1;
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
          options={options}
          steps={steps}
          current={step}
          reached={reached}
          draft={draft}
          onGo={go}
        />
        <main
          className="stage-card flex min-w-0 flex-1 flex-col"
          data-testid="creation-step"
          data-step={step}
        >
          <div className="card-band flex shrink-0 flex-col gap-0.5 px-5 py-3">
            <span className="text-label text-ink-muted">
              {t("creation.stepOf", { n: index + 1, of: steps.length })}
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
            {step === "spellbook" && <SpellbookStep {...props} />}
            {step === "identity" && <IdentityStep {...props} />}
            {step === "review" && <ReviewStep {...props} go={go} />}
          </div>
          <div className="flex shrink-0 items-center gap-3 border-t border-border px-5 py-3">
            {index > 0 && (
              <Button
                variant="quiet"
                size="lg"
                onClick={() => go(steps[index - 1]!)}
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
                onClick={() => go(steps[index + 1]!)}
                aria-label={t("creation.nextHint")}
                title={t("creation.nextHint")}
              >
                {t("creation.next")}
                <ArrowRight aria-hidden />
              </Button>
            )}
          </div>
        </main>
        {/* The panel reads the server's languages and warnings once the draft was seen where the
            former preview was asked, as it did. */}
        <Building
          {...props}
          answer={{ ...answer, current: seen ? answer.current : null }}
        />
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
function useStepChoice(
  draft: FlowDraft,
  options: CreationOptions,
): (step: StepId) => string | null {
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
      case "proficiencies": {
        const parts = [
          draft.weaponPicks.length > 0 &&
            t("creation.proficiencies.weapons", {
              count: draft.weaponPicks.length,
            }),
          draft.picks.length > 0 &&
            t("creation.proficiencies.pickedN", { count: draft.picks.length }),
        ].filter(Boolean);
        return parts.length > 0 ? parts.join(" · ") : null;
      }
      case "spellbook": {
        const n = draftSpellbook(options, draft).length;
        return n > 0 ? t("creation.spellbook.spellsN", { count: n }) : null;
      }
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

/** The steps as a vertical list; a done one shows its choice and opens again on a click. `reached`
 * is the furthest step reached, counted in STEPS. */
function StepList({
  options,
  steps,
  current,
  reached,
  draft,
  onGo,
}: {
  options: CreationOptions;
  steps: StepId[];
  current: StepId;
  reached: number;
  draft: FlowDraft;
  onGo: (s: StepId) => void;
}) {
  const { t } = useTranslation();
  const choice = useStepChoice(draft, options);
  return (
    <nav
      className="zone-card flex w-[clamp(14rem,15vw,16rem)] shrink-0 flex-col"
      aria-label={t("creation.steps")}
      data-testid="creation-steps"
    >
      <CardHead>{t("creation.steps")}</CardHead>
      <ol className="flex min-h-0 flex-col gap-1 overflow-y-auto p-2">
        {steps.map((s, i) => {
          const at = STEPS.indexOf(s);
          const done = at < Math.max(reached, STEPS.indexOf(current));
          const on = s === current;
          const open = at <= reached && !on;
          const shown = done && !on ? choice(s) : null;
          return (
            <li key={s}>
              <button
                type="button"
                disabled={!open}
                aria-current={on ? "step" : undefined}
                onClick={() => onGo(s)}
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

/** The review's foot: Créer's network error, else the draft's verdict ("1 avertissement, rien de
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
  if (props.refusals.length > 0 || props.answer.failed)
    return <>{t("creation.review.refusedFoot")}</>;
  if (!props.answer.current?.derived) return null;
  return warnings.length > 0 ? (
    <>{t("creation.review.warningsFoot", { count: warnings.length })}</>
  ) : (
    <>{t("creation.review.noWarning")}</>
  );
}

/** The character being built, filled step by step (the dashboard character card's width). */
function Building({ options, draft, xp, answer, body, language }: StepProps) {
  const { t } = useTranslation();
  const spellName = useSpellName();
  const spells = draftSpellbook(options, draft);
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
  const read = answer.current?.derived ? answer.current : null;
  const languages = read?.languages ?? draft.details.languages;
  const details = body?.details;
  const build = [
    details?.height_cm
      ? formatHeight(details.height_cm, units, language)
      : null,
    details?.weight_kg
      ? formatWeight(details.weight_kg, units, language)
      : null,
  ].filter(Boolean);
  const warned = read?.derived
    ? warnedFields(t, read.warnings, read.derived.skillBudget.length)
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
          <dt className="text-ink-muted">
            {t("creation.proficiencies.weaponsName")}
          </dt>
          <dd className="break-words" data-testid="building-weapons">
            {draft.weaponPicks.length > 0
              ? draft.weaponPicks
                  .map((p) =>
                    isSpecialization(p)
                      ? `${t(`weapons.types.${p.type}`, { defaultValue: p.type })} (${t("weaponProficiencies.specialized")})`
                      : t(`weapons.types.${p.type}`, { defaultValue: p.type }),
                  )
                  .join(", ")
              : dash}
          </dd>
          <dt className="text-ink-muted">{t("creation.proficiencies.name")}</dt>
          <dd className="break-words" data-testid="building-proficiencies">
            {draft.picks.length > 0
              ? draft.picks.map((p) => skillName(t, p.id)).join(", ")
              : dash}
          </dd>
          {hasSpellbook(options, draft.cls) && (
            <>
              <dt className="text-ink-muted">{t("creation.spellbook.name")}</dt>
              <dd className="break-words" data-testid="building-spellbook">
                {spells.length > 0
                  ? spells.map((id) => spellName(id)).join(", ")
                  : dash}
              </dd>
            </>
          )}
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
