import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { useBuffsStore } from "@/stores/buffs";
import { useCharacterStore } from "@/stores/character";
import { useXpStore } from "@/stores/xp";
import { CharacterDetails } from "./CharacterDetails";

const ABILITIES = ["str", "dex", "con", "int", "wis", "cha"] as const;

/**
 * Who: name, level (a button opening the Expérience workspace, underlined with the server's
 * progress to the next level, ADR 0015), then the abilities in one row of seven cells, STR as the server's effective
 * value (sheet value in brackets when buffed) and THAC0 last, set apart by a tone step.
 */
export function Identity({ onOpenXp }: { onOpenXp: () => void }) {
  const { t, i18n } = useTranslation();
  const xp = useXpStore((s) => s.ledger);
  const character = useCharacterStore((s) => s.character);
  const strength = useBuffsStore((s) => s.strength);
  const strBuffed = strength !== null && strength.effective !== strength.base;

  return (
    <div className="flex min-w-0 flex-col gap-2">
      <div className="min-w-0">
        <h1 className="min-w-0">
          <CharacterDetails character={character} />
        </h1>
        {character && (
          <button
            type="button"
            onClick={onOpenXp}
            title={
              xp?.nextLevel
                ? t("xp.levelLinkTitle", {
                    xp: new Intl.NumberFormat(i18n.language).format(xp.total),
                    percent: xp.progressPercent,
                    level: xp.nextLevel,
                  })
                : undefined
            }
            className="relative pb-1 text-body text-muted-foreground tabular-nums outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            {t("app.levelValue", { level: character.level })}
            {xp && (
              <span
                aria-hidden
                className="absolute inset-x-0 bottom-0 h-0.5 rounded-full bg-neu-800"
              >
                <span
                  className="absolute inset-y-0 left-0 rounded-full bg-tint"
                  style={{ width: `${xp.progress * 100}%` }}
                  data-testid="level-progress"
                />
              </span>
            )}
          </button>
        )}
      </div>
      {character && (
        <dl
          className="grid grid-cols-[repeat(6,minmax(0,1fr))_minmax(0,1.35fr)] gap-1"
          aria-label={t("app.abilities")}
        >
          {ABILITIES.map((k) => {
            const isStr = k === "str" && strength !== null;
            return (
              <div
                key={k}
                className="flex min-w-0 flex-col items-center rounded-md bg-neu-900/60 py-1"
                title={isStr ? t("app.strTitle", { ...strength }) : undefined}
              >
                <dt className="text-label text-muted-foreground">
                  {t(`attr.${k}`)}
                </dt>
                <dd
                  className={cn(
                    "text-key tabular-nums",
                    isStr && strBuffed && "text-success",
                  )}
                >
                  {isStr ? (
                    <span data-testid="str-value">{strength.effective}</span>
                  ) : (
                    character[k]
                  )}
                </dd>
                {isStr && strBuffed && (
                  <dd className="text-label text-muted-foreground tabular-nums">
                    {t("app.strBase", { base: strength.base })}
                  </dd>
                )}
              </div>
            );
          })}
          <div className="flex min-w-0 flex-col items-center rounded-md bg-neu-800 py-1">
            <dt className="text-label text-muted-foreground">
              {t("app.thac0")}
            </dt>
            <dd className="text-key tabular-nums" data-testid="thac0-value">
              {character.thac0Base}
            </dd>
          </div>
        </dl>
      )}
    </div>
  );
}
