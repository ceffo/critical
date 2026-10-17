import { useTranslation } from "react-i18next";
import { ArrowRight, DiceFive, Plus } from "@phosphor-icons/react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { useDiceStore } from "@/stores/dice";
import { useSpellStore } from "@/stores/spells";
import { useTableStore } from "@/stores/table";
import { localized } from "@/i18n/localized";
import { spellName, spellTexts } from "@/i18n/spellName";
import { SpellDescription } from "@/components/dashboard/SpellDescription";
import { statDice, statValue, type StatKey } from "@/lib/spellValues";
import { statLocale, statText, type StatField } from "@/lib/statText";
import { damageFormula, damageText, exposureDamageText } from "@/lib/spellDice";
import { damageRows, exposureRows } from "@/lib/spellMode";
import { classLabel, schoolLabel, spheresLabel } from "@/lib/lookupLabels";
import type { Spell } from "@/types/spells";

// A spell's detail (ADR 0022 step 2.5): the lookup's right card and the Spells tab's details popup
// (ADR 0027 step 10.4) show the same view.

const STATS: StatKey[] = ["range", "duration", "area", "castingTime", "save"];

/** The catalogue field of a stat of the detail grid. */
const FIELD: Record<StatKey, StatField> = {
  range: "range",
  duration: "duration",
  area: "area_of_effect",
  castingTime: "casting_time",
  save: "saving_throw",
};

/** "1 rd" and "1 rd." say the same: the raw text is not repeated after its value. */
function same(a: string, b: string): boolean {
  const norm = (s: string) => s.toLowerCase().replace(/[^a-z0-9½/-]/g, "");
  return norm(a) === norm(b);
}

/** The spell's name in the UI language; it wraps onto more lines, never cut. */
export function SpellName({
  spell,
  className,
}: {
  spell: Spell;
  className?: string;
}) {
  const { i18n } = useTranslation();
  return (
    <span
      lang={i18n.language}
      className={cn("min-w-0 break-words", className)}
      data-testid="spell-name"
    >
      {spellName(i18n.language, spell)}
    </span>
  );
}

