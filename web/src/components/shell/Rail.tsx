import type { ComponentType } from "react";
import { useTranslation } from "react-i18next";
import {
  Backpack,
  BookOpenText,
  Brain,
  HandFist,
  TrendUp,
  type IconProps,
} from "@phosphor-icons/react";
import { cn } from "@/lib/utils";
import { ACTIVITIES, type Activity } from "@/lib/activities";
import { useCharacterStore } from "@/stores/character";

const ICONS: Record<Activity, ComponentType<IconProps>> = {
  act: HandFist,
  spells: BookOpenText,
  powers: Brain,
  inventory: Backpack,
  xp: TrendUp,
};

/** The activities the rail lists: the powers one only for a psionicist. */
export function railActivities(psionic: boolean): Activity[] {
  return ACTIVITIES.filter((a) => a !== "powers" || psionic);
}

/**
 * The rail (ADR 0022 step 2.4), on the ground between the character card and the stage: bare icons
 * with their caption under them, the focused activity on a tint pill. A click focuses that
 * activity. Captions wrap rather than being cut.
 */
export function Rail({
  active,
  onSelect,
}: {
  active: Activity;
  onSelect: (a: Activity) => void;
}) {
  const { t } = useTranslation();
  const psionic = useCharacterStore((s) => !!s.character?.derived?.knownPowers);
  return (
    <nav
      aria-label={t("shell.activities")}
      className="flex w-[4.25rem] shrink-0 flex-col items-stretch gap-1"
      data-testid="rail"
    >
      {railActivities(psionic).map((a) => {
        const Icon = ICONS[a];
        const on = active === a;
        return (
          <button
            key={a}
            type="button"
            aria-current={on ? "page" : undefined}
            className={cn(
              "flex flex-col items-center gap-1 rounded-lg px-0.5 py-2.5 text-center text-label break-words transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50 [&_svg]:size-5",
              on
                ? "selected font-medium"
                : "text-muted-foreground hover:bg-raised-hover hover:text-foreground",
            )}
            onClick={() => onSelect(a)}
          >
            <Icon weight={on ? "fill" : "regular"} aria-hidden />
            <span className="max-w-full">{t(`shell.activity.${a}`)}</span>
          </button>
        );
      })}
    </nav>
  );
}
