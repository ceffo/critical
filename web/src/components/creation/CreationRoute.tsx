import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowRight, SignOut } from "@phosphor-icons/react";
import { logout } from "@/api/auth";
import { fetchCharacters } from "@/api/characters";
import { CharacterList } from "@/components/characters/CharacterDialogs";
import { Button } from "@/components/ui/button";
import { APP_NAME } from "@/lib/brand";
import { STEPS } from "@/lib/creationFlow";
import { LANGUAGES } from "@/lib/settings";
import { navigate } from "@/lib/navigation";
import { cn } from "@/lib/utils";
import { useSessionStore } from "@/stores/session";
import { useSettingsStore } from "@/stores/settings";
import { CreationFlow } from "./CreationFlow";
import { CardHead, StepDot, Wordmark } from "./kit";

/**
 * Every creation goes through one full-screen route (ADR 0034 step 2). A user with no current
 * character first sees the intro (step 1: a new player's first login; one whose current character
 * was retired elsewhere also finds their others there); Commencer opens the flow and Quitter comes
 * back to it. A user who plays a character reaches the flow from the cog menu, and Quitter returns
 * to the game.
 */
export function CreationRoute({ hasCurrent }: { hasCurrent: boolean }) {
  const [started, setStarted] = useState(hasCurrent);
  if (!started) return <Intro onStart={() => setStarted(true)} />;
  return (
    <CreationFlow
      onLeave={() => (hasCurrent ? navigate("/") : setStarted(false))}
    />
  );
}

/** ADR 0034 step 1: one card, what the app does, the steps, language, Log out and Commencer. */
function Intro({ onStart }: { onStart: () => void }) {
  const { t } = useTranslation();
  const { lang, update } = useSettingsStore();
  const me = useSessionStore((s) => s.me);
  const [others, setOthers] = useState(false);
  const [leaving, setLeaving] = useState(false);
  useEffect(() => {
    document.title = `${t("intro.title")} · ${APP_NAME}`;
  }, [t]);
  useEffect(() => {
    let live = true;
    fetchCharacters().then(
      (l) => live && setOthers(l.characters.length > 0),
      () => undefined,
    );
    return () => {
      live = false;
    };
  }, []);
  const logoutHint = me
    ? t("settings.logoutHint", { email: me.email })
    : t("settings.logoutHintNoEmail");

  return (
    <main
      className="flex h-dvh flex-col items-center overflow-y-auto p-3"
      data-screen="intro"
    >
      <div className="my-auto flex w-full max-w-[44rem] flex-col gap-3">
        <section
          className="stage-card flex flex-col"
          aria-labelledby="intro-title"
          data-testid="intro"
        >
          <div className="flex flex-col gap-5 px-8 pt-8 pb-6">
            <Wordmark size="hero" />
            <h1 id="intro-title" className="text-headline">
              {t("intro.title")}
            </h1>
            <p className="text-title font-normal">{t("intro.what")}</p>
            <div className="flex flex-col gap-2">
              <p className="text-body text-ink-muted">
                {t("intro.steps", { count: STEPS.length })}
              </p>
              <ol className="grid grid-cols-2 gap-x-6 gap-y-1.5">
                {STEPS.map((s, i) => (
                  <li key={s} className="flex items-center gap-2 text-body">
                    <StepDot n={i + 1} done={false} on />
                    {t(`creation.${s}.name`)}
                  </li>
                ))}
              </ol>
            </div>
            <p className="text-body text-ink-muted">{t("intro.dm")}</p>
          </div>
          <div className="flex flex-wrap items-center gap-3 border-t border-border px-8 py-4">
            <div
              role="radiogroup"
              aria-label={t("settings.language")}
              className="flex gap-1"
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
            <Button
              variant="quiet"
              aria-label={logoutHint}
              title={logoutHint}
              disabled={leaving}
              onClick={() => {
                setLeaving(true);
                void logout();
              }}
            >
              <SignOut aria-hidden />
              {t("settings.logout")}
            </Button>
            <span className="flex-1" />
            <Button
              variant="main"
              size="lg"
              onClick={onStart}
              aria-label={t("intro.startHint")}
              title={t("intro.startHint")}
            >
              {t("intro.start")}
              <ArrowRight aria-hidden />
            </Button>
          </div>
        </section>
        {others && (
          <section
            aria-labelledby="intro-yours"
            className="zone-card flex flex-col"
            data-testid="create-yours"
          >
            <CardHead>
              <span id="intro-yours">{t("characters.yours")}</span>
            </CardHead>
            <div className="px-4 pt-3 pb-4">
              <p className="mb-3 text-body text-ink-muted">
                {t("intro.others")}
              </p>
              <CharacterList />
            </div>
          </section>
        )}
      </div>
    </main>
  );
}
