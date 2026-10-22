import { useEffect, useMemo, useState, type KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";
import { Check, PencilSimple, Plus, Trash, X } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { cn } from "@/lib/utils";
import { parseXpAmount } from "@/lib/xpForm";
import { useXpStore } from "@/stores/xp";
import { useCharacterStore } from "@/stores/character";
import { useClassName } from "@/lib/useClassName";
import { splitParts } from "@/lib/xpSplit";
import type { DerivedClass } from "@/types/character";
import { useTableStore } from "@/stores/table";
import type { XpEntry, XpLedger, XpLevel } from "@/types/xp";
import type { ApiError } from "@/api/http";
import { useXpPreview } from "./useXpPreview";

/** Number formatters in the UI language (display only). */
function useFormats() {
  const { i18n } = useTranslation();
  return useMemo(() => {
    const lang = i18n.language;
    const n = new Intl.NumberFormat(lang);
    const compact = new Intl.NumberFormat(lang, {
      notation: "compact",
      maximumFractionDigits: 2,
    });
    const pct = new Intl.NumberFormat(lang, {
      style: "percent",
      maximumFractionDigits: 0,
    });
    const day = new Intl.DateTimeFormat(lang, {
      day: "numeric",
      month: "short",
    });
    return {
      n: (v: number) => n.format(v),
      compact: (v: number) => compact.format(v),
      pct: (whole: number) => pct.format(whole / 100),
      day: (iso: string) => {
        const [y, m, d] = iso.split("-").map(Number);
        return y && m && d ? day.format(new Date(y, m - 1, d)) : iso;
      },
    };
  }, [i18n.language]);
}

/** The top card's two columns: progress, then the 24rem Add XP form. */
const XP_COLS = "grid grid-cols-[minmax(0,1fr)_24rem] gap-x-6";

/**
 * Experience activity (ADR 0015; layout ADR 0022 step 2.5): one card with the progress (the total,
 * the bar towards the next level, the level cells) beside the Add XP form (24rem, 24 px between the
 * two text columns, step 3.2), the +10% bonus defaulting to the server's eligibility and the total
 * previewed by the server; the ledger card below is as tall as its rows, bounded by the rest of the
 * height, its list scrolling on its own past that (ADR 0027 step 14.4), with in-row edit and delete. Every number is the server's. No text is cut (step 1): reasons
 * wrap.
 */
export function XpPanel() {
  const { t } = useTranslation();
  const ledger = useXpStore((s) => s.ledger);
  const refresh = useTableStore((s) => s.refresh);
  const className = useClassName();
  // ADR 0042: a multi-class or dual-class character has one standing per class entry; the ledger's
  // total, level and progress are the first active entry's.
  const entries = useCharacterStore((s) => s.character?.derived.classes);
  const multi = (entries?.length ?? 0) > 1;
  const first = entries?.find((e) => e.state === "active");
  useEffect(() => {
    void refresh(["xp"]);
  }, [refresh]);

  if (!ledger)
    return (
      <p className="text-body text-muted-foreground">{t("app.loading")}</p>
    );
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      {/* The focused card of the Experience activity (ADR 0026 step 6.1): progress beside Add XP,
          their two headings in one header band across the card (step 6.2), over their columns. */}
      <div
        className={cn(
          XP_COLS,
          "stage-card shrink-0 gap-y-3 overflow-hidden px-4 pb-3.5",
        )}
        data-testid="xp-top"
      >
        <div className={cn(XP_COLS, "card-band bleed-x-4 col-span-2 py-2")}>
          <h2 className="section-label">
            {multi && first
              ? t("xp.headingClass", {
                  class: className(first.class),
                  level: ledger.level,
                })
              : t("xp.heading", { level: ledger.level })}
          </h2>
          <h2 className="section-label">{t("xp.add.title")}</h2>
        </div>
        <Progress ledger={ledger} entries={multi ? entries : undefined} />
        <AddForm bonusDefault={ledger.bonusEligible} />
      </div>
      <Ledger ledger={ledger} />
    </div>
  );
}

/**
 * The class entries' standings (ADR 0042 steps 5 and 7), one line each: the class and its level, its
 * XP and the XP before its next level, all the server's. A frozen entry says it earns nothing.
 */
