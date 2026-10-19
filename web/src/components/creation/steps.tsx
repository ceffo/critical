import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import {
  ArrowRight,
  DiceFive,
  Detective,
  HandsPraying,
  Keyboard,
  MagicWand,
  MusicNotes,
  Prohibit,
  Sword,
  UserCircle,
} from "@phosphor-icons/react";
import { rollHp, rollScores } from "@/api/characters";
import { ApiError } from "@/api/http";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import { LanguagePicker } from "@/components/characters/LanguagePicker";
import { Button } from "@/components/ui/button";
import { ALIGNMENT_CODES } from "@/lib/characterDetails";
import {
  alignmentLimit,
  allRaces,
  classOf,
  initialPlacement,
  isPlaced,
  place,
  signed,
  emptyPlacement,
} from "@/lib/creationFlow";
import { raceLanguages } from "@/lib/languages";
import { formatNumber } from "@/lib/format";
import { skillName } from "@/lib/logSummary";
import { useLanguageNames } from "@/lib/useLanguageNames";
import { parseWhole } from "@/lib/wholeNumber";
import { cn } from "@/lib/utils";
import { ABILITIES, type AbilityCode } from "@/types/character";
import { arrangeFor, pickRace } from "./arrange";
import { Choice, Die, Group, Note, Row, Seg } from "./kit";
import type { FlowDraft, StepProps } from "./flow";

const CLASS_ICONS: Record<string, ReactNode> = {
  bard: <MusicNotes aria-hidden />,
  fighter: <Sword aria-hidden />,
  mage: <MagicWand aria-hidden />,
  cleric: <HandsPraying aria-hidden />,
  thief: <Detective aria-hidden />,
};

/** A refusal under a field, worded; the field's id names it for `aria-describedby`. */
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

/** The attributes of an input that may carry a refusal. */
function fieldProps(field: string, text: string | null, extra?: string) {
  const id = `new-character-${field}`;
  return {
    id,
    "aria-invalid": text ? true : undefined,
    "aria-describedby": text ? `${id}-error` : undefined,
    className: cn(fieldClass, text && "border-destructive", extra),
  };
}

// --- Step 1: class ------------------------------------------------------------------------

