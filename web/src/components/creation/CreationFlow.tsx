import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowLeft, ArrowRight, UserPlus, X } from "@phosphor-icons/react";
import { createCharacter, fetchCreationChoices } from "@/api/characters";
import { ApiError } from "@/api/http";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { toBody } from "@/lib/characterDetails";
import {
  STEPS,
  blockedReason,
  classOf,
  stepOfField,
  type StepId,
} from "@/lib/creationFlow";
import { holdPath } from "@/lib/navigation";
import { useLanguageNames } from "@/lib/useLanguageNames";
import { parseWhole } from "@/lib/wholeNumber";
import { APP_NAME } from "@/lib/brand";
import { cn } from "@/lib/utils";
import {
  ABILITIES,
  type CreationOptions,
  type NewCharacter,
} from "@/types/character";
import { CardHead, StepDot, Wordmark } from "./kit";
import {
  EMPTY_DRAFT,
  draftXp,
  settle,
  type FieldError,
  type DraftPatch,
  mergeDraft,
  type FlowDraft,
  type StepProps,
} from "./flow";
import {
  ClassStep,
  ExtrasStep,
  IdentityStep,
  RaceStep,
  ScoresStep,
} from "./steps";

/** The server's field names for the flow's: the race is `race` in its errors. */
const ERROR_FIELD: Record<string, string> = { race: "raceCode" };

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

  const className = t(`details.classes.${draft.cls}`, {
    defaultValue: draft.cls,
  });

  const wording = (e: FieldError): string => {
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
  const errorFor = (field: string) =>
    error?.field === field ? wording(error) : null;

  const index = STEPS.indexOf(step);
  const go = (to: StepId) => {
    setStep(to);
    setReached((r) => Math.max(r, STEPS.indexOf(to)));
  };

  const submit = async () => {
    if (!options) return;
    const fail = (field: string, code: string) => {
      setError({ field, code });
      go(stepOfField(field));
    };
    const whole = (raw: string) => parseWhole(raw);
    const body: NewCharacter = {
      name: draft.name,
      class: draft.cls,
      raceCode: draft.race,
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
    for (const a of ABILITIES) {
      const v = whole(draft.scores[a]);
      if (Number.isNaN(v)) return fail(a, "out_of_range");
      body[a] = v;
    }
    for (const [f, raw] of [
      ["hpMax", draft.hpMax],
      ["xp", xp],
    ] as const) {
      const v = whole(raw);
      if (Number.isNaN(v)) return fail(f, "out_of_range");
      body[f] = v;
    }
    const cls = classOf(options, draft.cls);
    if (
      cls?.exceptionalStrengthRaces.includes(draft.race) &&
      body.str === 18 &&
      draft.strExceptional.trim() !== ""
    ) {
      const v = whole(draft.strExceptional);
      if (v === null || Number.isNaN(v))
        return fail("strExceptional", "out_of_range");
      body.strExceptional = v;
    }
    if ((cls?.skills ?? []).length > 0) {
      body.skillArmor = draft.skillArmor;
      body.skillPoints = {};
      for (const skill of cls!.skills) {
        const raw = draft.skillPoints[skill] ?? "";
        if (raw.trim() === "") continue;
        const v = whole(raw);
        if (v === null || Number.isNaN(v))
          return fail(`skillPoints.${skill}`, "out_of_range");
        body.skillPoints[skill] = v;
      }
    }
    const {
      body: details,
      ageInvalid,
      invalid,
    } = toBody(draft.details, i18n.language);
    if (ageInvalid) return fail("details.age", "out_of_range");
    if (invalid.length > 0)
      return fail(`details.${invalid[0]}`, "not_a_number");
    body.details = details;
    setBusy(true);
    setError(null);
    try {
      await createCharacter(body);
      going.current = true;
      globalThis.location.assign("/");
    } catch (err) {
      setBusy(false);
      if (err instanceof ApiError && err.field) {
        const field = ERROR_FIELD[err.field] ?? err.field;
        setError({ field, code: err.message, message: err.message });
        go(stepOfField(field));
      } else {
        setError({
          field: "name",
          code: "",
          message: err instanceof Error ? err.message : String(err),
        });
        go("identity");
      }
    }
  };

  const leaveNow = () => {
    going.current = true;
    onLeave();
  };
  const leave = () => (dirty ? setConfirmLeave(true) : leaveNow());

  if (loadError || !options)
    return (
      <main className="flex h-dvh items-center justify-center p-3">
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
  };
  const blocked = blockedReason(step, {
    cls: draft.cls,
    race: draft.race,
    xp,
    scores: draft.scores,
    hpMax: draft.hpMax,
  });
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
      className="mx-auto flex h-dvh max-w-[1680px] flex-col gap-3 overflow-hidden p-3"
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
            {step === "identity" && <IdentityStep {...props} />}
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
              {blocked ? t(`creation.blocked.${blocked}`) : null}
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
        <Building options={options} draft={draft} xp={xp} />
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
      default:
        return draft.name.trim() || null;
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

/** The character being built, filled step by step (the dashboard character card's width). */
function Building({
  options,
  draft,
  xp,
}: {
  options: CreationOptions;
  draft: FlowDraft;
  xp: string;
}) {
  const { t } = useTranslation();
  const languageName = useLanguageNames(options);
  const dash = <span className="text-ink-muted">—</span>;
  const line = [
    draft.cls
      ? t(`details.classes.${draft.cls}`, { defaultValue: draft.cls })
      : null,
    draft.race ? t(`details.races.${draft.race}`) : null,
    draft.cls ? levelText(t, { ...draft, xp }) : null,
  ].filter(Boolean);
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
            {draft.details.languages.length > 0
              ? draft.details.languages.map(languageName).join(", ")
              : dash}
          </dd>
        </dl>
      </div>
    </aside>
  );
}
