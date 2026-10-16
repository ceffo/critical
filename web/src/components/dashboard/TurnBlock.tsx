import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowRight, ArrowsClockwise, DotsThree } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { buffName, useSpellName } from "@/i18n/spellName";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { previewPassTime } from "@/api/round";
import {
  hoursToAsk,
  previewFor,
  type KeyedAnswer,
} from "@/lib/passTimePreview";
import { useBuffsStore } from "@/stores/buffs";
import { useTableStore } from "@/stores/table";
import { fieldClass } from "./fieldStyles";

type Mode = "menu" | "passing" | "setting" | "confirmEnd";

/**
 * The turn block of the header: the turn number, the initiative row (the server's d10 for the
 * turn, with a reroll), Next turn, and a menu with Time passes, Set turn number and End combat.
 * Every action goes through the round endpoints and is logged server-side.
 */
export function TurnBlock() {
  const { t } = useTranslation();
  const turn = useBuffsStore((s) => s.round);
  const initiative = useBuffsStore((s) => s.initiative);
  const { nextRound, rerollInitiative, busy } = useTableStore();
  const [open, setOpen] = useState(false);
  const [mode, setMode] = useState<Mode>("menu");

  return (
    <div
      className="flex shrink-0 items-center gap-3 pr-3"
      role="group"
      aria-label={t("turn.title")}
    >
      <div className="flex items-baseline gap-2">
        <span className="text-body text-muted-foreground">
          {t("turn.title")}
        </span>
        <span
          className="min-w-[2ch] text-headline tabular-nums"
          data-testid="round-value"
        >
          {turn}
        </span>
      </div>
      <div
        className="line-clamp-2 w-44 text-body text-muted-foreground"
        data-testid="initiative-row"
      >
        {turn === 0 ? (
          t("turn.outOfCombatInitiative")
        ) : (
          <span
            className="flex items-center gap-2"
            title={t("turn.initiativeHint")}
          >
            {t("turn.initiative")}
            {initiative ? (
              <>
                <span
                  className="rounded-md bg-neu-900 px-2 font-medium text-foreground tabular-nums"
                  data-testid="initiative-value"
                  data-entry-id={initiative.logEntryId}
                >
                  {initiative.value}
                </span>
                <span>{t("turn.initiativeDie")}</span>
                <button
                  type="button"
                  aria-label={t("turn.initiativeReroll")}
                  title={t("turn.initiativeReroll")}
                  disabled={busy}
                  onClick={() => void rerollInitiative()}
                  className="flex size-6 items-center justify-center rounded-md text-foreground transition-colors outline-none hover:bg-neu-900 focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-50 [&_svg]:size-4"
                >
                  <ArrowsClockwise />
                </button>
              </>
            ) : (
              <span className="rounded-md bg-neu-900 px-2">
                {t("turn.initiativeNone")}
              </span>
            )}
          </span>
        )}
      </div>
      <div className="flex gap-1">
        <Button size="lg" disabled={busy} onClick={() => void nextRound()}>
          {t("turn.next")}
          <ArrowRight />
        </Button>
        <Popover
          open={open}
          onOpenChange={(next) => {
            setOpen(next);
            if (next) setMode("menu");
          }}
        >
          <PopoverTrigger
            aria-label={t("turn.menu")}
            title={t("turn.menu")}
            className="flex size-8 items-center justify-center rounded-lg bg-neu-900 text-foreground transition-colors outline-none hover:bg-neu-800 focus-visible:ring-3 focus-visible:ring-ring/50 [&_svg]:size-4"
          >
            <DotsThree weight="bold" />
          </PopoverTrigger>
          <PopoverContent side="bottom" align="start" className="w-80 p-2">
            {mode === "menu" && <TurnMenu onPick={setMode} />}
            {mode === "passing" && (
              <PassTimeForm onDone={() => setOpen(false)} />
            )}
            {mode === "setting" && (
              <SetTurnForm onDone={() => setOpen(false)} />
            )}
            {mode === "confirmEnd" && (
              <EndCombatConfirm onDone={() => setOpen(false)} />
            )}
          </PopoverContent>
        </Popover>
      </div>
    </div>
  );
}

const menuItem =
  "flex h-7 w-full items-center rounded-md px-2 text-left text-body outline-none hover:bg-neu-900 focus-visible:bg-neu-900";

function TurnMenu({ onPick }: { onPick: (m: Mode) => void }) {
  const { t } = useTranslation();
  return (
    <div role="menu" aria-label={t("turn.menu")} className="flex flex-col">
      <button
        type="button"
        role="menuitem"
        className={menuItem}
        onClick={() => onPick("passing")}
      >
        {t("turn.passTime")}
      </button>
      <button
        type="button"
        role="menuitem"
        className={menuItem}
        onClick={() => onPick("setting")}
      >
        {t("turn.setTurn")}
      </button>
      <button
        type="button"
        role="menuitem"
        className={menuItem}
        onClick={() => onPick("confirmEnd")}
      >
        {t("turn.endCombat")}
      </button>
    </div>
  );
}

