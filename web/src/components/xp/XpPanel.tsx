import { useEffect, useMemo, useState, type KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";
import { Check, PencilSimple, Plus, Trash, X } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { cn } from "@/lib/utils";
import { parseXpAmount } from "@/lib/xpForm";
import { useXpStore } from "@/stores/xp";
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

/**
 * Expérience workspace (ADR 0015): the total and the progress towards the next level, a form to
 * add XP with the +10% bonus (default from the server's eligibility, total previewed by the
 * server) and the ledger with in-row edit and delete. Every number is the server's.
 */
export function XpPanel() {
  const { t } = useTranslation();
  const ledger = useXpStore((s) => s.ledger);
  const fetchXp = useXpStore((s) => s.fetchXp);
  useEffect(() => {
    void fetchXp();
  }, [fetchXp]);

  if (!ledger)
    return (
      <p className="text-body text-muted-foreground">{t("app.loading")}</p>
    );
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <ProgressCard ledger={ledger} />
      <AddForm bonusDefault={ledger.bonusEligible} />
      <Ledger ledger={ledger} />
    </div>
  );
}

function ProgressCard({ ledger: l }: { ledger: XpLedger }) {
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
      className="flex shrink-0 flex-col gap-3 rounded-xl bg-card p-3"
    >
      <div className="flex flex-wrap items-end justify-between gap-x-3 gap-y-1">
        <div className="flex min-w-0 flex-col gap-1">
          <span className="text-body text-muted-foreground">
            {t("xp.heading", { level: l.level })}
          </span>
          <span className="flex items-baseline gap-2 tabular-nums">
            <span className="text-headline" data-testid="xp-total">
              {f.n(l.total)}
            </span>
            <span className="text-body text-muted-foreground">
              {t("xp.unit")}
            </span>
          </span>
        </div>
        <span
          className="text-body font-medium text-amb-300 tabular-nums"
          data-testid="xp-to-next"
        >
          {headline}
        </span>
      </div>

      <div className="flex flex-col gap-1">
        <div
          role="progressbar"
          aria-label={t("xp.progressLabel", { level: l.nextLevel ?? l.level })}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={l.progressPercent}
          className="relative h-3 overflow-hidden rounded-md bg-neu-800"
        >
          <div
            className="absolute inset-y-0 left-0 rounded-md bg-tint transition-[width] duration-300"
            style={{ width: `${l.progress * 100}%` }}
          />
          {Array.from({ length: 9 }, (_, i) => (
            <span
              key={i}
              aria-hidden
              className="absolute inset-y-0 w-px bg-background/60"
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
        className="grid gap-2"
        style={{
          gridTemplateColumns: `repeat(${Math.max(1, l.levels.length)}, minmax(0, 1fr))`,
        }}
      >
        {l.levels.map((c) => (
          <LevelCell key={c.level} cell={c} />
        ))}
      </ol>
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
        "flex min-w-0 flex-col items-center rounded-md py-1.5 leading-tight tabular-nums",
        c.status === "current" && "bg-amb-900 ring-1 ring-amb-600 ring-inset",
        c.status === "next" && "bg-neu-900 ring-1 ring-neu-600 ring-inset",
        c.status === "past" && "bg-neu-900/60 text-muted-foreground",
        c.status === "ahead" && "bg-neu-900/60",
      )}
    >
      <span className="text-body font-semibold">{c.level}</span>
      <span className="max-w-full truncate text-label text-muted-foreground">
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
  const busy = useXpStore((s) => s.busy);
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
      className="flex shrink-0 flex-col gap-2 rounded-xl bg-card p-3"
    >
      <h2 className="text-title">{t("xp.add.title")}</h2>
      <div className="flex flex-wrap items-start gap-2">
        <label className="flex w-28 min-w-0 flex-col gap-1">
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
        <label className="flex min-w-48 flex-1 flex-col gap-1">
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
        <label className="flex w-28 min-w-0 flex-col gap-1">
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
          className="flex h-7 cursor-pointer items-center gap-2 text-body whitespace-nowrap"
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
    </section>
  );
}

function Ledger({ ledger }: { ledger: XpLedger }) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState<number | null>(null);
  const cols =
    "grid grid-cols-[minmax(0,1fr)_5.5rem_4rem_6rem_4.5rem] items-center gap-2";
  return (
    <section
      aria-label={t("xp.ledger.title")}
      className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl bg-card"
    >
      <div className="flex shrink-0 items-center justify-between bg-neu-900/50 px-3 py-2.5">
        <h2 className="text-title">{t("xp.ledger.title")}</h2>
        <span className="text-label text-muted-foreground tabular-nums">
          {t("xp.ledger.count", { count: ledger.entries.length })}
        </span>
      </div>
      <div
        className={cn(
          cols,
          "shrink-0 px-3 pt-2 pb-1 text-label text-muted-foreground",
        )}
      >
        <span>{t("xp.ledger.reason")}</span>
        <span className="text-right">{t("xp.ledger.base")}</span>
        <span className="text-right">{t("xp.ledger.bonus")}</span>
        <span className="text-right">{t("xp.ledger.total")}</span>
        <span />
      </div>
      <ul className="min-h-0 flex-1 overflow-y-auto px-2 pb-2">
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
  const busy = useXpStore((s) => s.busy);
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
        "rounded-md px-2 py-1.5 text-body tabular-nums",
        fresh && "bg-amb-900/50",
      )}
    >
      <span className="flex min-w-0 flex-col">
        <span className="truncate">{reason}</span>
        <span className="truncate text-label text-muted-foreground">
          {meta}
        </span>
      </span>
      <span className="text-right">{f.n(e.base)}</span>
      <span
        className={cn(
          "text-right",
          e.bonus ? "text-amb-300" : "text-muted-foreground",
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
  const busy = useXpStore((s) => s.busy);
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
        "rounded-md bg-neu-900 px-2 py-1.5 text-body tabular-nums",
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
      <span className="col-span-3 truncate">{t("xp.ledger.opening")}</span>
      <span className="text-right">{f.n(opening)}</span>
      <span />
    </li>
  );
}
