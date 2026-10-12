import { useTranslation } from "react-i18next";
import { LanguageToggle } from "@/components/LanguageToggle";
import { cn } from "@/lib/utils";
import { useBuffsStore } from "@/stores/buffs";
import { useCharacterStore } from "@/stores/character";
import { BuffStrip } from "./BuffStrip";
import { HpBar } from "./HpBar";
import { RoundControl } from "./RoundControl";
import { captionClass } from "./fieldStyles";

const ABILITIES = ["str", "dex", "con", "int", "wis", "cha"] as const;

/** Persistent header: who, how hurt, which round, what's active. */
export function Header() {
  const { t } = useTranslation();
  const character = useCharacterStore((s) => s.character);
  const strength = useBuffsStore((s) => s.strength);
  const strBuffed = strength !== null && strength.effective !== strength.base;

  return (
    <header className="shrink-0 border-b bg-card px-4 py-2">
      <div className="grid grid-cols-[minmax(0,1fr)_minmax(320px,440px)_auto_auto] items-center gap-6">
        <div className="flex min-w-0 items-center gap-5">
          <div className="min-w-0">
            <h1 className="truncate text-xl font-black uppercase tracking-tight">
              {character?.name || t("app.loading")}
            </h1>
            {character && (
              <div className="text-xs text-muted-foreground">
                {t("app.levelValue", { level: character.level })}
              </div>
            )}
          </div>
          {character && (
            <dl
              className="flex items-center gap-3"
              aria-label={t("app.abilities")}
            >
              {ABILITIES.map((k) =>
                k === "str" && strength ? (
                  <div
                    key={k}
                    className="flex flex-col items-center leading-tight"
                    title={t("app.strTitle", { ...strength })}
                  >
                    <dt className={captionClass}>{t(`attr.${k}`)}</dt>
                    <dd
                      className={cn(
                        "font-mono text-base font-bold",
                        strBuffed && "text-green-600 dark:text-green-400",
                      )}
                    >
                      <span data-testid="str-value">{strength.effective}</span>
                      {strBuffed && (
                        <span className="ml-1 text-xs font-normal text-muted-foreground">
                          {t("app.strBase", { base: strength.base })}
                        </span>
                      )}
                    </dd>
                  </div>
                ) : (
                  <div
                    key={k}
                    className="flex flex-col items-center leading-tight"
                  >
                    <dt className={captionClass}>{t(`attr.${k}`)}</dt>
                    <dd className="font-mono text-base font-bold">
                      {character[k]}
                    </dd>
                  </div>
                ),
              )}
              <div className="ml-2 flex flex-col items-center border-l pl-4 leading-tight">
                <dt className={captionClass}>{t("app.thac0")}</dt>
                <dd className="font-mono text-base font-bold">
                  {character.thac0Base}
                </dd>
              </div>
            </dl>
          )}
        </div>
        <HpBar />
        <RoundControl />
        <LanguageToggle />
      </div>
      <div className="mt-2 border-t pt-2">
        <BuffStrip />
      </div>
    </header>
  );
}
