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
 * The dice composer under the journal: dice buttons and the text field build a formula, the server
 * parses, rolls and logs it (Roll, Enter), favourites live on the server. Nothing is rolled or
 * evaluated here.
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
      className="flex min-w-0 shrink-0 flex-col gap-2 bg-neu-900/40 px-3 py-3"
      aria-label={t("dice.title")}
      onSubmit={(ev) => {
        ev.preventDefault();
        void s.roll();
      }}
    >
      <ul
        className="flex min-h-7 items-center gap-2 overflow-x-auto"
        aria-label={t("dice.favourites")}
        data-testid="dice-favourites"
      >
        {s.favourites.length === 0 && (
          <li className="text-body text-muted-foreground">
            {t("dice.noFavourites")}
          </li>
        )}
        {s.favourites.map((f, i) => (
          <li
            key={f.id}
            className="flex shrink-0 items-center rounded-md bg-card"
            data-testid="dice-favourite"
          >
            {s.favourites.length > 1 && (
              <button
                type="button"
                className="rounded-l-md px-1 py-1 text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-30"
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
              className="h-7 px-2 text-body tabular-nums outline-none hover:bg-neu-900 focus-visible:ring-3 focus-visible:ring-ring/50"
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
                className="px-1 py-1 text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-30"
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

      <div className="grid min-w-0 grid-cols-7 gap-2">
        {DICE_SIDES.map((sides) => {
          const count = dieCount(s.formula, sides);
          const name = dieName(sides);
          return (
            <div key={sides} className="relative min-w-0">
              <button
                type="button"
                className="h-7 w-full rounded-lg bg-card text-body font-medium tabular-nums outline-none hover:bg-neu-900 focus-visible:ring-3 focus-visible:ring-ring/50"
                aria-label={t("dice.add", { die: name })}
                data-testid={`die-${sides}`}
                onClick={() => s.add(sides)}
                onContextMenu={(ev: MouseEvent) => {
                  ev.preventDefault();
                  s.remove(sides);
                }}
              >
                {name}
              </button>
              {count > 0 && (
                <button
                  type="button"
                  className="absolute -top-1.5 right-0 rounded-md bg-tint px-1 text-label font-semibold tabular-nums text-on-accent outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
                  aria-label={t("dice.count", { count, die: name })}
                  data-testid={`die-badge-${sides}`}
                  onClick={() => s.remove(sides)}
                >
                  {count} −
                </button>
              )}
            </div>
          );
        })}
      </div>

      <div className="flex min-w-0 items-center gap-2">
        <input
          className={cn(
            fieldClass,
            "min-w-0 flex-1 tabular-nums",
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
        <Button
          type="button"
          size="sm"
          variant="ghost"
          disabled={empty}
          onClick={s.clear}
        >
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
      {s.error && (
        <p
          className="text-body text-danger"
          role="alert"
          data-testid="dice-error"
        >
          {t("dice.invalid")}
        </p>
      )}
    </form>
  );
}
