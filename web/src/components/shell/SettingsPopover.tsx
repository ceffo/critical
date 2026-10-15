import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Desktop, GearSix, Moon, Sun } from "@phosphor-icons/react";
import { Popover, PopoverContent, PopoverTitle, PopoverTrigger } from "@/components/ui/popover";
import { LANGUAGES } from "@/lib/settings";
import {
  AMBIANCES,
  MODES,
  isLight,
  systemPrefersDark,
  themeVars,
  type Mode,
} from "@/lib/theme";
import { cn } from "@/lib/utils";
import { useSettingsStore } from "@/stores/settings";

const MODE_ICONS: Record<Mode, ReactNode> = {
  dark: <Moon aria-hidden />,
  light: <Sun aria-hidden />,
  system: <Desktop aria-hidden />,
};

/** One row of the popover: a caption and a set of choices (radio semantics). */
function Choice({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-2">
      <span className="text-sm text-muted-foreground">{label}</span>
      <div role="radiogroup" aria-label={label} className="flex flex-wrap gap-1.5">
        {children}
      </div>
    </div>
  );
}

const segment =
  "flex h-9 min-w-0 flex-1 items-center justify-center gap-1.5 rounded-md px-2 text-sm transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50 [&_svg]:size-4";

/**
 * Display preferences of this device (ADR 0009): language, appearance and ambiance. Each choice
 * applies live and is saved; opened from the gear at the bottom of the rail.
 */
export function SettingsPopover() {
  const { t } = useTranslation();
  const { lang, mode, amb, update } = useSettingsStore();
  // Each swatch shows its ambiance's own tint in the current appearance.
  const light = isLight(mode, systemPrefersDark());

  return (
    <Popover>
      <PopoverTrigger
        data-testid="settings-trigger"
        aria-label={t("settings.open")}
        title={t("settings.open")}
        className="flex size-11 items-center justify-center rounded-md text-muted-foreground transition-colors outline-none hover:bg-neu-900 hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 data-popup-open:bg-neu-900 data-popup-open:text-foreground [&_svg]:size-6"
      >
        <GearSix />
      </PopoverTrigger>
      <PopoverContent
        side="right"
        align="end"
        className="flex w-80 flex-col gap-5"
        aria-label={t("settings.title")}
        data-testid="settings-popover"
      >
        <PopoverTitle className="text-base font-semibold">{t("settings.title")}</PopoverTitle>

        <Choice label={t("settings.language")}>
          {LANGUAGES.map((l) => (
            <button
              key={l}
              type="button"
              role="radio"
              aria-checked={lang === l}
              className={cn(
                segment,
                lang === l ? "bg-amb-900 text-amb-200" : "bg-neu-900/60 hover:bg-neu-900",
              )}
              onClick={() => update({ lang: l })}
            >
              {t(`language.${l}`)}
            </button>
          ))}
        </Choice>

        <Choice label={t("settings.appearance")}>
          {MODES.map((m) => (
            <button
              key={m}
              type="button"
              role="radio"
              aria-checked={mode === m}
              className={cn(
                segment,
                mode === m ? "bg-amb-900 text-amb-200" : "bg-neu-900/60 hover:bg-neu-900",
              )}
              onClick={() => update({ mode: m })}
            >
              {MODE_ICONS[m]}
              {t(`settings.mode.${m}`)}
            </button>
          ))}
        </Choice>

        <Choice label={t("settings.ambiance")}>
          <div className="grid w-full grid-cols-2 gap-1.5">
            {AMBIANCES.map((a) => (
              <button
                key={a.id}
                type="button"
                role="radio"
                aria-checked={amb === a.id}
                className={cn(
                  "flex h-10 items-center gap-2 rounded-md px-2 text-sm transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                  amb === a.id ? "bg-amb-900 text-amb-200" : "bg-neu-900/60 hover:bg-neu-900",
                )}
                onClick={() => update({ amb: a.id })}
              >
                <span
                  aria-hidden
                  className="size-4 shrink-0 rounded-full"
                  style={{ background: themeVars(a.id, light)["--tint"] }}
                />
                <span className="truncate">{t(`settings.ambiances.${a.id}`)}</span>
              </button>
            ))}
          </div>
        </Choice>
      </PopoverContent>
    </Popover>
  );
}
