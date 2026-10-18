import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { SignOut } from "@phosphor-icons/react";
import { logout } from "@/api/auth";
import { fetchCharacters } from "@/api/characters";
import { BrandIcon } from "@/components/shell/BrandMark";
import { Button } from "@/components/ui/button";
import { APP_NAME } from "@/lib/brand";
import { LANGUAGES } from "@/lib/settings";
import { cn } from "@/lib/utils";
import { useSessionStore } from "@/stores/session";
import { useSettingsStore } from "@/stores/settings";
import { CharacterForm } from "./CharacterForm";
import { CharacterList } from "./CharacterDialogs";

/**
 * The page of a user with no current character (ADR 0029 step 7.2): a new player creates their
 * first one; a player whose current one is gone (retired elsewhere) plays another of theirs or
 * creates one. The cog menu is the dashboard's, so the language and Log out are here too.
 */
export function CreateCharacterPage() {
  const { t } = useTranslation();
  const { lang, update } = useSettingsStore();
  const me = useSessionStore((s) => s.me);
  const [others, setOthers] = useState(false);
  const [leaving, setLeaving] = useState(false);
  useEffect(() => {
    document.title = `${t("characters.newTitle")} · ${APP_NAME}`;
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

  return (
    <main
      className="flex h-dvh items-start justify-center overflow-hidden p-3 sm:items-center"
      data-screen="create-character"
    >
      {/* The page never scrolls (as the sheet's): in a window shorter than the form, the form's card scrolls on its own. */}
      <div className="flex max-h-full min-h-0 w-full max-w-[40rem] flex-col gap-4">
        <div
          className="flex shrink-0 items-center gap-3"
          role="img"
          aria-label={APP_NAME}
        >
          <BrandIcon className="size-10" />
          <span
            aria-hidden
            className="font-display text-hero leading-none font-bold tracking-wide select-none"
          >
            {APP_NAME.slice(0, -1)}
            <span className="text-highlight">{APP_NAME.slice(-1)}</span>
          </span>
        </div>

        {others && (
          <section
            aria-labelledby="create-yours"
            className="zone-card flex max-h-[30dvh] shrink-0 flex-col"
            data-testid="create-yours"
          >
            <div className="card-band px-4 py-2">
              <h2 id="create-yours" className="section-label">
                {t("characters.yours")}
              </h2>
            </div>
            <div className="min-h-0 overflow-y-auto px-4 pt-3 pb-4">
              <CharacterList />
            </div>
          </section>
        )}

        <section
          aria-labelledby="create-title"
          className="stage-card flex min-h-0 flex-col"
          data-testid="create-card"
        >
          <div className="card-band shrink-0 px-4 py-2">
            <h1 id="create-title" className="section-label">
              {t("characters.newTitle")}
            </h1>
          </div>
          <div className="flex min-h-0 flex-col gap-4 overflow-y-auto px-4 pt-3 pb-4">
            <p className="text-body text-ink-muted">
              {others
                ? t("characters.noCurrentLead")
                : t("characters.firstLead")}
            </p>
            <CharacterForm />
          </div>
        </section>

        <div className="flex shrink-0 flex-wrap items-center justify-between gap-2">
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
      </div>
    </main>
  );
}
