import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Minus, Plus, X } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import { ApiError } from "@/api/http";
import {
  attackPower,
  contactPower,
  defend,
  getCostTable,
  invokePower,
  previewContact,
  type AttackResult,
  type ContactPreview,
  type CostTable,
  type DefendResult,
  type PowerCheckResult,
} from "@/api/psionics";
import { localized } from "@/i18n/localized";
import { useTableStore } from "@/stores/table";
import type { Power } from "@/types/powers";
import { EMPTY_ATTACK, attackRequest, type AttackDraft } from "../attack";
import { CHECK_DIE, EMPTY_DEFEND, defendRequest } from "../defend";
import {
  EMPTY_DRAFT,
  MAX_ENTERED_SCORE,
  asks,
  requestOf,
  type UseDraft,
} from "../powerUse";
import { useBuffsStore } from "@/stores/buffs";
import { contactMinds } from "./model";
import { fixedCost, formOf, type TargetDraft } from "./bar";
import {
  controlsOf,
  initialPicks,
  tableRequest,
  type Control,
  type Picks,
} from "./tableCost";
import { useCatalogue } from "./useCatalogue";
import { AttackOutcome, DefendOutcome, UseOutcome } from "./outcomes";

/** What the bar is open on: a known power, or an attack on the character the DM makes. */
export type Selection = { kind: "power"; power: Power } | { kind: "incoming" };

type Modifiers = AttackDraft["modifiers"];

/**
 * The action bar (ADR 0047 item 2), docked under the Powers lists: one form for every entry made
 * before a roll, picked by the shape of the power's data (`formOf`), or the incoming attack's. It
 * sends the requests the old dialogs did; the checks and the contests are the server's. Keyed by the
 * caller on the selection, so each opening starts a fresh form.
 */
export function ActionBar({
  selection,
  target,
  modifiers,
  onClose,
}: {
  selection: Selection;
  target: TargetDraft;
  /** The tab's adjustment, as the labelled modifier the request carries. */
  modifiers: Modifiers;
  onClose: () => void;
}) {
  const { t, i18n } = useTranslation();
  const incoming = selection.kind === "incoming";
  const name = incoming
    ? t("powers.tab.bar.incoming")
    : localized(i18n.language, selection.power.name);
  const form = incoming ? "incoming" : formOf(selection.power);
  return (
    <section
      className="flex flex-col gap-2 rounded-lg border border-ring/60 bg-raised px-3 py-2"
      data-testid="action-bar"
      data-form={form}
      aria-label={name}
    >
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-body font-medium" lang={i18n.language}>
          {name}
        </h3>
        <Button
          className="px-1.5"
          aria-label={t("powers.tab.bar.close")}
          onClick={onClose}
        >
          <X aria-hidden />
        </Button>
      </div>
      {selection.kind === "incoming" ? (
        <IncomingForm />
      ) : form === "attack" ? (
        <AttackBarForm
          power={selection.power}
          target={target}
          modifiers={modifiers}
        />
      ) : form === "table" ? (
        <TableBarForm
          power={selection.power}
          target={target}
          modifiers={modifiers}
        />
      ) : form === "contact" ? (
        <ContactBarForm
          power={selection.power}
          target={target}
          modifiers={modifiers}
        />
      ) : (
        <UseBarForm
          power={selection.power}
          variable={form === "variable"}
          modifiers={modifiers}
        />
      )}
    </section>
  );
}

/** Runs a request through the table store (busy flag, reads again what it changed); a 4xx is the form's to word. */
async function send<T>(
  request: () => Promise<T>,
): Promise<{ done: T } | { refused: string }> {
  let done: T | undefined;
  let refused = "";
  await useTableStore.getState().submit(async () => {
    try {
      done = await request();
    } catch (e) {
      if (e instanceof ApiError && e.status < 500) {
        refused = e.message;
        return;
      }
      throw e;
    }
  });
  return done !== undefined ? { done } : { refused };
}

const row = "flex flex-wrap items-end gap-x-4 gap-y-2";

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="flex flex-col gap-0.5">
      <span className={labelClass}>{label}</span>
      {children}
    </label>
  );
}