export function ClassStep({ options, draft, update, errorFor, xp }: StepProps) {
  const { t, i18n } = useTranslation();
  const raceList = allRaces(options);
  const cls = classOf(options, draft.cls);
  const table = cls?.levelXp ?? [];
  const levelsOffered = table.length > 0;
  const mode = levelsOffered ? draft.levelMode : "xp";
  // Inside a sentence: "pour un barde", "Pas barde".
  const className = t(`details.classes.${draft.cls}`, {
    defaultValue: draft.cls,
  }).toLocaleLowerCase();
  const xpError = errorFor("xp");

  const pickClass = (next: string) => {
    const c = classOf(options, next);
    const patch: Partial<FlowDraft> = { cls: next };
    // The class limits the race and the alignment (DMG Table 7, ADR 0032 step 2): a pick it no
    // longer admits is cleared.
    if (draft.race && !c?.races.includes(draft.race)) patch.race = "";
    if (
      draft.details.alignment &&
      !c?.alignments.includes(draft.details.alignment)
    )
      patch.details = { ...draft.details, alignment: "" };
    update(patch);
  };

  return (
    <>
      <div
        role="radiogroup"
        aria-label={t("creation.class.title")}
        className="grid grid-cols-[repeat(auto-fill,minmax(16rem,1fr))] gap-3"
      >
        {options.classes.map((c) => {
          const name = t(`details.classes.${c.class}`, {
            defaultValue: c.class,
          });
          const limit = alignmentLimit(c.alignments, ALIGNMENT_CODES);
          const names = (codes: string[]) =>
            codes.map((a) => t(`details.alignments.${a}`)).join(", ");
          return (
            <Choice
              key={c.class}
              on={draft.cls === c.class}
              onPick={() => pickClass(c.class)}
              label={name}
              testId={`class-${c.class}`}
            >
              <span className="flex items-center gap-2 [&_svg]:size-5">
                {CLASS_ICONS[c.class] ?? <UserCircle aria-hidden />}
                <span className="text-key">{name}</span>
              </span>
              {c.hitDie ? (
                <Row k={t("creation.class.hitDie")} v={`d${c.hitDie}`} />
              ) : null}
              <Row
                k={t("creation.class.alignment")}
                v={
                  limit.kind === "all"
                    ? t("creation.class.allAlignments")
                    : limit.kind === "except"
                      ? t("creation.class.allBut", {
                          list: names(limit.codes),
                        })
                      : names(limit.codes)
                }
              />
              <Row
                k={t("creation.class.races")}
                v={
                  c.races.length === raceList.length
                    ? t("creation.class.allRaces")
                    : c.races.map((r) => t(`details.races.${r}`)).join(", ")
                }
              />
            </Choice>
          );
        })}
      </div>
      <FieldError id="new-character-class" text={errorFor("class")} />

      <Group title={t("creation.level.title")} testId="starting-level">
        <div className="flex flex-wrap items-center gap-4">
          <Seg
            label={t("creation.level.title")}
            value={mode}
            onChange={(v) => update({ levelMode: v })}
            options={[
              {
                value: "level",
                content: t("creation.level.level"),
                disabled: !levelsOffered,
              },
              { value: "xp", content: t("creation.level.xp") },
            ]}
          />
          <p className="min-w-0 flex-1 text-body text-ink-muted">
            {t("creation.level.lead")}
          </p>
        </div>
        {mode === "level" ? (
          <>
            <div
              role="radiogroup"
              aria-label={t("creation.level.level")}
              className="flex flex-wrap gap-1"
            >
              {table.map((r) => (
                <button
                  key={r.level}
                  type="button"
                  role="radio"
                  aria-checked={draft.level === r.level}
                  aria-label={t("creation.level.pick", { level: r.level })}
                  onClick={() => update({ level: r.level })}
                  className={cn(
                    "flex h-8 min-w-9 items-center justify-center rounded-md px-2 text-body tabular-nums outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                    draft.level === r.level
                      ? "chip-chosen font-semibold"
                      : "chip hover:bg-raised-hover",
                  )}
                >
                  {r.level}
                </button>
              ))}
            </div>
            <p className="text-body" data-testid="level-xp">
              {t("creation.level.levelN", { level: draft.level })}
              <span className="text-ink-muted">
                {draft.cls
                  ? t("creation.level.xpOfLevel", {
                      xp: formatNumber(Number(xp), i18n.language),
                      level: draft.level,
                      className,
                    })
                  : t("creation.level.xpOfLevelNoClass")}
              </span>
            </p>
          </>
        ) : (
          <div className="flex w-56 min-w-0 flex-col gap-1">
            <label htmlFor="new-character-xp" className={labelClass}>
              {t("characters.xp")}
            </label>
            <input
              {...fieldProps("xp", xpError)}
              inputMode="numeric"
              autoComplete="off"
              value={draft.xp}
              onChange={(e) => update({ xp: e.target.value })}
            />
          </div>
        )}
        <FieldError id="new-character-xp" text={xpError} />
      </Group>
    </>
  );
}

// --- Step 2: race -------------------------------------------------------------------------

