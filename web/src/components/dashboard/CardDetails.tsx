import { useTranslation } from "react-i18next";
import { DetailRows } from "@/components/dashboard/DetailRows";
import { SAVE_FIELD } from "@/lib/saveFields";
import { SAVE_CATEGORIES } from "@/types/combat";
import { groupHeadClass } from "@/components/dashboard/fieldStyles";
import { useCharacterStore } from "@/stores/character";

/**
 * The card's lower half: the details and languages the details popover also shows, inline, so the
 * card's empty space carries something a player reads at a glance. Only what is set shows; the
 * popover stays the way to edit them. It takes the room the card has left under the saves and scrolls inside itself (ADR 0017
 * step 7), so the saves above keep their width; the -mt-6 and the section's pt-6 swap the card's
 * gap for padding, so a card with no room left loses nothing to it. Under 900 px of height the
 * region keeps a floor and the card itself scrolls, so the details stay reachable.
 */
export function CardDetails() {
  const { t } = useTranslation();
  const character = useCharacterStore((s) => s.character);
  if (!character) return null;
  // The saves' modifiers: the race's bonus on a roll, per category, as the server derives it; the
  // categories without one are left out.
  const bonuses = SAVE_CATEGORIES.map((cat) => ({
    cat,
    value: character.derived.raceSaveBonus?.[SAVE_FIELD[cat]] ?? 0,
  })).filter((b) => b.value !== 0);
  return (
    <div
      className="-mt-6 min-h-0 min-w-0 overflow-y-auto [@media(max-height:899px)]:min-h-24"
      data-testid="card-details"
    >
      <section
        className="flex min-w-0 flex-col gap-2 pt-6"
        aria-label={t("details.title")}
      >
        <h2 className={`${groupHeadClass} section-label`}>
          {t("details.title")}
        </h2>
        <DetailRows character={character} />
        {bonuses.length > 0 && (
          <dl>
            <div
              className="grid grid-cols-[minmax(4.5rem,6rem)_minmax(0,1fr)] gap-2 text-body"
              data-testid="save-bonuses"
            >
              <dt className="text-muted-foreground">{t("saves.bonusLabel")}</dt>
              <dd className="min-w-0">
                <ul className="flex flex-col">
                  {bonuses.map((b) => (
                    <li key={b.cat} className="break-words tabular-nums">
                      {t("saves.bonus", {
                        name: t(`saves.categories.${b.cat}`),
                        value: b.value,
                      })}
                    </li>
                  ))}
                </ul>
              </dd>
            </div>
          </dl>
        )}
      </section>
    </div>
  );
}