function Refusal({ text, testId }: { text: string | null; testId: string }) {
  return text ? (
    <p role="alert" className="text-body text-danger" data-testid={testId}>
      {text}
    </p>
  ) : null;
}

/** A whole-number field with its − and + (the design's cost steppers). */
function Stepper({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (next: string) => void;
}) {
  const { t } = useTranslation();
  const step = (by: number) => {
    const n = Number.parseInt(value, 10);
    onChange(String(Math.max(1, (Number.isNaN(n) ? 0 : n) + by)));
  };
  return (
    <div className="flex flex-col gap-0.5">
      <span className={labelClass}>{label}</span>
      <span className="flex items-center gap-1">
        <Button
          className="px-1.5"
          aria-label={t("powers.tab.bar.less", { label })}
          onClick={() => step(-1)}
        >
          <Minus aria-hidden />
        </Button>
        <input
          className={`${fieldClass} !w-14 text-center tabular-nums`}
          inputMode="numeric"
          autoComplete="off"
          aria-label={label}
          value={value}
          onChange={(e) => onChange(e.target.value)}
        />
        <Button
          className="px-1.5"
          aria-label={t("powers.tab.bar.more", { label })}
          onClick={() => step(1)}
        >
          <Plus aria-hidden />
        </Button>
      </span>
    </div>
  );
}

type Roll = { roll: string; success: boolean };

/** The DM's rolls, each with whether its check succeeded. */
function Rolls({
  rolls,
  onChange,
}: {
  rolls: Roll[];
  onChange: (next: Roll[]) => void;
}) {
  const { t } = useTranslation();
  return (
    <fieldset
      className="flex flex-wrap items-end gap-x-4 gap-y-2"
      aria-label={t("powers.defence.rolls")}
    >
      <span className={`${labelClass} self-center`}>
        {t("powers.defence.rolls")}
      </span>
      {rolls.map((r, i) => (
        <span key={i} className="flex items-center gap-1.5">
          <input
            className={`${fieldClass} !w-14 text-center tabular-nums`}
            inputMode="numeric"
            autoComplete="off"
            aria-label={t("powers.defence.rollN", { n: i + 1 })}
            placeholder={`1–${CHECK_DIE}`}
            value={r.roll}
            onChange={(e) =>
              onChange(
                rolls.map((x, j) =>
                  j === i ? { ...x, roll: e.target.value } : x,
                ),
              )
            }
          />
          <label className="flex items-center gap-1.5 text-body">
            <input
              type="checkbox"
              checked={r.success}
              onChange={(e) =>
                onChange(
                  rolls.map((x, j) =>
                    j === i ? { ...x, success: e.target.checked } : x,
                  ),
                )
              }
            />
            {t("powers.defence.succeeded", { n: i + 1 })}
          </label>
        </span>
      ))}
    </fieldset>
  );
}

/** An attack mode: the target comes from the Target band; the bar takes the DM's two rolls (with a defence up). */
function AttackBarForm({
  power,
  target,
  modifiers,
}: {
  power: Power;
  target: TargetDraft;
  modifiers: Modifiers;
}) {
  const { t, i18n } = useTranslation();
  const busy = useTableStore((s) => s.busy);
  const [rolls, setRolls] = useState<Roll[]>(EMPTY_ATTACK.rolls);
  const [result, setResult] = useState<AttackResult | null>(null);
  const [refused, setRefused] = useState<string | null>(null);
  const req = attackRequest({ ...EMPTY_ATTACK, ...target, rolls, modifiers });
  const name = localized(i18n.language, power.name);
  const fresh =
    <A,>(f: (a: A) => void) =>
    (a: A) => {
      f(a);
      setResult(null);
      setRefused(null);
    };

  async function attack() {
    if (!req || busy) return;
    setRefused(null);
    const out = await send(
      async () => (await attackPower(power.id, req)).attack,
    );
    if ("done" in out) setResult(out.done);
    else setRefused(out.refused);
  }

  return (
    <form
      className="flex flex-col gap-2"
      data-testid="attack-form"
      onSubmit={(e) => {
        e.preventDefault();
        void attack();
      }}
    >
      <div className={row}>
        {target.defence ? (
          <Rolls rolls={rolls} onChange={fresh(setRolls)} />
        ) : (
          <span className="text-body text-ink-muted">
            {t("powers.tab.bar.noRolls")}
          </span>
        )}
        <Button
          type="submit"
          variant="main"
          className="ml-auto"
          disabled={busy || !req}
          aria-label={t("powers.attack.attackNamed", { name })}
        >
          {t("powers.attack.attack")}
        </Button>
      </div>
      {!target.target.trim() && (
        <span className="text-label text-ink-muted">
          {t("powers.tab.bar.needTarget")}
        </span>
      )}
      <Refusal text={refused} testId="attack-refused" />
      {result && <AttackOutcome result={result} />}
    </form>
  );
}

