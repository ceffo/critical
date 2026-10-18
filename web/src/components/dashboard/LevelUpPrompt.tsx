import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowFatLinesUp } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { ApiError } from "@/api/http";
import type { LevelUp } from "@/types/character";

/**
 * The level-up prompt of the character card (ADR 0030 step 7): the lowest level whose hit points
 * are owed, one at a time. Up to the name level the server rolls the hit die ("Lancer"), or takes
 * the die rolled at the table ("Saisir", the manual entry); above it the gain is fixed ("Ajouter").
 * Every number comes from the server; the card only words them. When an XP correction brought the
 * level under the max HP's (step 7.4), it warns instead.
 */
export function LevelUpPrompt({
  levelUp,
  level,
  busy,
  onRoll,
}: {
  levelUp: LevelUp;
  /** The level the XP gives (step 3), for the step 7.4 warning. */
  level: number;
  busy: boolean;
  onRoll: (level: number, die?: number) => Promise<ApiError | null>;
}) {
  const { t } = useTranslation();
  const [entering, setEntering] = useState(false);
  const [die, setDie] = useState("");
  const [error, setError] = useState<string | null>(null);
  const next = levelUp.pending[0];

  if (!next) {
    if (!levelUp.hpAboveLevel) return null;
    return (
      <div
        role="status"
        data-testid="level-up-warning"
        className="flex flex-col gap-0.5 rounded-md bg-raised px-2.5 py-1.5 text-body"
      >
        <p className="font-semibold text-warning">
          {t("app.levelUp.hpAbove", { hpLevel: levelUp.hpLevel, level })}
        </p>
        <p className="text-muted-foreground">{t("app.levelUp.hpAboveHint")}</p>
      </div>
    );
  }

  const fixed = next.die === 0;
  const more = levelUp.pending.length - 1;
  const roll = async (entered?: number) => {
    setError(null);
    const rejected = await onRoll(next.level, entered);
    if (rejected) {
      setError(
        t(`app.levelUp.dieErrors.${rejected.message}`, {
          die: next.die,
          defaultValue: rejected.message,
        }),
      );
      return;
    }
    setEntering(false);
    setDie("");
  };
  const enteredDie = Number(die);
  const dieValid =
    die.trim() !== "" &&
    Number.isInteger(enteredDie) &&
    enteredDie >= 1 &&
    enteredDie <= next.die;

  return (
    <div
      role="group"
      aria-label={t("app.levelUp.title", { level: next.level })}
      data-testid="level-up"
      className="flex flex-col gap-1.5 rounded-md bg-raised px-2.5 py-2 text-body"
    >
      <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1.5">
        <div className="flex min-w-0 items-start gap-2">
          <ArrowFatLinesUp
            aria-hidden
            weight="fill"
            className="mt-0.5 size-4 shrink-0 text-tint"
          />
          <div className="flex min-w-0 flex-col">
            <p className="font-semibold" data-testid="level-up-title">
              {t("app.levelUp.title", { level: next.level })}
            </p>
            <p className="text-muted-foreground">
              {fixed
                ? t("app.levelUp.fixedHint", { gain: next.fixedGain })
                : t("app.levelUp.rollHint", { die: next.die })}
            </p>
            {more > 0 && (
              <p className="text-muted-foreground" data-testid="level-up-more">
                {t("app.levelUp.more", { count: more })}
              </p>
            )}
          </div>
        </div>
        {!entering && (
          <div className="flex shrink-0 items-center gap-1.5">
            {!fixed && (
              <Tooltip>
                <TooltipTrigger
                  delay={400}
                  render={
                    <Button
                      variant="quiet"
                      size="sm"
                      disabled={busy}
                      aria-label={t("app.levelUp.enterTitle")}
                      onClick={() => setEntering(true)}
                    />
                  }
                >
                  {t("app.levelUp.enter")}
                </TooltipTrigger>
                <TooltipContent>{t("app.levelUp.enterTitle")}</TooltipContent>
              </Tooltip>
            )}
            <Tooltip>
              <TooltipTrigger
                delay={400}
                render={
                  <Button
                    variant="main"
                    size="sm"
                    disabled={busy}
                    data-testid="level-up-roll"
                    aria-label={
                      fixed
                        ? t("app.levelUp.applyTitle", {
                            gain: next.fixedGain,
                            level: next.level,
                          })
                        : t("app.levelUp.rollTitle", {
                            die: next.die,
                            level: next.level,
                          })
                    }
                    onClick={() => void roll()}
                  />
                }
              >
                {fixed ? t("app.levelUp.apply") : t("app.levelUp.roll")}
              </TooltipTrigger>
              <TooltipContent>
                {fixed
                  ? t("app.levelUp.applyTitle", {
                      gain: next.fixedGain,
                      level: next.level,
                    })
                  : t("app.levelUp.rollTitle", {
                      die: next.die,
                      level: next.level,
                    })}
              </TooltipContent>
            </Tooltip>
          </div>
        )}
      </div>
      {entering && (
        <form
          className="flex flex-wrap items-center gap-1.5"
          onSubmit={(ev) => {
            ev.preventDefault();
            if (dieValid) void roll(enteredDie);
          }}
        >
          <Input
            type="number"
            inputMode="numeric"
            min={1}
            max={next.die}
            value={die}
            autoFocus
            aria-label={t("app.levelUp.dieLabel", { die: next.die })}
            placeholder={`1–${next.die}`}
            aria-invalid={error ? true : undefined}
            onChange={(ev) => setDie(ev.target.value)}
            className="w-20"
            data-testid="level-up-die"
          />
          <Button
            type="submit"
            variant="main"
            size="sm"
            disabled={busy || !dieValid}
          >
            {t("app.levelUp.confirm")}
          </Button>
          <Button
            type="button"
            variant="quiet"
            size="sm"
            onClick={() => {
              setEntering(false);
              setDie("");
              setError(null);
            }}
          >
            {t("app.levelUp.cancel")}
          </Button>
          <span className="text-muted-foreground">
            {t("app.levelUp.dieLabel", { die: next.die })}
          </span>
        </form>
      )}
      {error && (
        <p role="alert" className="text-danger">
          {error}
        </p>
      )}
    </div>
  );
}
