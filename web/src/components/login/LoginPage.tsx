import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Code, GoogleLogo, WarningCircle } from "@phosphor-icons/react";
import {
  GOOGLE_START,
  devLogin,
  loginOptions,
  type LoginOptions,
} from "@/api/auth";
import { ApiError } from "@/api/http";
import { BrandIcon } from "@/components/shell/BrandMark";
import { Button, buttonVariants } from "@/components/ui/button";
import { APP_NAME } from "@/lib/brand";
import { LANGUAGES } from "@/lib/settings";
import { loginErrorKey } from "@/lib/session";
import { cn } from "@/lib/utils";
import { useSettingsStore } from "@/stores/settings";

/**
 * The login page (ADR 0029 steps 3 and 6.1), the one page shown without a session: the brand, the
 * refusal the server sent back (`?error=<code>`, worded here, ADR 0023) and the server's login
 * methods, each captioned "Log in" with its icon (the method in its tooltip): Google, and the dev
 * login when the server offers it (no OAuth settings, step 8.1). The page's one main button is the server's login (ADR 0027 step 3.4). The language
 * switch is here too: the cog menu needs a session.
 */
export function LoginPage({ error }: { error: string | null }) {
  const { t } = useTranslation();
  const { lang, update } = useSettingsStore();
  const [options, setOptions] = useState<LoginOptions | null>(null);
  const [devError, setDevError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    void loginOptions().then(setOptions);
  }, []);
  useEffect(() => {
    document.title = `${t("login.title")} · ${APP_NAME}`;
  }, [t]);

  const errorKey = loginErrorKey(error);
  const message = devError ?? (errorKey ? t(errorKey) : null);
  const googleMain = options?.google ?? false;

  const dev = async () => {
    setBusy(true);
    setDevError(null);
    try {
      await devLogin();
    } catch (e) {
      setDevError(e instanceof ApiError ? e.message : t("login.error.unknown"));
      setBusy(false);
    }
  };

  return (
    <main
      className="flex min-h-screen items-center justify-center p-3"
      data-screen="login"
    >
      <section
        aria-labelledby="login-title"
        className="stage-card flex w-full max-w-[28rem] flex-col"
        data-testid="login-card"
      >
        <div className="card-band px-4 py-2">
          <h1 id="login-title" className="section-label">
            {t("login.title")}
          </h1>
        </div>
        <div className="flex flex-col gap-5 px-6 pt-6 pb-5">
          <div
            className="flex items-center gap-3"
            role="img"
            aria-label={APP_NAME}
          >
            <BrandIcon className="size-12" />
            <span
              aria-hidden
              className="font-display text-hero leading-none font-bold tracking-wide select-none"
            >
              {APP_NAME.slice(0, -1)}
              <span className="text-highlight">{APP_NAME.slice(-1)}</span>
            </span>
          </div>
          <p className="text-body text-ink-muted">{t("login.lead")}</p>

          {message && (
            <p
              role="alert"
              data-testid="login-error"
              className="flex items-start gap-2 rounded-md bg-raised px-3 py-2 text-body text-danger"
            >
              <WarningCircle aria-hidden className="mt-px size-4 shrink-0" />
              <span>{message}</span>
            </p>
          )}

          <div className="flex flex-col gap-2">
            {options?.google && (
              <a
                href={GOOGLE_START}
                aria-label={t("login.googleHint")}
                title={t("login.googleHint")}
                className={cn(
                  buttonVariants({ variant: "main", size: "lg" }),
                  "h-10 w-full",
                )}
              >
                <GoogleLogo weight="bold" aria-hidden />
                {t("login.action")}
              </a>
            )}
            {options?.devLogin && (
              <Button
                variant={googleMain ? "quiet" : "main"}
                size="lg"
                className="h-10 w-full"
                aria-label={t("login.devHint")}
                title={t("login.devHint")}
                disabled={busy}
                onClick={() => void dev()}
              >
                <Code aria-hidden />
                {t("login.action")}
              </Button>
            )}
          </div>

          <p className="text-label text-ink-muted">{t("login.inviteOnly")}</p>

          <div
            role="radiogroup"
            aria-label={t("settings.language")}
            className="flex gap-1 self-start"
          >
            {LANGUAGES.map((l) => (
              <button
                key={l}
                type="button"
                role="radio"
                aria-checked={lang === l}
                className={cn(
                  "min-h-7 rounded-md px-2 py-1 text-label outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                  lang === l ? "chip-chosen" : "chip hover:bg-raised-hover",
                )}
                onClick={() => update({ lang: l })}
              >
                {t(`language.${l}`)}
              </button>
            ))}
          </div>
        </div>
      </section>
    </main>
  );
}