/**
 * A power paid on contact: used on one of the minds a contact row holds (made in an earlier round).
 * With none, the bar says so; a mind the Target band names without a contact is still sent, and the
 * server's refusal is shown.
 */
function ContactBarForm({
  power,
  target,
  modifiers,
}: {
  power: Power;
  target: TargetDraft;
  modifiers: Modifiers;
}) {
  const { t, i18n } = useTranslation();
  const busy = useTableStore((s) => s.busy);
  const buffs = useBuffsStore((s) => s.buffs);
  const minds = contactMinds(buffs).map((m) => m.name);
  const named = target.target.trim();
  const options = named && !minds.includes(named) ? [...minds, named] : minds;
  const [chosen, setChosen] = useState("");
  const mind = chosen || (minds.includes(named) ? named : (minds[0] ?? named));
  const [result, setResult] = useState<PowerCheckResult | null>(null);
  const [refused, setRefused] = useState<string | null>(null);
  const need = asks(power);
  const [score, setScore] = useState("");
  const req = requestOf(power, { ...EMPTY_DRAFT, score, modifiers });
  const name = localized(i18n.language, power.name);

  async function use() {
    if (!req || !mind || busy) return;
    setRefused(null);
    const out = await send(
      async () => (await invokePower(power.id, { ...req, target: mind })).check,
    );
    if ("done" in out) setResult(out.done);
    else setRefused(out.refused);
  }

  return (
    <form
      className="flex flex-col gap-2"
      data-testid="contact-form"
      onSubmit={(e) => {
        e.preventDefault();
        void use();
      }}
    >
      <div className={row}>
        {options.length > 0 ? (
          <Field label={t("powers.tab.bar.contactMind")}>
            <select
              className={fieldClass}
              value={mind}
              aria-label={t("powers.tab.bar.contactMind")}
              onChange={(e) => {
                setChosen(e.target.value);
                setResult(null);
                setRefused(null);
              }}
            >
              {options.map((m) => (
                <option key={m} value={m}>
                  {m}
                </option>
              ))}
            </select>
          </Field>
        ) : (
          <span className="text-body text-ink-muted" data-testid="no-contact">
            {t("powers.tab.bar.noContact")}
          </span>
        )}
        {need.score && (
          <Field label={t("powers.use.score")}>
            <input
              className={`${fieldClass} !w-60`}
              inputMode="numeric"
              autoComplete="off"
              aria-label={t("powers.use.score")}
              placeholder={t("powers.use.scoreHint", {
                max: MAX_ENTERED_SCORE,
              })}
              value={score}
              onChange={(e) => setScore(e.target.value)}
            />
          </Field>
        )}
        <Button
          type="submit"
          variant="main"
          className="ml-auto"
          disabled={busy || !req || !mind}
          aria-label={t("powers.use.useNamed", { name })}
        >
          {t("powers.use.use")}
        </Button>
      </div>
      <Refusal text={refused} testId="use-refused" />
      {result && <UseOutcome result={result} />}
    </form>
  );
}

/** The labelled modifiers of the tab's adjustment, as the contact request carries them. */
const labelledOf = (m: Modifiers) =>
  m.flatMap((x) => {
    const value = Number.parseInt(x.value, 10);
    return Number.isNaN(value) || value === 0
      ? []
      : [{ value, reason: x.reason.trim() }];
  });

