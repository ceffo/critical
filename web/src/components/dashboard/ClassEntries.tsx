import { useState } from "react";
import { useTranslation } from "react-i18next";
import { CaretDown, Snowflake, Warning } from "@phosphor-icons/react";
import {
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
} from "@/components/ui/popover";
import { useClassName } from "@/lib/useClassName";
import { useCreationOptions } from "@/lib/useCreationOptions";
import { schoolOf, useSwitchableEntries } from "@/lib/classSwitch";
import type { Character } from "@/types/character";

/**
 * A multi-class or dual-class character's class entries (ADR 0042 steps 5 and 7), under the name:
 * one summary line, "Fighter 5 / Mage 5 / Thief 6", that opens a popover (a button, so Enter and
 * Space work) with one row per entry: the class and its level taken, then its XP over the XP that
 * reaches its next level, all the server's figures (`derived.classes`). A frozen entry (a
 * dual-class's old class) carries its tag and the level it was left at; an entry above the race's
 * level limit carries the server's `level_limit` warning, and the summary line a warning mark. A
 * row opens the Experience activity. Nothing is cut: the summary and the class names wrap.
 * An entry that may switch to a school of magic (ADR 0050 step 15.7) has its name in the line as a
 * link, dotted underline and caret, opening the school picker. The line's popover trigger is then
 * laid under the text and the link is a second button over it, so the two are separate targets.
 */
export function ClassEntries({
  character,
  onOpenXp,
  onOpenSchool,
}: {
  character: Character;
  onOpenXp: () => void;
  onOpenSchool: (entry: number) => void;
}) {
  const { options } = useCreationOptions();
  const switchable = useSwitchableEntries(character);
  const { t, i18n } = useTranslation();
  const className = useClassName();
  const [open, setOpen] = useState(false);
  const n = new Intl.NumberFormat(i18n.language);
  const entries = character.derived.classes ?? [];
  const limitOf = (cls: string) =>
    character.warnings.find((w) => w.code === "level_limit" && w.class === cls);
  // ADR 0050 step 14.1: a specialist of a race the book does not list for the class.
  const raceOf = (cls: string) =>
    character.warnings.find(
      (w) => w.code === "specialist_race" && w.class === cls,
    );
  const summary = entries
    .map((e) =>
      t("app.classEntry.summaryItem", {
        class: className(e.class),
        level: e.level,
      }),
    )
    .join(" / ");
  const school = (cls: string) => {
    const code = schoolOf(options, cls);
    return code === ""
      ? ""
      : t(`lookup.schoolNames.${code}`, { defaultValue: code });
  };
  // The class entries' index in the character's classes is the switch's: derived.classes follows it
  // by class, so the entry is found by its class.
  const indexOf = (cls: string) =>
    character.classes.findIndex((e) => e.class === cls);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <div
        className="relative w-full min-w-0 rounded-md bg-raised px-2.5 py-1 text-body font-medium break-words hover:bg-raised-hover"
        data-testid="class-summary-line"
      >
        <PopoverTrigger
          aria-label={t("app.classEntry.summaryOpen", { summary })}
          data-testid="class-summary"
          className="absolute inset-0 cursor-pointer rounded-md text-left"
        >
          <span className="sr-only">{summary}</span>
        </PopoverTrigger>
        {entries.map((e, i) => {
          const at = indexOf(e.class);
          const link = switchable.includes(at);
          const name = className(e.class);
          const sch = school(e.class);
          return (
            <span key={e.class} className="pointer-events-none relative">
              {link ? (
                <button
                  type="button"
                  onClick={() => onOpenSchool(at)}
                  title={
                    sch === ""
                      ? t("switchClass.linkTitleGeneralist")
                      : t("switchClass.linkTitleSpecialist", { school: sch })
                  }
                  data-testid={`class-school-link-${e.class}`}
                  className="pointer-events-auto inline-flex cursor-pointer items-center gap-0.5 rounded-sm underline decoration-tint decoration-dotted underline-offset-4 hover:text-amb-300"
                >
                  {sch === "" ? name : `${name} · ${sch}`}
                  <CaretDown aria-hidden className="size-3 shrink-0" />
                </button>
              ) : (
                <span>{name}</span>
              )}
              {"\u00a0"}
              {e.level}
              {i < entries.length - 1 && " / "}
            </span>
          );
        })}
        {entries.some((e) => limitOf(e.class) || raceOf(e.class)) && (
          <Warning
            aria-hidden
            className="pointer-events-none relative ml-1.5 inline size-3.5 align-middle text-warning"
          />
        )}
      </div>
      <PopoverContent
        align="start"
        className="w-(--anchor-width) min-w-64 max-w-[calc(100vw-2rem)] p-2"
        data-testid="class-entries"
      >
        <PopoverTitle className="px-2.5 pb-1 text-label font-medium tracking-wide text-muted-foreground uppercase">
          {t("app.classEntry.list")}
        </PopoverTitle>
        <ul className="flex min-w-0 flex-col gap-1">
          {entries.map((e) => {
            const name = className(e.class);
            const frozen = e.state === "frozen";
            const limit = limitOf(e.class);
            return (
              <li
                key={e.class}
                className="flex min-w-0 flex-col gap-0.5 rounded-md bg-raised"
              >
                <button
                  type="button"
                  onClick={() => {
                    setOpen(false);
                    onOpenXp();
                  }}
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
                  className="grid w-full min-w-0 cursor-pointer grid-cols-[minmax(0,1fr)_auto] items-baseline gap-x-2 rounded-md px-2.5 pt-1 text-left hover:bg-raised-hover"
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
                {raceOf(e.class) && (
                  <span
                    className="flex items-center gap-1 px-2.5 text-label text-warning"
                    data-testid={`class-race-${e.class}`}
                  >
                    <Warning aria-hidden className="size-3.5 shrink-0" />
                    <span className="min-w-0 break-words">
                      {t("app.classEntry.specialistRace", {
                        race: t(`details.races.${raceOf(e.class)?.race}`, {
                          defaultValue: raceOf(e.class)?.race ?? "",
                        }),
                      })}
                    </span>
                  </span>
                )}
                <span aria-hidden className="h-1" />
              </li>
            );
          })}
        </ul>
      </PopoverContent>
    </Popover>
  );
}
