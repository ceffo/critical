import { useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Desktop, GearSix, Moon, Sun } from "@phosphor-icons/react";
import {
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
} from "@/components/ui/popover";
import { LANGUAGES, themePatch } from "@/lib/settings";
import {
  MODES,
  isLight,
  systemPrefersDark,
  themeVars,
  themesFor,
  watchSystemScheme,
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
function Choice({
  label,
  testId,
  mode,
  children,
}: {
  label: string;
  testId?: string;
  mode?: string;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-2" data-testid={testId} data-mode={mode}>
      <span className="text-label text-muted-foreground">{label}</span>
      <div
        role="radiogroup"
        aria-label={label}
        className="flex flex-wrap gap-1"
      >
        {children}
      </div>
    </div>
  );
}

const segment =
  "flex min-h-7 min-w-0 flex-1 items-center justify-center gap-1 rounded-md px-2 py-1 text-body transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50 [&_svg]:size-4";

/** A theme's swatch in its own colours for that mode: its tint on its card, ringed by its page. */
function Swatch({ id, light }: { id: string; light: boolean }) {
  const v = themeVars(id, light);
  return (
    <span
      aria-hidden
      className="size-4 shrink-0 rounded-full"
      style={{
        background: `radial-gradient(circle, ${v["--tint"]} 0 45%, ${v["--surface"]} 50%)`,
        boxShadow: `0 0 0 1.5px ${v["--ground"]}`,
      }}
    />
  );
}

/**
 * Display preferences of this device (ADR 0009, ADR 0026 step 4): language, appearance mode, the
 * dark theme and the light theme. Each choice applies live and is saved; opened from the cog at the
 * left of the dice footer (ADR 0022 step 2.6).
 */
export function SettingsPopover() {
  const { t } = useTranslation();
  const { lang, mode, darkTheme, lightTheme, update } = useSettingsStore();
  // The list follows the appearance in effect; "system" follows the OS live (ADR 0026 step 4.6).
  const [osDark, setOsDark] = useState(systemPrefersDark);
  useEffect(() => watchSystemScheme(() => setOsDark(systemPrefersDark())), []);
  const light = isLight(mode, osDark);
  const shown = light ? lightTheme : darkTheme;
  // The chosen option is the selected tone with a tint outline (ADR 0026 step 5.3).
  const chosen = (on: boolean) =>
    on ? "chip-chosen" : "chip hover:bg-raised-hover";

  return (
    <Popover>
      <PopoverTrigger
        data-testid="settings-trigger"
        aria-label={t("settings.open")}
        title={t("settings.open")}
        className="flex size-8 items-center justify-center rounded-md text-muted-foreground transition-colors outline-none hover:bg-raised-hover hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 data-popup-open:bg-raised-hover data-popup-open:text-foreground [&_svg]:size-5"
      >
        <GearSix />
      </PopoverTrigger>
      <PopoverContent
        side="top"
        align="start"
        className="flex w-80 flex-col gap-3"
        aria-label={t("settings.title")}
        data-testid="settings-popover"
      >
        <PopoverTitle className="section-label">
          {t("settings.title")}
        </PopoverTitle>

        <Choice label={t("settings.language")}>
          {LANGUAGES.map((l) => (
            <button
              key={l}
              type="button"
              role="radio"
              aria-checked={lang === l}
              className={cn(segment, chosen(lang === l))}
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
              className={cn(segment, chosen(mode === m))}
              onClick={() => update({ mode: m })}
            >
              {MODE_ICONS[m]}
              {t(`settings.mode.${m}`)}
            </button>
          ))}
        </Choice>

        <Choice
          label={t("settings.theme")}
          testId="settings-theme"
          mode={light ? "light" : "dark"}
        >
          {themesFor(light).map((id) => (
            <button
              key={id}
              type="button"
              role="radio"
              aria-checked={shown === id}
              className={cn(segment, chosen(shown === id))}
              onClick={() => update(themePatch(light, id))}
            >
              <Swatch id={id} light={light} />
              <span className="break-words">{t(`settings.themes.${id}`)}</span>
            </button>
          ))}
        </Choice>
      </PopoverContent>
    </Popover>
  );
}