/** The controls of a table's modifier rows: a toggle for a kind with one row, a menu for several. */
export function TableControls({
  controls,
  picks,
  onPick,
}: {
  controls: Control[];
  picks: Picks;
  onPick: (kind: string, code: string) => void;
}) {
  const { t } = useTranslation();
  const word = (kind: string, code: string) =>
    t(`log.contactWords.${kind}.${code}`, { defaultValue: code });
  const signed = (n: number) => (n > 0 ? `+${n}` : String(n));
  return (
    <>
      {controls.map((c) => {
        const label = t(`log.contactKinds.${c.kind}`, { defaultValue: c.kind });
        return c.as === "toggle" ? (
          <label
            key={c.kind}
            className="flex items-center gap-1.5 self-end text-body"
          >
            <input
              type="checkbox"
              data-kind={c.kind}
              checked={picks[c.kind] === c.rows[0].code}
              onChange={(e) =>
                onPick(c.kind, e.target.checked ? c.rows[0].code : "")
              }
            />
            {word(c.kind, c.rows[0].code)} {signed(c.rows[0].modifier)}
          </label>
        ) : (
          <Field key={c.kind} label={label}>
            <select
              className={fieldClass}
              data-kind={c.kind}
              aria-label={label}
              value={picks[c.kind]}
              onChange={(e) => onPick(c.kind, e.target.value)}
            >
              {c.rows.map((r) => (
                <option key={r.code} value={r.code}>
                  {word(c.kind, r.code)} {signed(r.modifier)}
                </option>
              ))}
            </select>
          </Field>
        );
      })}
    </>
  );
}

/** What the server's preview says: the cost and the score, whether the pool covers it, the warnings. */
export function TablePreview({ preview }: { preview: ContactPreview }) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-0.5" data-testid="table-preview">
      <span className="text-body tabular-nums">
        {t("powers.tab.bar.tableCost", {
          cost: preview.cost,
          target: preview.target,
        })}
      </span>
      {!preview.enough && (
        <span className="text-label text-danger" data-testid="table-short">
          {t("powers.tab.bar.notEnough", { current: preview.current })}
        </span>
      )}
      {(preview.warnings ?? []).map((w) => (
        <span key={w} className="text-label text-danger">
          {t(`powers.use.warning.${w}`, { defaultValue: w })}
        </span>
      ))}
    </div>
  );
}

/**
 * A power whose cost a rule table gives: the opponent comes from the Target band, the controls from the
 * table's modifier rows, and the cost and score from the server's preview after each change (the web
 * finds no row itself). A level the table lacks shows the DM's cost field once the server says so.
 */
