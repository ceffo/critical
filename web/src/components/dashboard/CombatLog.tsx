import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useBuffsStore } from "@/stores/buffs";
import { useLogStore } from "@/stores/log";
import { useTableStore } from "@/stores/table";
import { groupByRound } from "@/lib/groupByRound";
import { formatSigned, formatTime } from "@/lib/format";
import { entryText } from "@/lib/logSummary";
import { cn } from "@/lib/utils";
import type { LogEntry } from "@/types/log";
import { captionClass, fieldClass } from "./fieldStyles";

/** The action log (GET /api/log) as a timeline grouped by round, with inline amend/void. */
export function CombatLog() {
  const { t } = useTranslation();
  const entries = useLogStore((s) => s.entries);
  const round = useBuffsStore((s) => s.round);
  const groups = groupByRound(entries, round);

  return (
    <section
      className="flex min-h-0 min-w-0 flex-1 flex-col rounded-xl border bg-card"
      aria-label={t("log.title")}
    >
      <h2 className="shrink-0 border-b px-4 py-2.5 text-sm font-semibold">
        {t("log.title")}
      </h2>
      <div className="min-h-0 flex-1 overflow-y-auto px-3 py-2">
        {groups.map((g) => (
          <section
            key={g.round}
            className="mb-3"
            aria-label={t("log.round", { round: g.round })}
          >
            <h3 className="sticky top-0 z-10 flex items-center gap-2 bg-card py-1">
              <span className={captionClass}>{t("log.round", { round: g.round })}</span>
              {g.round === round && <Badge variant="secondary">{t("log.current")}</Badge>}
            </h3>
            {g.entries.length === 0 ? (
              <p className="px-1 text-xs italic text-muted-foreground">
                {entries.length === 0 ? t("log.empty") : t("log.nothingThisRound")}
              </p>
            ) : (
              <ul className="flex flex-col gap-1">
                {g.entries.map((e) => (
                  <LogRow key={e.id} entry={e} />
                ))}
              </ul>
            )}
          </section>
        ))}
      </div>
    </section>
  );
}

type Mode = "view" | "amend" | "confirmVoid";

function LogRow({ entry: e }: { entry: LogEntry }) {
  const { t, i18n } = useTranslation();
  const { voidEntry, busy } = useTableStore();
  const [mode, setMode] = useState<Mode>("view");
  const voided = e.status === "voided";
  const amended = e.status === "amended";
  const isHp = e.kind === "hp_change";
  const text = entryText(e, t);

  return (
    <li
      className={cn(
        "rounded-lg border px-2.5 py-1.5",
        voided && "bg-muted/50",
      )}
      data-status={e.status}
    >
      <div className="flex items-center gap-2">
        <Badge variant="outline" className="shrink-0">
          {t(`log.kinds.${e.kind}`, { defaultValue: e.kind })}
        </Badge>
        <span
          className={cn(
            "min-w-0 flex-1 truncate text-sm",
            voided && "text-muted-foreground line-through",
          )}
          title={text}
        >
          {text}
        </span>
        {isHp && (
          <span
            className={cn(
              "shrink-0 font-mono text-sm font-semibold tabular-nums",
              voided
                ? "text-muted-foreground line-through"
                : e.effectiveHpDelta < 0
                  ? "text-red-600 dark:text-red-400"
                  : "text-green-600 dark:text-green-400",
            )}
            data-testid="log-delta"
          >
            {formatSigned(voided ? e.hpDelta : e.effectiveHpDelta)}
          </span>
        )}
      </div>
      <div className="mt-0.5 flex items-center gap-2 text-xs text-muted-foreground">
        <span>{formatTime(e.createdAt, i18n.language)}</span>
        {amended && (
          <>
            <Badge variant="secondary">{t("log.status.amended")}</Badge>
            {isHp && <span>{t("log.was", { delta: formatSigned(e.hpDelta) })}</span>}
          </>
        )}
        {voided && <Badge variant="destructive">{t("log.status.voided")}</Badge>}
        {!voided && mode === "view" && (
          <span className="ml-auto flex gap-1">
            {isHp && (
              <Button size="xs" variant="ghost" onClick={() => setMode("amend")}>
                {t("log.amend")}
              </Button>
            )}
            <Button
              size="xs"
              variant="ghost"
              className="text-destructive"
              onClick={() => setMode("confirmVoid")}
            >
              {t("log.void")}
            </Button>
          </span>
        )}
      </div>
      {mode === "confirmVoid" && !voided && (
        <div className="mt-1.5 flex items-center gap-2 rounded-md bg-destructive/5 px-2 py-1 text-sm">
          <span className="flex-1">{t("log.confirmVoid")}</span>
          <Button
            size="xs"
            variant="destructive"
            disabled={busy}
            onClick={async () => {
              await voidEntry(e.id);
              setMode("view");
            }}
          >
            {t("log.confirm")}
          </Button>
          <Button size="xs" variant="ghost" onClick={() => setMode("view")}>
            {t("log.cancel")}
          </Button>
        </div>
      )}
      {mode === "amend" && !voided && (
        <AmendForm entry={e} onDone={() => setMode("view")} />
      )}
    </li>
  );
}

function AmendForm({ entry: e, onDone }: { entry: LogEntry; onDone: () => void }) {
  const { t } = useTranslation();
  const { amendEntry, busy } = useTableStore();
  const [delta, setDelta] = useState(String(e.effectiveHpDelta));
  const [source, setSource] = useState(e.effectiveSummary);
  const n = Number.parseInt(delta, 10);

  return (
    <form
      className="mt-1.5 grid grid-cols-[96px_minmax(0,1fr)] items-end gap-2 rounded-md bg-muted/50 p-2"
      aria-label={t("log.amendLabel")}
      onSubmit={async (ev) => {
        ev.preventDefault();
        if (Number.isNaN(n)) return;
        if (await amendEntry(e.id, n, source.trim())) onDone();
      }}
    >
      <label className="flex min-w-0 flex-col gap-1">
        <span className={captionClass}>{t("log.newDelta")}</span>
        <input
          className={fieldClass}
          type="number"
          value={delta}
          onChange={(ev) => setDelta(ev.target.value)}
          autoFocus
        />
      </label>
      <label className="flex min-w-0 flex-col gap-1">
        <span className={captionClass}>{t("log.source")}</span>
        <input
          className={fieldClass}
          type="text"
          value={source}
          onChange={(ev) => setSource(ev.target.value)}
        />
      </label>
      <div className="col-span-2 flex justify-end gap-1">
        <Button size="xs" type="button" variant="ghost" onClick={onDone}>
          {t("log.cancel")}
        </Button>
        <Button size="xs" type="submit" disabled={busy || Number.isNaN(n)}>
          {t("log.save")}
        </Button>
      </div>
    </form>
  );
}