/**
 * Hours (decimal, "1.5" or "1,5"). The server answers what they would do: the turns and the
 * effects that would end (GET /api/round/pass-time/preview), or why it refuses them. Only the
 * answer for the hours typed now is shown (`previewFor`); older requests are aborted.
 */
function PassTimeForm({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation();
  const spellName = useSpellName();
  const { passTime, busy } = useTableStore();
  const [hours, setHours] = useState("");
  const [answer, setAnswer] = useState<KeyedAnswer | null>(null);
  const value = hoursToAsk(hours);
  const preview = previewFor(hours, answer);

  useEffect(() => {
    if (value === null) return;
    const ctl = new AbortController();
    const timer = setTimeout(() => {
      previewPassTime(value, { signal: ctl.signal })
        .then((p) => setAnswer({ hours: value, answer: { kind: "ok", ...p } }))
        .catch((e: unknown) => {
          if (!ctl.signal.aborted)
            setAnswer({
              hours: value,
              answer: {
                kind: "error",
                message: e instanceof Error ? e.message : String(e),
              },
            });
        });
    }, 150);
    return () => {
      clearTimeout(timer);
      ctl.abort();
    };
  }, [value]);

  return (
    <form
      className="flex flex-col gap-2 p-1"
      aria-label={t("turn.passTime")}
      onSubmit={async (ev) => {
        ev.preventDefault();
        if (preview.kind !== "ok" || value === null) return;
        if (await passTime(value)) onDone();
      }}
    >
      <span className="text-title">{t("turn.passTime")}</span>
      <div className="flex items-center gap-2 text-body">
        <input
          className={`${fieldClass} !w-20 tabular-nums`}
          type="text"
          inputMode="decimal"
          placeholder={t("turn.passTimePlaceholder")}
          aria-label={t("turn.passTimeLabel")}
          title={t("turn.passTimeHint")}
          value={hours}
          onChange={(ev) => setHours(ev.target.value)}
          autoFocus
        />
        <span>{t("turn.passTimeHours")}</span>
        <span
          className="tabular-nums text-muted-foreground"
          data-testid="pass-time-preview"
        >
          {preview.kind === "ok"
            ? t("turn.passTimePreview", { count: preview.rounds })
            : preview.kind === "invalid"
              ? t("turn.passTimeInvalid")
              : preview.kind === "error"
                ? preview.message
                : ""}
        </span>
      </div>
      {preview.kind === "ok" && (
        <p className="text-body" data-testid="pass-time-ending">
          {preview.expiring.length > 0
            ? t("turn.passTimeEnding", {
                names: preview.expiring
                  .map((b) => buffName(spellName, b))
                  .join(", "),
              })
            : t("turn.passTimeNothingEnds")}
        </p>
      )}
      <p className="text-label text-muted-foreground">
        {t("turn.passTimeHint")}
      </p>
      <div className="flex justify-end gap-1">
        <Button size="sm" variant="ghost" type="button" onClick={onDone}>
          {t("turn.cancel")}
        </Button>
        <Button
          size="sm"
          type="submit"
          disabled={busy || preview.kind !== "ok"}
        >
          {t("turn.confirm")}
        </Button>
      </div>
    </form>
  );
}

function SetTurnForm({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation();
  const turn = useBuffsStore((s) => s.round);
  const { setRound, busy } = useTableStore();
  const [draft, setDraft] = useState(String(turn));
  const valid = /^\d+$/.test(draft.trim());
  return (
    <form
      className="flex flex-col gap-2 p-1"
      aria-label={t("turn.setTurn")}
      onSubmit={async (ev) => {
        ev.preventDefault();
        if (!valid) return;
        if (await setRound(Number(draft))) onDone();
      }}
    >
      <label className="flex items-center justify-between gap-2 text-body">
        {t("turn.setLabel")}
        <input
          className={`${fieldClass} !w-20 tabular-nums`}
          type="number"
          min={0}
          step={1}
          value={draft}
          onChange={(ev) => setDraft(ev.target.value)}
          autoFocus
        />
      </label>
      <div className="flex justify-end gap-1">
        <Button size="sm" variant="ghost" type="button" onClick={onDone}>
          {t("turn.cancel")}
        </Button>
        <Button size="sm" type="submit" disabled={busy || !valid}>
          {t("turn.apply")}
        </Button>
      </div>
    </form>
  );
}

function EndCombatConfirm({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation();
  const { endCombat, busy } = useTableStore();
  return (
    <div
      className="flex flex-col gap-2 p-1"
      role="alertdialog"
      aria-label={t("turn.endCombat")}
    >
      <p className="text-body">{t("turn.endCombatConfirm")}</p>
      <div className="flex justify-end gap-1">
        <Button size="sm" variant="ghost" onClick={onDone}>
          {t("turn.cancel")}
        </Button>
        <Button
          size="sm"
          variant="destructive"
          disabled={busy}
          onClick={async () => {
            if (await endCombat()) onDone();
          }}
        >
          {t("turn.confirm")}
        </Button>
      </div>
    </div>
  );
}
