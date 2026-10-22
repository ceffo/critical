import { useTranslation } from "react-i18next";
import { Snowflake, Warning } from "@phosphor-icons/react";
import { useClassName } from "@/lib/useClassName";
import type { Character } from "@/types/character";

/**
 * One line per class entry of a multi-class or dual-class character (ADR 0042 steps 5 and 7), under
 * the name: the class and its level taken, then its XP over the XP that reaches its next level, all
 * the server's figures (`derived.classes`). A frozen entry (a dual-class's old class) carries its
 * tag and the level it was left at; an entry above the race's level limit carries the server's
 * `level_limit` warning. A line opens the Experience activity. Nothing is cut: the class names wrap.
 */
export function ClassEntries({
  character,
  onOpenXp,
}: {
  character: Character;
  onOpenXp: () => void;
}) {
  const { t, i18n } = useTranslation();
  const className = useClassName();
  const n = new Intl.NumberFormat(i18n.language);
  return (
    <ul
      aria-label={t("app.classEntry.list")}
      className="flex min-w-0 flex-col gap-1"
      data-testid="class-entries"
    >
      {(character.derived.classes ?? []).map((e) => {
        const name = className(e.class);
        const frozen = e.state === "frozen";
        const limit = character.warnings.find(
          (w) => w.code === "level_limit" && w.class === e.class,
        );
        return (
          <li
            key={e.class}
            className="flex min-w-0 flex-col gap-0.5 rounded-md bg-raised"
          >
            <button
              type="button"
              onClick={onOpenXp}
              title={
                frozen
                  ? t("app.classEntry.frozenTitle", {
                      class: name,
                      level: e.level,
                    })
                  : undefined
              }
              aria-label={t("app.classEntry.open", {
                class: name,
                level: e.level,
              })}
              data-testid={`class-entry-${e.class}`}
              data-state={e.state}
              className="grid w-full min-w-0 cursor-pointer grid-cols-[minmax(0,1fr)_auto] items-baseline gap-x-2 rounded-md px-2.5 pt-1 text-left outline-none hover:bg-raised-hover focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              <span className="flex min-w-0 flex-wrap items-center gap-x-2 text-body font-medium break-words">
                {name}
                {frozen && (
                  <span
                    className="inline-flex items-center gap-1 text-label font-normal text-muted-foreground"
                    data-testid={`class-frozen-${e.class}`}
                  >
                    <Snowflake aria-hidden className="size-3.5" />
                    {t("app.classEntry.frozen")}
                  </span>
                )}
              </span>
              <span
                className="text-body text-muted-foreground tabular-nums"
                data-testid={`class-level-${e.class}`}
              >
                {t("app.levelValue", { level: e.level })}
              </span>
            </button>
            {!frozen && (
              <span
                className="px-2.5 text-label text-muted-foreground tabular-nums"
                data-testid={`class-xp-${e.class}`}
              >
                {e.nextLevelXp === null
                  ? t("app.classEntry.xpMax", { xp: n.format(e.xp) })
                  : t("app.classEntry.xp", {
                      xp: n.format(e.xp),
                      next: n.format(e.nextLevelXp),
                    })}
              </span>
            )}
            {limit && (
              <span
                className="flex items-center gap-1 px-2.5 text-label text-warning"
                data-testid={`class-limit-${e.class}`}
              >
                <Warning aria-hidden className="size-3.5 shrink-0" />
                <span className="min-w-0 break-words">
                  {t("app.classEntry.limit", { max: limit.max })}
                </span>
              </span>
            )}
            <span aria-hidden className="h-1" />
          </li>
        );
      })}
    </ul>
  );
}