function ClassStandings({ entries }: { entries: DerivedClass[] }) {
  const { t } = useTranslation();
  const f = useFormats();
  const className = useClassName();
  return (
    <section
      aria-label={t("xp.classes.title")}
      className="flex min-w-0 flex-col gap-1"
      data-testid="xp-classes"
    >
      <h3 className="section-label">{t("xp.classes.title")}</h3>
      <ul className="flex flex-col gap-1">
        {entries.map((e) => (
          <li
            key={e.class}
            data-testid={`xp-class-${e.class}`}
            className="grid grid-cols-[minmax(0,1fr)_auto] items-baseline gap-x-3 rounded-md bg-raised px-2.5 py-1 text-body tabular-nums"
          >
            <span className="min-w-0 font-medium break-words">
              {t("xp.classes.level", {
                class: className(e.class),
                level: e.level,
              })}
            </span>
            {e.state === "frozen" ? (
              <span className="text-label text-muted-foreground">
                {t("xp.classes.frozen", { level: e.level })}
              </span>
            ) : (
              <span className="text-right">
                <span data-testid={`xp-class-xp-${e.class}`}>
                  {f.n(e.xp)} {t("xp.unit")}
                </span>
                <span className="block text-label text-muted-foreground">
                  {e.nextLevelXp === null
                    ? t("xp.classes.maxLevel")
                    : t("xp.classes.toNext", {
                        xp: f.n(Math.max(0, e.nextLevelXp - e.xp)),
                        level: e.targetLevel + 1,
                      })}
                </span>
              </span>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}

function Progress({
  ledger: l,
  entries,
}: {
  ledger: XpLedger;
  entries?: DerivedClass[];
}) {
  const { t } = useTranslation();
  const f = useFormats();
  const headline =
    l.nextLevel === null
      ? t("xp.maxLevel")
      : l.toNext > 0
        ? t("xp.toNext", { level: l.nextLevel, xp: f.n(l.toNext) })
        : t("xp.reached", { level: l.nextLevel });
  return (
    <section
      aria-label={t("xp.progressTitle")}
      className="flex min-w-0 flex-col gap-3"
    >
      <div className="flex flex-col gap-1">
        <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
          <span className="flex items-baseline gap-2 tabular-nums">
            {/* The number the zone is about: the one hero size (ADR 0022 step 3.4). */}
            <span className="text-hero" data-testid="xp-total">
              {f.n(l.total)}
            </span>
            <span className="text-body text-muted-foreground">
              {t("xp.unit")}
            </span>
          </span>
          <span
            className="text-body font-medium text-highlight tabular-nums"
            data-testid="xp-to-next"
          >
            {headline}
          </span>
        </div>
      </div>

      <div className="flex flex-col gap-1">
        <div
          role="progressbar"
          aria-label={t("xp.progressLabel", { level: l.nextLevel ?? l.level })}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={l.progressPercent}
          className="relative h-3 overflow-hidden rounded-md bg-raised"
        >
          <div
            className="absolute inset-y-0 left-0 rounded-md bg-tint transition-[width] duration-300"
            style={{ width: `${l.progress * 100}%` }}
          />
          {Array.from({ length: 9 }, (_, i) => (
            <span
              key={i}
              aria-hidden
              className="absolute inset-y-0 w-px bg-surface/60"
              style={{ left: `${(i + 1) * 10}%` }}
            />
          ))}
        </div>
        <div className="flex justify-between text-label text-muted-foreground tabular-nums">
          <span>{f.n(l.currentFloor)}</span>
          <span className="text-foreground" data-testid="xp-percent">
            {f.pct(l.progressPercent)}
          </span>
          <span>{l.nextThreshold === null ? "" : f.n(l.nextThreshold)}</span>
        </div>
      </div>

      <ol
        aria-label={t("xp.levels")}
        className="grid grid-cols-[repeat(auto-fit,minmax(3.5rem,1fr))] gap-2"
      >
        {l.levels.map((c) => (
          <LevelCell key={c.level} cell={c} />
        ))}
      </ol>
      {entries && <ClassStandings entries={entries} />}
    </section>
  );
}

function LevelCell({ cell: c }: { cell: XpLevel }) {
  const { t } = useTranslation();
  const f = useFormats();
  return (
    <li
      title={t("xp.levelTitle", { level: c.level, xp: f.n(c.threshold) })}
      data-status={c.status}
      className={cn(
        "flex flex-col items-center rounded-md px-1 py-1.5 leading-tight tabular-nums",
        // The current level is the chosen chip; the next one has the field edge (ADR 0026 step 5.3).
        c.status === "current" && "chip-chosen",
        c.status === "next" && "bg-raised ring-1 ring-input ring-inset",
        c.status === "past" && "bg-raised text-muted-foreground",
        c.status === "ahead" && "bg-raised",
      )}
    >
      <span className="text-body font-semibold">{c.level}</span>
      <span className="text-label whitespace-nowrap text-muted-foreground">
        {f.compact(c.threshold)}
      </span>
    </li>
  );
}

function FieldMessage({
  error,
  field,
}: {
  error: ApiError | null;
  field: string;
}) {
  if (error?.field !== field) return null;
  return (
    <span role="alert" className="text-label text-danger">
      {error.message}
    </span>
  );
}

function AddForm({ bonusDefault }: { bonusDefault: boolean }) {
  const { t } = useTranslation();
  const f = useFormats();
  const add = useXpStore((s) => s.add);
  const busy = useTableStore((s) => s.busy);
  const [amount, setAmount] = useState("");
  const [reason, setReason] = useState("");
  const [session, setSession] = useState("");
  const [bonus, setBonus] = useState(bonusDefault);
  const [error, setError] = useState<ApiError | null>(null);
  const base = parseXpAmount(amount);
  const preview = useXpPreview(base, bonus);

  const submit = async () => {
    if (busy || base === null) return;
    const rejected = await add({ base, bonus, reason, session });
    setError(rejected);
    if (!rejected) {
      setAmount("");
      setReason("");
      setBonus(bonusDefault);
    }
  };
  const onKey = (e: KeyboardEvent) => {
    if (e.key === "Enter") {
      e.preventDefault();
      void submit();
    }
  };

  return (
    <section
      aria-label={t("xp.add.title")}
      className="flex min-w-0 flex-col gap-2"
    >
      {/* Amount and reason on one line, the reason taking the rest so its placeholder shows in
          full; session, the bonus and Add on the next. */}
      <div className="grid grid-cols-[6rem_minmax(0,1fr)] items-start gap-2">
        <label className="flex min-w-0 flex-col gap-1">
          <input
            className={cn(fieldClass, "h-7 tabular-nums")}
            inputMode="numeric"
            aria-label={t("xp.add.amount")}
            placeholder={t("xp.add.amount")}
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            onKeyDown={onKey}
            aria-invalid={error?.field === "base" || undefined}
          />
          <FieldMessage error={error} field="base" />
        </label>
        <label className="flex min-w-0 flex-col gap-1">
          <input
            className={cn(fieldClass, "h-7")}
            aria-label={t("xp.add.reason")}
            placeholder={t("xp.add.reasonPlaceholder")}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            onKeyDown={onKey}
          />
          <FieldMessage error={error} field="reason" />
        </label>
      </div>
      <div className="flex flex-wrap items-start gap-2">
        <label className="flex w-24 min-w-0 flex-col gap-1">
          <input
            className={cn(fieldClass, "h-7")}
            aria-label={t("xp.add.session")}
            placeholder={t("xp.add.session")}
            value={session}
            onChange={(e) => setSession(e.target.value)}
            onKeyDown={onKey}
          />
          <FieldMessage error={error} field="session" />
        </label>
        <label
          className="flex h-7 cursor-pointer items-center gap-2 text-body"
          title={t("xp.bonusHint")}
        >
          <input
            type="checkbox"
            className="size-4 accent-tint"
            checked={bonus}
            onChange={(e) => setBonus(e.target.checked)}
            onKeyDown={onKey}
          />
          {t("xp.bonus")}
        </label>
        <Button
          className="h-7"
          disabled={busy || base === null || base < 1}
          onClick={() => void submit()}
        >
          <Plus />
          <span className="tabular-nums">
            {preview
              ? t("xp.add.submitWith", { total: f.n(preview.total) })
              : t("xp.add.submit")}
          </span>
        </Button>
      </div>
      {/* ADR 0042 step 5.1: a multi-class character's award shared between its classes. */}
      {preview?.split && (
        <p
          className="text-body text-muted-foreground tabular-nums"
          data-testid="xp-split"
        >
          {t("xp.split.preview", { parts: splitParts(t, preview.split) })}
        </p>
      )}
      {/* The bonus rule in full, without a hover (ADR 0022 step 1.2). */}
      <p className="text-body text-muted-foreground">{t("xp.bonusHint")}</p>
    </section>
  );
}

function Ledger({ ledger }: { ledger: XpLedger }) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState<number | null>(null);
  const cols =
    "grid grid-cols-[minmax(0,1fr)_5.5rem_4.5rem_6rem_4.5rem] items-center gap-2";
  return (
    <section
      aria-label={t("xp.ledger.title")}
      className="zone-card flex min-h-0 flex-col gap-2 pb-3.5"
      data-testid="xp-ledger"
    >
      <div className="card-band flex shrink-0 items-baseline justify-between gap-3 px-4 py-2">
        <h2 className="section-label">{t("xp.ledger.title")}</h2>
        <span className="text-body text-muted-foreground tabular-nums">
          {t("xp.ledger.count", { count: ledger.entries.length })}
        </span>
      </div>
      {/* Column headers are section labels (ADR 0022 step 4.2) on the group heading (ADR 0026 6.7). */}
      <div className={cn(cols, "group-heading section-label shrink-0 px-4")}>
        <span>{t("xp.ledger.reason")}</span>
        <span className="text-right">{t("xp.ledger.base")}</span>
        <span className="text-right">{t("xp.ledger.bonus")}</span>
        <span className="text-right">{t("xp.ledger.total")}</span>
        <span />
      </div>
      <ul className="min-h-0 overflow-y-auto px-2">
        {ledger.entries.length === 0 && (
          <li className="px-2 py-2 text-body text-muted-foreground">
            {t("xp.ledger.none")}
          </li>
        )}
        {ledger.entries.map((e) =>
          editing === e.id ? (
            <EditRow
              key={e.id}
              entry={e}
              cols={cols}
              onDone={() => setEditing(null)}
            />
          ) : (
            <Row
              key={e.id}
              entry={e}
              cols={cols}
              onEdit={() => setEditing(e.id)}
            />
          ),
        )}
        <OpeningRow opening={ledger.opening} cols={cols} />
      </ul>
    </section>
  );
}

function Row({
  entry: e,
  cols,
  onEdit,
}: {
  entry: XpEntry;
  cols: string;
  onEdit: () => void;
}) {
  const { t } = useTranslation();
  const f = useFormats();
  const remove = useXpStore((s) => s.remove);
  const busy = useTableStore((s) => s.busy);
  const fresh = useXpStore((s) => s.freshIds.has(e.id));
  // Delete asks, Confirm deletes (the journal's Undo brings the entry back, ADR 0015 step 4).
  const [confirming, setConfirming] = useState(false);
  const reason = e.reason || t("xp.defaultReason");
  const meta = e.session
    ? t("xp.ledger.meta", { date: f.day(e.date), session: e.session })
    : f.day(e.date);
  return (
    <li
      aria-label={reason}
      data-testid="xp-row"
      className={cn(
        cols,
        "min-h-9 rounded-md px-2 py-1.5 text-body tabular-nums",
        fresh && "bg-selected",
      )}
    >
      {/* The reason wraps between words, never cut (ADR 0022 step 1). */}
      <span className="flex min-w-0 flex-col break-words">
        <span>{reason}</span>
        <span className="text-label text-muted-foreground">{meta}</span>
        {e.split && (
          <span
            className="text-label text-muted-foreground"
            data-testid="xp-row-split"
          >
            {splitParts(t, e.split)}
          </span>
        )}
      </span>
      <span className="text-right">{f.n(e.base)}</span>
      <span
        className={cn(
          "text-right",
          e.bonus ? "text-highlight" : "text-muted-foreground",
        )}
      >
        {e.bonus ? `+${f.n(e.bonusAmount)}` : "–"}
      </span>
      {confirming ? (
        <span
          className="col-span-2 flex items-center justify-end gap-1"
          onKeyDown={(ev) => {
            if (ev.key === "Escape") {
              ev.preventDefault();
              setConfirming(false);
            }
          }}
        >
          <Button
            size="sm"
            variant="destructive"
            disabled={busy}
            autoFocus
            onClick={() => void remove(e.id)}
            aria-label={t("xp.ledger.confirmDelete", { reason })}
          >
            {t("xp.ledger.confirm")}
          </Button>
          <Button
            size="icon-sm"
            variant="ghost"
            onClick={() => setConfirming(false)}
            aria-label={t("xp.ledger.cancel")}
          >
            <X />
          </Button>
        </span>
      ) : (
        <>
          <span className="text-right font-semibold" data-testid="xp-row-total">
            {f.n(e.total)}
          </span>
          <span className="flex justify-end gap-1">
            <Button
              size="icon-sm"
              variant="ghost"
              disabled={busy}
              onClick={onEdit}
              aria-label={t("xp.ledger.edit", { reason })}
            >
              <PencilSimple />
            </Button>
            <Button
              size="icon-sm"
              variant="ghost"
              disabled={busy}
              onClick={() => setConfirming(true)}
              aria-label={t("xp.ledger.delete", { reason })}
            >
              <Trash />
            </Button>
          </span>
        </>
      )}
    </li>
  );
}

function EditRow({
  entry: e,
  cols,
  onDone,
}: {
  entry: XpEntry;
  cols: string;
  onDone: () => void;
}) {
  const { t } = useTranslation();
  const f = useFormats();
  const update = useXpStore((s) => s.update);
  const busy = useTableStore((s) => s.busy);
  const [reason, setReason] = useState(e.reason);
  const [amount, setAmount] = useState(String(e.base));
  const [bonus, setBonus] = useState(e.bonus);
  const [error, setError] = useState<ApiError | null>(null);
  const base = parseXpAmount(amount);
  const preview = useXpPreview(base, bonus);

  const save = async () => {
    if (busy || base === null) return;
    const rejected = await update(e.id, { reason, base, bonus });
    setError(rejected);
    if (!rejected) onDone();
  };
  const onKey = (ev: KeyboardEvent) => {
    if (ev.key === "Enter") {
      ev.preventDefault();
      void save();
    } else if (ev.key === "Escape") {
      ev.preventDefault();
      onDone();
    }
  };
  return (
    <li
      aria-label={t("xp.ledger.editing")}
      className={cn(
        cols,
        "rounded-md bg-raised px-2 py-1.5 text-body tabular-nums",
      )}
    >
      <span className="flex min-w-0 flex-col gap-1">
        <input
          className={fieldClass}
          aria-label={t("xp.add.reason")}
          value={reason}
          onChange={(ev) => setReason(ev.target.value)}
          onKeyDown={onKey}
          autoFocus
        />
        <FieldMessage error={error} field="reason" />
      </span>
      <span className="flex min-w-0 flex-col gap-1">
        <input
          className={cn(fieldClass, "text-right tabular-nums")}
          inputMode="numeric"
          aria-label={t("xp.add.amount")}
          value={amount}
          onChange={(ev) => setAmount(ev.target.value)}
          onKeyDown={onKey}
        />
        <FieldMessage error={error} field="base" />
      </span>
      <label
        className="flex cursor-pointer items-center justify-end gap-1 text-label"
        title={t("xp.bonusHint")}
      >
        <input
          type="checkbox"
          className="size-4 accent-tint"
          checked={bonus}
          onChange={(ev) => setBonus(ev.target.checked)}
          onKeyDown={onKey}
          aria-label={t("xp.bonus")}
        />
      </label>
      <span className="text-right font-semibold" data-testid="xp-edit-total">
        {preview ? f.n(preview.total) : "–"}
      </span>
      <span className="flex justify-end gap-1">
        <Button
          size="icon-sm"
          variant="ghost"
          disabled={busy || base === null}
          onClick={() => void save()}
          aria-label={t("xp.ledger.save")}
        >
          <Check />
        </Button>
        <Button
          size="icon-sm"
          variant="ghost"
          onClick={onDone}
          aria-label={t("xp.ledger.cancel")}
        >
          <X />
        </Button>
      </span>
    </li>
  );
}

/** The sheet's starting XP, the base the ledger adds to (not an entry: no edit or delete). */
function OpeningRow({ opening, cols }: { opening: number; cols: string }) {
  const { t } = useTranslation();
  const f = useFormats();
  return (
    <li
      className={cn(
        cols,
        "px-2 py-1.5 text-body text-muted-foreground tabular-nums",
      )}
      data-testid="xp-opening"
    >
      <span className="col-span-3">{t("xp.ledger.opening")}</span>
      <span className="text-right">{f.n(opening)}</span>
      <span />
    </li>
  );
}
