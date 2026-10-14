import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useBuffsStore } from "@/stores/buffs";
import { useTableStore } from "@/stores/table";
import { previewRounds } from "@/lib/passTime";
import { parseDecimal } from "@/lib/decimalInput";
import { captionClass, fieldClass } from "./fieldStyles";

type Mode = "idle" | "confirmEnd" | "setting" | "passing";

/** Round number + Next Round, End combat (confirmed), Set round and Time passes (hours, previewed). */
export function RoundControl() {
  const { t } = useTranslation();
  const round = useBuffsStore((s) => s.round);
  const { nextRound, endCombat, setRound, passTime, busy } = useTableStore();
  const [mode, setMode] = useState<Mode>("idle");
  const [draft, setDraft] = useState("");
  const [hours, setHours] = useState("");
  const valid = /^\d+$/.test(draft.trim());
  // Display only: the server converts and refuses 0 rounds with its own message.
  const preview = previewRounds(hours);
  return (
    <div
      className="flex items-center gap-3"
      role="group"
      aria-label={t("round.title")}
    >
      <div className="flex flex-col items-center leading-none">
        <span className={captionClass}>{t("round.title")}</span>
        <span
          className="font-mono text-3xl font-black tabular-nums"
          data-testid="round-value"
        >
          {round}
        </span>
      </div>
      <div className="flex flex-col gap-1">
        <Button size="lg" className="h-11 px-4 text-base" disabled={busy} onClick={() => void nextRound()}>
          {t("round.next")}
          <ChevronRight />
        </Button>
        {mode === "idle" && (
          <div className="flex gap-1">
            <Button size="xs" variant="ghost" disabled={busy} onClick={() => setMode("confirmEnd")}>
              {t("round.endCombat")}
            </Button>
            <Button
              size="xs"
              variant="ghost"
              disabled={busy}
              onClick={() => {
                setDraft(String(round));
                setMode("setting");
              }}
            >
              {t("round.set")}
            </Button>
            <Button
              size="xs"
              variant="ghost"
              disabled={busy}
              title={t("round.passTimeHint")}
              onClick={() => {
                setHours("");
                setMode("passing");
              }}
            >
              {t("round.passTime")}
            </Button>
          </div>
        )}
        {mode === "passing" && (
          <form
            className="flex items-center gap-1 text-xs"
            aria-label={t("round.passTime")}
            onSubmit={async (ev) => {
              ev.preventDefault();
              if (preview === null) return;
              if (await passTime(parseDecimal(hours))) setMode("idle");
            }}
          >
            <input
              className={`${fieldClass} !w-16 font-mono tabular-nums`}
              type="text"
              inputMode="decimal"
              placeholder={t("round.passTimePlaceholder")}
              aria-label={t("round.passTimeLabel")}
              title={t("round.passTimeHint")}
              value={hours}
              onChange={(ev) => setHours(ev.target.value)}
              autoFocus
            />
            <span>{t("round.passTimeHours")}</span>
            <span className="font-mono tabular-nums text-muted-foreground" data-testid="pass-time-preview">
              {preview === null
                ? hours.trim() === ""
                  ? ""
                  : t("round.passTimeInvalid")
                : t("round.passTimePreview", { count: preview })}
            </span>
            <Button size="xs" type="submit" disabled={busy || preview === null}>
              {t("round.confirm")}
            </Button>
            <Button size="xs" variant="ghost" type="button" onClick={() => setMode("idle")}>
              {t("round.cancel")}
            </Button>
          </form>
        )}
        {mode === "confirmEnd" && (
          <div className="flex items-center gap-1 text-xs" role="alertdialog" aria-label={t("round.endCombat")}>
            <span>{t("round.endCombatConfirm")}</span>
            <Button
              size="xs"
              variant="destructive"
              disabled={busy}
              onClick={async () => {
                await endCombat();
                setMode("idle");
              }}
            >
              {t("round.confirm")}
            </Button>
            <Button size="xs" variant="ghost" onClick={() => setMode("idle")}>
              {t("round.cancel")}
            </Button>
          </div>
        )}
        {mode === "setting" && (
          <form
            className="flex items-center gap-1"
            onSubmit={async (ev) => {
              ev.preventDefault();
              if (!valid) return;
              if (await setRound(Number(draft))) setMode("idle");
            }}
          >
            <input
              className={`${fieldClass} !w-16 font-mono tabular-nums`}
              type="number"
              min={0}
              step={1}
              aria-label={t("round.setLabel")}
              value={draft}
              onChange={(ev) => setDraft(ev.target.value)}
              autoFocus
            />
            <Button size="xs" type="submit" disabled={busy || !valid}>
              {t("round.apply")}
            </Button>
            <Button size="xs" variant="ghost" type="button" onClick={() => setMode("idle")}>
              {t("round.cancel")}
            </Button>
          </form>
        )}
      </div>
    </div>
  );
}