export function RaceStep({
  options,
  draft,
  update,
  errorFor,
  watch,
}: StepProps) {
  const { t } = useTranslation();
  const languageName = useLanguageNames(options);
  const cls = classOf(options, draft.cls);
  // Inside a sentence: "pour un barde", "Pas barde".
  const className = t(`details.classes.${draft.cls}`, {
    defaultValue: draft.cls,
  }).toLocaleLowerCase();
  return (
    <>
      <div
        role="radiogroup"
        aria-label={t("creation.race.title")}
        className="grid grid-cols-[repeat(auto-fill,minmax(16rem,1fr))] gap-3"
      >
        {allRaces(options).map((r) => {
          const ok = !!cls?.races.includes(r);
          const name = t(`details.races.${r}`);
          const adj = options.raceAdjustments?.find(
            (x) => x.race === r,
          )?.adjustments;
          const moved = ABILITIES.filter((a) => (adj?.[a] ?? 0) !== 0);
          const native = raceLanguages(options, r).native;
          return (
            <Choice
              key={r}
              on={draft.race === r}
              off={!ok}
              onPick={() => void pickRace(draft, r, update, watch)}
              label={
                ok ? name : t("creation.race.refusedHint", { name, className })
              }
              testId={`race-${r}`}
            >
              <span className="flex items-center gap-2">
                <span className="text-key">{name}</span>
                {!ok && (
                  <span className="ml-auto flex items-center gap-1 text-label text-danger">
                    <Prohibit aria-hidden weight="bold" className="size-3.5" />
                    {t("creation.race.refused", { className })}
                  </span>
                )}
              </span>
              {adj && (
                <Row
                  k={t("creation.race.adjustments")}
                  v={
                    moved.length === 0
                      ? t("creation.race.noAdjustment")
                      : moved
                          .map((a) => `${t(`attr.${a}`)} ${signed(adj[a]!)}`)
                          .join(" · ")
                  }
                />
              )}
              {native && (
                <Row k={t("creation.race.native")} v={languageName(native)} />
              )}
            </Choice>
          );
        })}
      </div>
      <FieldError id="new-character-raceCode" text={errorFor("raceCode")} />
    </>
  );
}

// --- Step 3: ability scores ---------------------------------------------------------------

