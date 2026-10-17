import { useEffect, type MouseEvent } from "react";
import { useTranslation } from "react-i18next";
import {
  CaretLeft,
  CaretRight,
  Minus,
  Plus,
  Star,
  X,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useDiceStore } from "@/stores/dice";
import { DICE_SIDES, dieCount, dieName } from "@/lib/diceFormula";
import { cn } from "@/lib/utils";
import { fieldClass } from "./fieldStyles";

/**
 * The dice composer, one line in the dice footer card (ADR 0022 step 2.6): the dice, the formula,
 * −1 / +1, Roll, then the favourites strip, which scrolls sideways when it outgrows the line (each
 * chip stays whole, step 1.2). Dice buttons and the text field build a formula; the server parses,
 * rolls and logs it (Roll, Enter); favourites live on the server. Nothing is rolled or evaluated
 * here.
 */
export function DiceComposer() {
  const { t } = useTranslation();
  const s = useDiceStore();
  const { fetchFavourites } = s;
  useEffect(() => {
    void fetchFavourites();
  }, [fetchFavourites]);

  const empty = s.formula.trim() === "";
  return (
    <form
      className="flex min-w-0 flex-1 items-center gap-2"
      aria-label={t("dice.title")}
      onSubmit={(ev) => {
        ev.preventDefault();
        void s.roll();
      }}
    >
      <span className="section-label shrink-0">{t("dice.title")}</span>
      <div className="flex shrink-0 gap-1">
        {DICE_SIDES.map((sides) => {
          const count = dieCount(s.formula, sides);
          const name = dieName(sides);
          return (
            <button
              key={sides}
              type="button"
              className={cn(
                "flex h-7 min-w-10 items-center justify-center gap-1 px-2 text-body font-medium tabular-nums outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                count > 0 ? "chip-chosen" : "chip hover:bg-raised-hover",
              )}
              aria-label={t("dice.add", { die: name })}
              title={
                count > 0 ? t("dice.count", { count, die: name }) : undefined
              }
              data-testid={`die-${sides}`}
              onClick={() => s.add(sides)}
              onContextMenu={(ev: MouseEvent) => {
                ev.preventDefault();
                s.remove(sides);
              }}
            >
              {name}
              {count > 0 && (
                <span
                  className="font-semibold"
                  data-testid={`die-badge-${sides}`}
                >
                  ×{count}
                </span>
              )}
            </button>
          );
        })}
      </div>

      <div className="flex shrink-0 items-center gap-1">
        <input
          className={cn(
            fieldClass,
            "!w-40 tabular-nums",
            s.error && "border-danger text-danger",
          )}
          type="text"
          aria-label={t("dice.formula")}
          placeholder={t("dice.placeholder")}
          aria-invalid={s.error !== null}
          title={s.error ?? undefined}
          maxLength={200}
          autoComplete="off"
          spellCheck={false}
          value={s.formula}
          onChange={(ev) => s.setFormula(ev.target.value)}
        />
        <Button
          type="button"
          size="icon-sm"
          variant="ghost"
          aria-label={t("dice.minus")}
          onClick={() => s.step(-1)}
        >
          <Minus aria-hidden />
        </Button>
        <Button
          type="button"
          size="icon-sm"
          variant="ghost"
          aria-label={t("dice.plus")}
          onClick={() => s.step(1)}
        >
          <Plus aria-hidden />
        </Button>
        <Button type="button" size="sm" disabled={empty} onClick={s.clear}>
          {t("dice.clear")}
        </Button>
        <Button
          type="button"
          size="icon-sm"
          variant="ghost"
          disabled={empty}
          aria-label={t("dice.saveFavourite")}
          title={t("dice.saveFavourite")}
          onClick={() => void s.saveFavourite()}
        >
          <Star aria-hidden />
        </Button>
        <Button type="submit" size="sm" disabled={empty || s.rolling}>
          {t("dice.roll")}
        </Button>
      </div>
      <ul
        className="flex min-h-7 min-w-0 flex-1 items-center gap-2 overflow-x-auto"
        data-scroll-x
        aria-label={t("dice.favourites")}
        data-testid="dice-favourites"
      >
        {s.favourites.length === 0 && (
          <li className="text-label text-muted-foreground">
            {t("dice.noFavourites")}
          </li>
        )}
        {s.favourites.map((f, i) => (
          <li
            key={f.id}
            className="chip flex shrink-0 items-center"
            data-testid="dice-favourite"
          >
            {s.favourites.length > 1 && (
              <button
                type="button"
                className="rounded-l-md px-1 py-1 text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 disabled:invisible"
                aria-label={t("dice.moveFavouriteEarlier", {
                  formula: f.formula,
                })}
                disabled={i === 0}
                onClick={() => void s.moveFavourite(f.id, -1)}
              >
                <CaretLeft aria-hidden className="size-3" />
              </button>
            )}
            <button
              type="button"
              className="h-7 px-2 text-body tabular-nums outline-none hover:text-tint focus-visible:ring-3 focus-visible:ring-ring/50"
              title={t("dice.rollFavourite", { formula: f.formula })}
              aria-label={t("dice.rollFavourite", { formula: f.formula })}
              disabled={s.rolling}
              onClick={() => void s.roll(f.formula)}
            >
              {f.formula}
            </button>
            {s.favourites.length > 1 && (
              <button
                type="button"
                className="px-1 py-1 text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 disabled:invisible"
                aria-label={t("dice.moveFavouriteLater", {
                  formula: f.formula,
                })}
                disabled={i === s.favourites.length - 1}
                onClick={() => void s.moveFavourite(f.id, 1)}
              >
                <CaretRight aria-hidden className="size-3" />
              </button>
            )}
            <button
              type="button"
              className="rounded-r-md px-1 py-1 text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
              aria-label={t("dice.removeFavourite", { formula: f.formula })}
              onClick={() => void s.removeFavourite(f.id)}
            >
              <X aria-hidden className="size-3" />
            </button>
          </li>
        ))}
      </ul>
      {s.error && (
        <p
          className="shrink-0 text-body text-danger"
          role="alert"
          data-testid="dice-error"
        >
          {t("dice.invalid")}
        </p>
      )}
    </form>
  );
}
