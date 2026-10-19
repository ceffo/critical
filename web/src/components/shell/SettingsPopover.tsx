import { useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import {
  Desktop,
  GearSix,
  Moon,
  ShieldCheck,
  SignOut,
  Sun,
} from "@phosphor-icons/react";
import { logout } from "@/api/auth";
import { Button, buttonVariants } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
} from "@/components/ui/popover";
import { ADMIN_PATH } from "@/lib/session";
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
import { useSessionStore } from "@/stores/session";
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
  "flex min-h-7 flex-auto items-center justify-center gap-1 rounded-md px-2 py-1 text-body transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50 [&_svg]:size-4";

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
 * left of the dice footer (ADR 0022 step 2.6). Under them the account: the logged-in email and Log
 * out (ADR 0029 step 4.4; account actions live in the cog menu), and for an admin the link to the
 * admin section (step 1.3). No character action: those live on the roster (ADR 0034 steps 7 and 8).
 */
export function SettingsPopover() {
  const { t } = useTranslation();
  const { lang, mode, darkTheme, lightTheme, update } = useSettingsStore();
  const me = useSessionStore((s) => s.me);
  const [leaving, setLeaving] = useState(false);
  const [open, setOpen] = useState(false);
  // The list follows the appearance in effect; "system" follows the OS live (ADR 0026 step 4.6).
  const [osDark, setOsDark] = useState(systemPrefersDark);
  useEffect(() => watchSystemScheme(() => setOsDark(systemPrefersDark())), []);
  const light = isLight(mode, osDark);
  const shown = light ? lightTheme : darkTheme;
  // The chosen option is the selected tone with a tint outline (ADR 0026 step 5.3).
  const chosen = (on: boolean) =>
    on ? "chip-chosen" : "chip hover:bg-raised-hover";

  return (
    <Popover open={open} onOpenChange={setOpen}>
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

        <div className="flex flex-col gap-2" data-testid="settings-account">
          <span className="text-label text-muted-foreground">
            {t("settings.account")}
          </span>
          <div className="flex flex-wrap items-center justify-between gap-2">
            {me && (
              <span className="min-w-0 text-body [overflow-wrap:anywhere]">
                {me.email}
              </span>
            )}
            <Button
              variant="quiet"
              aria-label={
                me
                  ? t("settings.logoutHint", { email: me.email })
                  : t("settings.logoutHintNoEmail")
              }
              title={
                me
                  ? t("settings.logoutHint", { email: me.email })
                  : t("settings.logoutHintNoEmail")
              }
              disabled={leaving}
              onClick={() => {
                setLeaving(true);
                void logout();
              }}
            >
              <SignOut aria-hidden />
              {t("settings.logout")}
            </Button>
          </div>
          {me?.role === "admin" && (
            <a
              href={ADMIN_PATH}
              className={cn(buttonVariants({ variant: "quiet" }), "self-start")}
              aria-label={t("admin.openHint")}
              title={t("admin.openHint")}
            >
              <ShieldCheck aria-hidden />
              {t("admin.open")}
            </a>
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}
