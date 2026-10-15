import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { useBuffsStore } from "@/stores/buffs";
import { useCharacterStore } from "@/stores/character";
import { captionClass } from "./fieldStyles";

const ABILITIES = ["str", "dex", "con", "int", "wis", "cha"] as const;

/** Who: name, level, abilities (STR as the server's effective value) and THAC0. */
export function Identity() {
  const { t } = useTranslation();
  const character = useCharacterStore((s) => s.character);
  const strength = useBuffsStore((s) => s.strength);
  const strBuffed = strength !== null && strength.effective !== strength.base;

  return (
    <div className="flex min-w-0 flex-col gap-3">
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
          className="flex flex-wrap items-center gap-x-3 gap-y-2"
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
          <div className="flex flex-col items-center leading-tight">
            <dt className={captionClass}>{t("app.thac0")}</dt>
            <dd className="font-mono text-base font-bold">
              {character.thac0Base}
            </dd>
          </div>
        </dl>
      )}
    </div>
  );
}