/** Name, meta line, stat tiles, cast box and description of `spell`. */
export function SpellDetail({ spell }: { spell: Spell }) {
  const { t, i18n } = useTranslation();
  const { description } = spellTexts(i18n.language, spell);
  const toComposer = useDiceStore((s) => s.setFormula);
  const roll = useDiceStore((s) => s.roll);
  const rolling = useDiceStore((s) => s.rolling);
  const addSpellEffect = useSpellStore((s) => s.addSpellEffect);
  const run = useTableStore((s) => s.run);
  const busy = useTableStore((s) => s.busy);
  const loc = statLocale(t, i18n.language);
  const value = (key: StatKey): string | null =>
    key === "save"
      ? spell.stats.save.kinds.length
        ? spell.stats.save.kinds
            .map((k) => t(`lookup.saveKinds.${k}`))
            .join(" / ")
        : null
      : statValue(spell, key, loc);
  const meta = [
    classLabel(t, spell.class),
    t("lookup.levelLabel", { level: spell.level }),
    schoolLabel(t, localized(i18n.language, spell.school)),
    spheresLabel(t, localized(i18n.language, spell.sphere)),
    statText(loc, spell, "components").text,
  ].filter(Boolean);
  // The dice row: the damage a cast rolls (one instance to the composer), one row per cast-time
  // mode with damage for a spell with modes, then the stats with dice.
  const damages = damageRows(t, spell);
  const dice: { key: string; label: string; text: string; formula: string }[] =
    [
      ...damages.map((d) => ({
        key: d.key,
        label: d.label,
        text: damageText(t, d.damage),
        formula: damageFormula(d.damage),
      })),
      // One round of exposure to the spell's area (ADR 0008 step 16), per mode when it has modes.
      ...exposureRows(t, spell).map((d) => ({
        key: d.key,
        label: d.label,
        text: exposureDamageText(t, d.damage),
        formula: damageFormula(d.damage),
      })),
      ...statDice(spell, loc).map((d) => ({
        key: d.key,
        label: t(`lookup.${d.key}`),
        text: d.formula,
        formula: d.formula,
      })),
    ];

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <div className="flex flex-col gap-1">
        <h3 className="text-headline">
          <SpellName spell={spell} />
        </h3>
        <p className="text-body text-muted-foreground" data-testid="spell-meta">
          {meta.join(" · ")}
        </p>
      </div>
      {/* The stats as tiles on a raised tone (ADR 0022 steps 2.5 and 4.1): as many per row as fit
          at 7.5rem, so a long value wraps inside its tile instead of being cut. */}
      <dl className="grid shrink-0 grid-cols-[repeat(auto-fit,minmax(7.5rem,1fr))] gap-2">
        {STATS.map((key) => {
          const v = value(key);
          const raw = statText(loc, spell, FIELD[key]).text;
          return (
            <div
              key={key}
              className="flex min-w-0 flex-col gap-0.5 rounded-md bg-raised px-2.5 py-2"
              data-testid={`stat-${key}`}
            >
              <dt className="section-label">{t(`lookup.${key}`)}</dt>
              <dd className="flex min-w-0 flex-col gap-0.5 break-words">
                {v ? (
                  <>
                    <span className="text-key" data-testid="stat-value">
                      {v}
                    </span>
                    {raw && !same(v, raw) && (
                      <span
                        className="text-label text-muted-foreground"
                        data-testid="stat-raw"
                      >
                        {raw}
                      </span>
                    )}
                  </>
                ) : (
                  <span
                    className="text-body font-medium"
                    data-testid="stat-raw"
                  >
                    {raw || "—"}
                  </span>
                )}
              </dd>
            </div>
          );
        })}
      </dl>
      {/* The cast box: what the spell rolls at the Bard's level, each to the composer or rolled, and
          its add-as-effect button; only when it has one of them (ADR 0027 step 13.3). */}
      {spell.caster_level && (dice.length > 0 || spell.effect) ? (
        <section
          aria-label={t("lookup.diceRow")}
          className="flex shrink-0 flex-col gap-1.5 rounded-md bg-raised px-3 py-2.5"
          data-testid="spell-cast-box"
        >
          <p className="section-label">
            {t("lookup.atLevel", { level: spell.caster_level })}
          </p>
          {dice.length > 0 && (
            <ul className="flex flex-col gap-1.5">
              {dice.map((d) => (
                <li
                  key={d.key}
                  className="flex flex-wrap items-center gap-x-3 gap-y-1.5 text-body"
                  data-testid={`spell-dice-${d.key}`}
                >
                  <span className="min-w-40 flex-1">
                    {d.label}{" "}
                    <span
                      className="font-medium text-magic tabular-nums"
                      data-testid="spell-dice-formula"
                    >
                      {d.text}
                    </span>
                  </span>
                  <span className="flex shrink-0 gap-2">
                    <Button
                      size="sm"
                      aria-label={t("dice.send", { formula: d.formula })}
                      title={t("dice.send", { formula: d.formula })}
                      onClick={() => toComposer(d.formula)}
                    >
                      <ArrowRight aria-hidden /> {t("lookup.toDice")}
                    </Button>
                    <Button
                      size="sm"
                      disabled={rolling}
                      aria-label={t("dice.rollFormula", { formula: d.formula })}
                      onClick={() => void roll(d.formula)}
                    >
                      <DiceFive aria-hidden /> {t("lookup.rollDice")}
                    </Button>
                  </span>
                </li>
              ))}
            </ul>
          )}
          {/* A spell that adds a buff goes on the Bard at his level, no slot, no damage (ADR 0004
              step 19); the server says which spells do. */}
          {spell.effect && (
            <Button
              size="sm"
              className="self-start"
              disabled={busy}
              aria-label={t("lookup.addEffectLabel", {
                name: spellName(i18n.language, spell),
                level: spell.caster_level,
              })}
              title={t("lookup.addEffectLabel", {
                name: spellName(i18n.language, spell),
                level: spell.caster_level,
              })}
              data-testid="spell-add-effect"
              onClick={() => void run(() => addSpellEffect(spell.id))}
            >
              <Plus aria-hidden /> {t("buffs.addShort")}
            </Button>
          )}
        </section>
      ) : null}
      {/* The description takes the pane's width and the height left below the stats, in two text
          columns when wide enough, and scrolls inside it. When the pane is too short for its
          floor, the whole detail scrolls instead. */}
      {description && (
        <section
          className="flex min-h-40 flex-1 flex-col gap-1.5"
          data-testid="spell-description"
        >
          <p className="section-label shrink-0">{t("lookup.description")}</p>
          <div
            className="min-h-0 flex-1 overflow-y-auto pr-2 text-body leading-relaxed text-foreground/85"
            data-testid="spell-description-text"
            tabIndex={0}
          >
            <SpellDescription text={description} columns />
          </div>
        </section>
      )}
    </div>
  );
}