export function ScoresStep({
  options,
  draft,
  update,
  errorFor,
  watch,
}: StepProps) {
  const { t } = useTranslation();
  const methods = options.scoreMethods ?? [];
  const [busy, setBusy] = useState(false);
  const [rollError, setRollError] = useState<string | null>(null);
  const [placeError, setPlaceError] = useState<string | null>(null);
  const mode = methods.length > 0 ? draft.scoreMode : "type";
  const roll = draft.roll;
  // Inside a sentence: "pour un barde", "Pas barde".
  const className = t(`details.classes.${draft.cls}`, {
    defaultValue: draft.cls,
  }).toLocaleLowerCase();

  const doRoll = async () => {
    if (!draft.method) return;
    setBusy(true);
    setRollError(null);
    setPlaceError(null);
    // A reply that lands after another change (an edited score) is dropped, its refusal too.
    const current = watch();
    try {
      const r = await rollScores(draft.method, draft.race);
      const patch = {
        roll: r,
        picked: null,
        ...(await arrangeFor(r, initialPlacement(r), draft.race, draft)),
      };
      if (current()) update(patch);
    } catch (e) {
      if (!current()) return;
      setRollError(
        e instanceof ApiError && e.field
          ? t(`creation.rollErrors.${e.message}`, {
              defaultValue: e.message,
            })
          : e instanceof Error
            ? e.message
            : String(e),
      );
    } finally {
      setBusy(false);
    }
  };

  // The placement is the player's choice; its scores, and a refusal (method VI above its max), are
  // the server's.
  const placeOn = async (ability: AbilityCode) => {
    if (!roll || draft.picked === null) return;
    const next = place(roll, draft.placement, draft.picked, ability);
    if (!next) return;
    setBusy(true);
    setPlaceError(null);
    const current = watch();
    try {
      const patch = {
        picked: null,
        ...(await arrangeFor(roll, next, draft.race, draft)),
      };
      if (current()) update(patch);
    } catch (e) {
      if (!current()) return;
      setPlaceError(
        e instanceof ApiError && e.field
          ? t(`creation.placeErrors.${e.message}`, {
              ability: t(`abilityCheck.names.${ability}`),
              max: roll.method.max,
              defaultValue: e.message,
            })
          : e instanceof Error
            ? e.message
            : String(e),
      );
    } finally {
      setBusy(false);
    }
  };

  const scoreInput = (a: AbilityCode, compact?: boolean) => {
    const text = errorFor(a);
    return (
      <div key={a} className="flex min-w-0 flex-col gap-1">
        <label
          htmlFor={`new-character-${a}`}
          className={cn(labelClass, compact && "sr-only")}
        >
          {compact ? t(`abilityCheck.names.${a}`) : t(`attr.${a}`)}
        </label>
        <input
          {...fieldProps(a, text, compact ? "w-16 tabular-nums" : undefined)}
          inputMode="numeric"
          autoComplete="off"
          title={t("characters.abilityHint", {
            name: t(`abilityCheck.names.${a}`),
            min: options.ability.min,
            max: options.ability.max,
          })}
          value={draft.scores[a]}
          onChange={(e) =>
            update({ scores: { ...draft.scores, [a]: e.target.value } })
          }
        />
        <FieldError id={`new-character-${a}`} text={text} />
      </div>
    );
  };

  const placedOn = (i: number): AbilityCode | undefined =>
    ABILITIES.find((a) => draft.placement[a].includes(i));

  return (
    <>
      <Seg
        label={t("creation.scores.mode")}
        value={mode}
        onChange={(v) => update({ scoreMode: v })}
        testId="score-mode"
        options={[
          {
            value: "roll",
            disabled: methods.length === 0,
            content: (
              <>
                <DiceFive aria-hidden />
                {t("creation.scores.roll")}
              </>
            ),
          },
          {
            value: "type",
            content: (
              <>
                <Keyboard aria-hidden />
                {t("creation.scores.type")}
              </>
            ),
          },
        ]}
      />
      {mode === "type" ? (
        <div className="flex flex-col gap-2">
          <p className="text-body text-ink-muted">
            {t("characters.abilitiesLead")}
          </p>
          <div className="grid grid-cols-3 gap-3 sm:grid-cols-6">
            {ABILITIES.map((a) => scoreInput(a))}
          </div>
        </div>
      ) : (
        <>
          <Group title={t("creation.scores.method")}>
            <div
              role="radiogroup"
              aria-label={t("creation.scores.method")}
              className="grid grid-cols-[repeat(auto-fill,minmax(12rem,1fr))] gap-2"
            >
              {methods.map((m) => (
                <button
                  key={m.code}
                  type="button"
                  role="radio"
                  aria-checked={draft.method === m.code}
                  onClick={() =>
                    update({
                      method: m.code,
                      ...(draft.method !== m.code
                        ? {
                            roll: null,
                            placement: emptyPlacement(),
                            picked: null,
                            arranged: null,
                          }
                        : {}),
                    })
                  }
                  className={cn(
                    "flex items-baseline gap-2 rounded-md px-3 py-2 text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                    draft.method === m.code
                      ? "chip-chosen"
                      : "chip hover:bg-raised-hover",
                  )}
                >
                  <span className="text-title">{m.code}</span>
                  <span className="min-w-0 text-body break-words">
                    {t(`creation.methods.${m.code}`, {
                      defaultValue: m.code,
                    })}
                  </span>
                </button>
              ))}
            </div>
          </Group>
          {rollError && <Note kind="refusal">{rollError}</Note>}
          <div className="grid grid-cols-[repeat(auto-fit,minmax(18rem,1fr))] gap-5">
            <Group
              title={
                roll
                  ? t("creation.scores.rollOf", { method: roll.method.code })
                  : t("creation.scores.rollTitle")
              }
              testId="score-roll"
              right={
                <Button
                  variant="quiet"
                  size="sm"
                  disabled={!draft.method || busy}
                  onClick={() => void doRoll()}
                  aria-label={t("creation.scores.rollHint")}
                  title={t("creation.scores.rollHint")}
                >
                  <DiceFive aria-hidden />
                  {roll ? t("creation.reroll") : t("creation.scores.roll")}
                </Button>
              }
            >
              {roll ? (
                <>
                  <ol className="flex flex-col gap-1.5">
                    {roll.rolls.map((r, i) => {
                      const on = placedOn(i);
                      const target =
                        roll.method.assign === "keep_per_ability" ||
                        roll.method.assign === "in_order"
                          ? (r.ability as AbilityCode)
                          : on;
                      const pickable = roll.method.assign !== "in_order";
                      return (
                        <li key={i}>
                          <button
                            type="button"
                            disabled={!pickable}
                            aria-pressed={
                              pickable ? draft.picked === i : undefined
                            }
                            aria-label={t("creation.scores.totalHint", {
                              n: i + 1,
                              total: r.total,
                            })}
                            onClick={() =>
                              update({
                                picked: draft.picked === i ? null : i,
                              })
                            }
                            data-testid={`score-total-${i}`}
                            className={cn(
                              "flex w-full flex-wrap items-center gap-2 rounded-md px-2 py-1 text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                              draft.picked === i
                                ? "chip-chosen"
                                : "bg-raised enabled:hover:bg-raised-hover",
                              pickable &&
                                isPlaced(draft.placement, i) &&
                                draft.picked !== i &&
                                "opacity-70",
                            )}
                          >
                            <span className="w-5 text-label text-ink-muted">
                              {i + 1}
                            </span>
                            <span className="flex gap-1">
                              {r.dice.map((d, j) => (
                                <Die key={j} v={d} dropped={j === r.dropped} />
                              ))}
                            </span>
                            {r.dice.length > 1 && (
                              <>
                                <span className="text-body text-ink-muted">
                                  =
                                </span>
                                <span className="w-6 text-key tabular-nums">
                                  {r.total}
                                </span>
                              </>
                            )}
                            {target && (
                              <>
                                <ArrowRight
                                  aria-hidden
                                  className="size-3.5 text-ink-muted"
                                />
                                <span className="text-body">
                                  {t(`abilityCheck.names.${target}`)}
                                </span>
                              </>
                            )}
                          </button>
                        </li>
                      );
                    })}
                  </ol>
                  <p className="text-label text-ink-muted">
                    {t(`creation.scores.how.${roll.method.assign}`, {
                      base: roll.method.base,
                      max: roll.method.max,
                    })}
                  </p>
                </>
              ) : (
                <p className="text-body text-ink-muted">
                  {draft.method
                    ? t("creation.scores.rollLead")
                    : t("creation.scores.pickMethod")}
                </p>
              )}
            </Group>
            <Group title={t("creation.scores.arrangement")}>
              <table className="w-full text-body" data-testid="arrangement">
                <thead>
                  <tr className="text-left text-label text-ink-muted">
                    <th className="pb-1 font-medium">
                      {t("creation.scores.ability")}
                    </th>
                    <th className="pb-1 font-medium">
                      {t("creation.scores.rolled")}
                    </th>
                    <th className="pb-1 font-medium">
                      {t("creation.scores.race")}
                    </th>
                    <th className="pb-1 font-medium">
                      {t("creation.scores.score")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {ABILITIES.map((a) => {
                    const shown = roll ? draft.arranged?.rolled[a] : undefined;
                    const adj = draft.arranged
                      ? draft.arranged.adjustments?.[a]
                      : roll?.adjustments?.[a];
                    return (
                      <tr key={a} className="border-t border-border">
                        <td className="py-1.5 pr-2">
                          {t(`abilityCheck.names.${a}`)}
                        </td>
                        <td className="pr-2">
                          <button
                            type="button"
                            disabled={!roll || draft.picked === null || busy}
                            onClick={() => void placeOn(a)}
                            aria-label={t("creation.scores.placeHint", {
                              ability: t(`abilityCheck.names.${a}`),
                            })}
                            data-testid={`place-${a}`}
                            className={cn(
                              "inline-flex h-6 min-w-8 items-center justify-center rounded-md px-1.5 tabular-nums outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                              shown != null
                                ? "chip-chosen"
                                : "chip text-ink-muted",
                              draft.picked !== null &&
                                "enabled:hover:bg-raised-hover",
                            )}
                          >
                            {shown ?? "—"}
                          </button>
                        </td>
                        <td className="pr-2 text-ink-muted tabular-nums">
                          {adj == null ? "—" : signed(adj)}
                        </td>
                        <td className="py-1">{scoreInput(a, true)}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
              {placeError && <Note kind="refusal">{placeError}</Note>}
              <p className="text-label text-ink-muted">
                {t("creation.scores.editable", { className })}
              </p>
            </Group>
          </div>
        </>
      )}
    </>
  );
}

// --- Step 4: hit points and the class extras ----------------------------------------------

export function ExtrasStep({
  options,
  draft,
  update,
  errorFor,
  xp,
  watch,
}: StepProps) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [rollError, setRollError] = useState<string | null>(null);
  const cls = classOf(options, draft.cls);
  const skills = cls?.skills ?? [];
  const armors = cls?.skillArmors ?? [];
  const offersExceptional =
    !!cls?.exceptionalStrengthRaces.includes(draft.race) &&
    parseWhole(draft.scores.str) === 18;
  const con = parseWhole(draft.scores.con);
  const xpValue = parseWhole(xp);
  const canRoll =
    con !== null &&
    !Number.isNaN(con) &&
    xpValue !== null &&
    !Number.isNaN(xpValue);
  const hp = draft.hpRoll;
  // Inside a sentence: "pour un barde", "Pas barde".
  const className = t(`details.classes.${draft.cls}`, {
    defaultValue: draft.cls,
  }).toLocaleLowerCase();

  const doRoll = async () => {
    if (!canRoll) return;
    setBusy(true);
    setRollError(null);
    const current = watch();
    try {
      const r = await rollHp(draft.cls, con, xpValue);
      if (!current()) return;
      update({
        hpRoll: r,
        hpRollFor: { cls: draft.cls, con, xp: xpValue },
        hpOutdated: false,
        hpMax: String(r.total),
      });
    } catch (e) {
      if (!current()) return;
      setRollError(
        e instanceof ApiError && e.field
          ? t(`creation.rollErrors.${e.message}`, {
              defaultValue: e.message,
            })
          : e instanceof Error
            ? e.message
            : String(e),
      );
    } finally {
      setBusy(false);
    }
  };

  const hpError = errorFor("hpMax");
  const strError = errorFor("strExceptional");
  const armorError = errorFor("skillArmor");

  return (
    <>
      <div className="grid grid-cols-[repeat(auto-fit,minmax(18rem,1fr))] gap-5">
        <Group
          title={t("creation.extras.hp")}
          testId="hp-roll"
          right={
            <Button
              variant="quiet"
              size="sm"
              disabled={!canRoll || busy}
              onClick={() => void doRoll()}
              aria-label={t("creation.extras.rollHint")}
              title={t("creation.extras.rollHint")}
            >
              <DiceFive aria-hidden />
              {hp ? t("creation.reroll") : t("creation.scores.roll")}
            </Button>
          }
        >
          {rollError && <Note kind="refusal">{rollError}</Note>}
          {!hp && draft.hpOutdated && (
            <Note kind="warning" testId="hp-outdated">
              {t("creation.extras.hpOutdated")}
            </Note>
          )}
          {hp ? (
            <>
              <table className="w-full text-body">
                <thead>
                  <tr className="text-left text-label text-ink-muted">
                    <th className="pb-1 font-medium">
                      {t("creation.extras.level")}
                    </th>
                    <th className="pb-1 font-medium">
                      {t("creation.extras.die")}
                    </th>
                    <th className="pb-1 font-medium">{t("attr.con")}</th>
                    <th className="pb-1 font-medium">
                      {t("creation.extras.gain")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {hp.levels.map((l) => (
                    <tr key={l.level} className="border-t border-border">
                      <td className="py-1.5">{l.level}</td>
                      <td>
                        {l.fixed ? (
                          <span className="text-ink-muted">
                            {t("creation.extras.fixed")}
                          </span>
                        ) : (
                          <span className="flex items-center gap-1.5">
                            <span className="text-ink-muted">d{l.hitDie}</span>
                            <Die v={l.rolled ?? 0} />
                            {l.counted !== null && l.counted !== l.rolled && (
                              <span className="text-label text-ink-muted">
                                {t("creation.extras.counted", {
                                  n: l.counted,
                                })}
                              </span>
                            )}
                          </span>
                        )}
                      </td>
                      <td className="text-ink-muted tabular-nums">
                        {l.fixed ? "—" : signed(l.conAdjustment)}
                      </td>
                      <td className="tabular-nums">{signed(l.gain)}</td>
                    </tr>
                  ))}
                  <tr className="border-t border-border">
                    <td className="py-1.5 font-medium" colSpan={3}>
                      {t("creation.extras.total")}
                    </td>
                    <td className="text-key tabular-nums">{hp.total}</td>
                  </tr>
                </tbody>
              </table>
              <p className="text-label text-ink-muted">
                {t("creation.extras.hpLead", { level: hp.level })}
              </p>
            </>
          ) : (
            <p className="text-body text-ink-muted">
              {canRoll
                ? t("creation.extras.hpRollLead")
                : t("creation.extras.hpNeedsCon")}
            </p>
          )}
          <div className="flex w-40 min-w-0 flex-col gap-1">
            <label htmlFor="new-character-hpMax" className={labelClass}>
              {t("characters.hpMax")}
            </label>
            <input
              {...fieldProps("hpMax", hpError)}
              inputMode="numeric"
              autoComplete="off"
              title={t("characters.range", { ...options.hpMax })}
              value={draft.hpMax}
              onChange={(e) => update({ hpMax: e.target.value })}
            />
          </div>
          <FieldError id="new-character-hpMax" text={hpError} />
          <p className="text-label text-ink-muted">
            {t("creation.extras.hpEditable")}
          </p>
        </Group>

        {(skills.length > 0 || offersExceptional) && (
          <div className="flex min-w-0 flex-col gap-5">
            {skills.length > 0 && (
              <Group
                title={t("creation.extras.skills", { className })}
                testId="thief-skills-entry"
              >
                <p className="text-label text-ink-muted">
                  {t("characters.skillPointsLead")}
                </p>
                <table className="w-full text-body">
                  <thead>
                    <tr className="text-left text-label text-ink-muted">
                      <th className="pb-1 font-medium">
                        {t("creation.extras.skill")}
                      </th>
                      <th className="pb-1 font-medium">
                        {t("creation.extras.points")}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {skills.map((skill) => {
                      const f = `skillPoints.${skill}`;
                      const text = errorFor(f);
                      return (
                        <tr key={skill} className="border-t border-border">
                          <td className="py-1.5 pr-2">
                            <label htmlFor={`new-character-${f}`}>
                              {skillName(t, skill)}
                            </label>
                          </td>
                          <td className="py-1">
                            <input
                              {...fieldProps(f, text, "w-20 tabular-nums")}
                              inputMode="numeric"
                              autoComplete="off"
                              title={t("characters.range", {
                                ...options.skillPoints,
                              })}
                              value={draft.skillPoints[skill] ?? ""}
                              onChange={(e) =>
                                update({
                                  skillPoints: {
                                    ...draft.skillPoints,
                                    [skill]: e.target.value,
                                  },
                                })
                              }
                            />
                            <FieldError id={`new-character-${f}`} text={text} />
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
                <FieldError
                  id="new-character-skillPoints"
                  text={errorFor("skillPoints")}
                />
                <div className="flex w-56 min-w-0 flex-col gap-1">
                  <label
                    htmlFor="new-character-skillArmor"
                    className={labelClass}
                  >
                    {t("characters.skillArmor")}
                  </label>
                  <select
                    {...fieldProps("skillArmor", armorError)}
                    value={draft.skillArmor}
                    onChange={(e) => update({ skillArmor: e.target.value })}
                  >
                    {armors.map((a) => (
                      <option key={a} value={a}>
                        {t(`characters.armors.${a}`, { defaultValue: a })}
                      </option>
                    ))}
                  </select>
                  <FieldError id="new-character-skillArmor" text={armorError} />
                </div>
              </Group>
            )}
            {offersExceptional && (
              <Group title={t("characters.strExceptional")}>
                <div className="flex w-40 min-w-0 flex-col gap-1">
                  <label
                    htmlFor="new-character-strExceptional"
                    className={labelClass}
                  >
                    {t("creation.extras.percentile")}
                  </label>
                  <input
                    {...fieldProps("strExceptional", strError)}
                    inputMode="numeric"
                    autoComplete="off"
                    title={t("characters.strExceptionalHint", {
                      min: options.strExceptional.min,
                      max: options.strExceptional.max,
                    })}
                    value={draft.strExceptional}
                    onChange={(e) => update({ strExceptional: e.target.value })}
                  />
                </div>
                <FieldError id="new-character-strExceptional" text={strError} />
              </Group>
            )}
          </div>
        )}
      </div>
      {skills.length === 0 && !offersExceptional && (
        <Note kind="info">
          {t("creation.extras.nothingElse", { className })}
        </Note>
      )}
    </>
  );
}

// --- Step 5: identity (today's name and details, until its own screen) --------------------

const DETAIL_FIELDS = ["age", "height", "weight", "hair", "eyes"] as const;

export function IdentityStep({ options, draft, update, errorFor }: StepProps) {
  const { t } = useTranslation();
  const cls = classOf(options, draft.cls);
  const nameError = errorFor("name");
  const setDetail = (f: keyof FlowDraft["details"], v: string) =>
    update({ details: { ...draft.details, [f]: v } });

  const pick = (f: "alignment" | "sex", codes: string[]) => {
    const field = `details.${f}`;
    const text = errorFor(field);
    return (
      <div key={f} className="flex min-w-0 flex-col gap-1">
        <label
          htmlFor={`new-character-${field}`}
          className={cn(labelClass, "break-words")}
        >
          {t(`details.fields.${f}`)}
        </label>
        <select
          {...fieldProps(field, text)}
          value={draft.details[f]}
          onChange={(e) => setDetail(f, e.target.value)}
        >
          <option value="">{t("details.pick")}</option>
          {codes.map((code) => (
            <option key={code} value={code}>
              {t(`details.${f === "sex" ? "sexes" : "alignments"}.${code}`)}
            </option>
          ))}
        </select>
        <FieldError id={`new-character-${field}`} text={text} />
      </div>
    );
  };

  return (
    <div className="flex flex-col gap-5" data-testid="new-character-details">
      <div className="flex max-w-[28rem] min-w-0 flex-col gap-1">
        <label htmlFor="new-character-name" className={labelClass}>
          {t("characters.name")}
        </label>
        <input
          {...fieldProps("name", nameError)}
          autoComplete="off"
          maxLength={options.maxNameLength * 2}
          value={draft.name}
          onChange={(e) => update({ name: e.target.value })}
        />
        <FieldError id="new-character-name" text={nameError} />
      </div>
      <p className="text-body text-ink-muted">{t("characters.detailsLead")}</p>
      <div className="grid grid-cols-[repeat(auto-fill,minmax(12rem,1fr))] gap-3">
        {pick("alignment", cls?.alignments ?? [])}
        {pick("sex", options.sexes)}
        {DETAIL_FIELDS.map((f) => {
          const field = `details.${f}`;
          const text = errorFor(field);
          return (
            <div key={f} className="flex min-w-0 flex-col gap-1">
              <label
                htmlFor={`new-character-${field}`}
                className={cn(labelClass, "break-words")}
              >
                {t(`details.fields.${f}`)}
              </label>
              <input
                {...fieldProps(field, text)}
                autoComplete="off"
                inputMode={f === "age" ? "numeric" : undefined}
                value={draft.details[f]}
                onChange={(e) => setDetail(f, e.target.value)}
              />
              <FieldError id={`new-character-${field}`} text={text} />
            </div>
          );
        })}
      </div>
      {(options.languages ?? []).length > 0 && (
        <div className="flex min-w-0 flex-col gap-1">
          <span className={labelClass}>{t("details.fields.languages")}</span>
          <LanguagePicker
            idPrefix="new-character-languages"
            options={options}
            race={draft.race}
            picked={draft.details.languages}
            onChange={(languages) =>
              update({ details: { ...draft.details, languages } })
            }
          />
          <FieldError
            id="new-character-details.languages"
            text={errorFor("details.languages")}
          />
        </div>
      )}
    </div>
  );
}