function TableBarForm({
  power,
  target,
  modifiers,
}: {
  power: Power;
  target: TargetDraft;
  modifiers: Modifiers;
}) {
  const { t, i18n } = useTranslation();
  const busy = useTableStore((s) => s.busy);
  const tableName = power.initial_cost_table ?? "";
  const [table, setTable] = useState<CostTable | null>(null);
  const [unavailable, setUnavailable] = useState(false);
  useEffect(() => {
    let live = true;
    getCostTable(tableName).then(
      (tb) => live && setTable(tb),
      () => live && setUnavailable(true),
    );
    return () => {
      live = false;
    };
  }, [tableName]);
  const controls = useMemo(() => (table ? controlsOf(table) : []), [table]);
  const [chosen, setChosen] = useState<Picks>({});
  const picks = { ...initialPicks(controls), ...chosen };
  const [entered, setEntered] = useState("1");
  const [pastLevel, setPastLevel] = useState<string | null>(null);
  const levelKey = target.level.trim();
  const dmCost = pastLevel === levelKey;
  const req = table
    ? tableRequest(
        target,
        controls,
        picks,
        labelledOf(modifiers),
        dmCost ? entered : undefined,
      )
    : null;
  const key = JSON.stringify(req);
  const [view, setView] = useState<{
    key: string;
    preview?: ContactPreview;
    refused?: string;
  } | null>(null);
  useEffect(() => {
    if (!req) return;
    let live = true;
    previewContact(power.id, req).then(
      (preview) => live && setView({ key, preview }),
      (e: unknown) => {
        if (!live) return;
        if (e instanceof ApiError && e.status < 500) {
          if (e.code === "level_out_of_table") setPastLevel(levelKey);
          setView({ key, refused: e.message });
          return;
        }
        setView({ key, refused: t("powers.tab.bar.previewFailed") });
      },
    );
    return () => {
      live = false;
    };
    // The request is its JSON: a new key is a new preview.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, power.id]);
  const shown = req && view?.key === key ? view : null;
  const [result, setResult] = useState<PowerCheckResult | null>(null);
  const [refused, setRefused] = useState<string | null>(null);
  const name = localized(i18n.language, power.name);

  async function use() {
    if (!req || !shown?.preview?.enough || busy) return;
    setRefused(null);
    const out = await send(
      async () => (await contactPower(power.id, req)).check,
    );
    if ("done" in out) setResult(out.done);
    else setRefused(out.refused);
  }

  return (
    <form
      className="flex flex-col gap-2"
      data-testid="table-form"
      onSubmit={(e) => {
        e.preventDefault();
        void use();
      }}
    >
      <div className={row}>
        <TableControls
          controls={controls}
          picks={picks}
          onPick={(kind, code) => {
            setChosen((c) => ({ ...c, [kind]: code }));
            setResult(null);
            setRefused(null);
          }}
        />
        {dmCost && (
          <Stepper
            label={t("powers.tab.bar.dmCost")}
            value={entered}
            onChange={(v) => {
              setEntered(v);
              setResult(null);
            }}
          />
        )}
        {shown?.preview && <TablePreview preview={shown.preview} />}
        <Button
          type="submit"
          variant="main"
          className="ml-auto"
          disabled={busy || !shown?.preview?.enough}
          aria-label={t("powers.use.useNamed", { name })}
        >
          {t("powers.use.use")}
        </Button>
      </div>
      {!req && table && (
        <span className="text-label text-ink-muted" data-testid="table-need">
          {t("powers.tab.bar.needOpponent")}
        </span>
      )}
      {unavailable && (
        <Refusal text={t("powers.tab.bar.previewFailed")} testId="table-down" />
      )}
      <Refusal text={shown?.refused ?? null} testId="table-refused" />
      <Refusal text={refused} testId="use-refused" />
      {result && <UseOutcome result={result} />}
    </form>
  );
}

/** Any other power: the cost fields its data asks for (a variable cost, a variable maintenance, a missing score), else the confirmation. */
function UseBarForm({
  power,
  variable,
  modifiers,
}: {
  power: Power;
  variable: boolean;
  modifiers: Modifiers;
}) {
  const { t, i18n } = useTranslation();
  const busy = useTableStore((s) => s.busy);
  const need = asks(power);
  const [draft, setDraft] = useState<UseDraft>({
    ...EMPTY_DRAFT,
    cost: need.cost ? "1" : "",
    maintenanceCost: need.maintenanceCost ? "1" : "",
  });
  const [result, setResult] = useState<PowerCheckResult | null>(null);
  const [refused, setRefused] = useState<string | null>(null);
  const req = requestOf(power, { ...draft, modifiers });
  const name = localized(i18n.language, power.name);
  const set = (patch: Partial<UseDraft>) => {
    setDraft((d) => ({ ...d, ...patch }));
    setResult(null);
    setRefused(null);
  };

  async function use() {
    if (!req || busy) return;
    setRefused(null);
    const out = await send(
      async () => (await invokePower(power.id, req)).check,
    );
    if ("done" in out) setResult(out.done);
    else setRefused(out.refused);
  }

  const cost = fixedCost(power);
  return (
    <form
      className="flex flex-col gap-2"
      data-testid="use-power"
      onSubmit={(e) => {
        e.preventDefault();
        void use();
      }}
    >
      <div className={row}>
        {variable && need.cost && (
          <Stepper
            label={t("powers.tab.bar.initialCost")}
            value={draft.cost}
            onChange={(cost) => set({ cost })}
          />
        )}
        {variable && need.maintenanceCost && (
          <Stepper
            label={t("powers.tab.bar.maintenanceCost")}
            value={draft.maintenanceCost}
            onChange={(maintenanceCost) => set({ maintenanceCost })}
          />
        )}
        {need.score && (
          <Field label={t("powers.use.score")}>
            <input
              className={`${fieldClass} !w-60`}
              inputMode="numeric"
              autoComplete="off"
              aria-label={t("powers.use.score")}
              placeholder={t("powers.use.scoreHint", {
                max: MAX_ENTERED_SCORE,
              })}
              value={draft.score}
              onChange={(e) => set({ score: e.target.value })}
            />
          </Field>
        )}
        {!variable && !need.score && (
          <span className="text-body text-ink-muted">
            {cost !== undefined
              ? t("lookup.powers.psp", { count: cost })
              : t("powers.tab.bar.confirm")}
          </span>
        )}
        <Button
          type="submit"
          variant="main"
          className="ml-auto"
          disabled={busy || !req}
          aria-label={t("powers.use.useNamed", { name })}
        >
          {t("powers.use.use")}
        </Button>
      </div>
      <Refusal text={refused} testId="use-refused" />
      {result && <UseOutcome result={result} />}
    </form>
  );
}

/** An attack on the character: the DM's mode, its one or two rolls, the roll to keep; then the contests decided. */
function IncomingForm() {
  const { t, i18n } = useTranslation();
  const busy = useTableStore((s) => s.busy);
  const modes = useCatalogue("attack_mode");
  const [draft, setDraft] = useState(EMPTY_DEFEND);
  const [result, setResult] = useState<DefendResult | null>(null);
  const [refused, setRefused] = useState<string | null>(null);
  const req = defendRequest(draft);
  const modeName = (id: string) => {
    const m = modes.find((x) => x.id === id);
    return m ? localized(i18n.language, m.name) : id;
  };
  const set = (patch: Partial<typeof draft>) => {
    setDraft((d) => ({ ...d, ...patch }));
    setResult(null);
    setRefused(null);
  };

  async function resolve() {
    if (!req || busy) return;
    setRefused(null);
    const out = await send(async () => (await defend(req)).defence);
    if ("done" in out) setResult(out.done);
    else setRefused(out.refused);
  }

  return (
    <form
      className="flex flex-col gap-2"
      data-testid="defend-form"
      onSubmit={(e) => {
        e.preventDefault();
        void resolve();
      }}
    >
      <div className={row}>
        <Field label={t("powers.defence.attack")}>
          <select
            className={fieldClass}
            value={draft.attack}
            aria-label={t("powers.defence.attack")}
            onChange={(e) => set({ attack: e.target.value })}
          >
            <option value="">{t("powers.defence.chooseAttack")}</option>
            {modes.map((m) => (
              <option key={m.id} value={m.id}>
                {localized(i18n.language, m.name)}
              </option>
            ))}
          </select>
        </Field>
        <label className="flex items-center gap-1.5 text-body">
          <input
            type="checkbox"
            checked={draft.rolls.length === 2}
            onChange={(e) =>
              set({
                rolls: e.target.checked
                  ? [...draft.rolls, { roll: "", success: false }]
                  : draft.rolls.slice(0, 1),
                keep: null,
              })
            }
          />
          {t("powers.defence.twoChecks")}
        </label>
        <Rolls rolls={draft.rolls} onChange={(rolls) => set({ rolls })} />
        <Field label={t("powers.defence.keep")}>
          <select
            className={fieldClass}
            value={draft.keep === null ? "" : String(draft.keep)}
            aria-label={t("powers.defence.keep")}
            onChange={(e) =>
              set({
                keep: e.target.value === "" ? null : Number(e.target.value),
              })
            }
          >
            <option value="">{t("powers.defence.keepNone")}</option>
            {draft.rolls.map((_, i) => (
              <option key={i} value={i}>
                {t("powers.defence.keepRoll", { n: i + 1 })}
              </option>
            ))}
          </select>
        </Field>
        <Button
          type="submit"
          variant="main"
          className="ml-auto"
          disabled={busy || !req}
        >
          {t("powers.tab.bar.resolve")}
        </Button>
      </div>
      <Refusal text={refused} testId="defend-refused" />
      {result && <DefendOutcome result={result} powerName={modeName} />}
    </form>
  );
}
