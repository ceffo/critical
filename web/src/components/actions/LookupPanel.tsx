import { useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useSpellsDbStore } from "@/stores/spells_db";
import { NO_FILTERS, type SpellFilters } from "@/api/spells_db";
import { spellId, spellName } from "@/i18n/spellName";
import type { Spell, SpellValue } from "@/types/spells";
import { Search, Loader2 } from "lucide-react";

const CLASSES = [
  ["", "classAll"],
  ["wizard", "classWizard"],
  ["priest", "classPriest"],
] as const;
const SAVES = [
  ["none", "saveNone"],
  ["neg", "saveNeg"],
  ["1/2", "saveHalf"],
  ["special", "saveSpecial"],
] as const;
const CASTING_TIMES = [
  "segment",
  "round",
  "turn",
  "hour",
  "day",
  "week",
  "special",
] as const;
const COMPONENTS = ["V", "S", "M"] as const;

const selectClass =
  "h-8 w-full rounded-md border border-input bg-background px-2 text-xs";

/** "210 yard", "3 round + 1d4"; display only, the numbers come from the server. */
function formatValue(v: SpellValue | undefined, raw: string): string {
  if (!v) return raw;
  const parts: string[] = [];
  if (v.amount !== 0 || !v.dice) {
    parts.push(v.upper ? `${v.amount}-${v.upper}` : `${v.amount}`);
  }
  if (v.dice) parts.push(v.dice);
  return parts.join(" + ") + (v.unit ? ` ${v.unit}` : "");
}

function formatArea(spell: Spell): string {
  const a = spell.values?.area;
  if (!a) return spell.area_of_effect;
  const dims = a.dims?.map((d) => formatValue(d, "")).join(" × ");
  return (
    dims || (a.targets ? formatValue(a.targets, "") : spell.area_of_effect)
  );
}

export function LookupPanel() {
  const { t } = useTranslation();
  const [f, setF] = useState<SpellFilters>(NO_FILTERS);
  const { results, search, loading } = useSpellsDbStore();
  const patch = (p: Partial<SpellFilters>) => setF((cur) => ({ ...cur, ...p }));
  const filtered =
    JSON.stringify({ ...f, q: "" }) !==
    JSON.stringify({ ...NO_FILTERS, q: "" });
  const query = f.q;

  useEffect(() => {
    const timer = setTimeout(() => {
      search(f);
    }, 300);
    return () => clearTimeout(timer);
  }, [f, search]);

  return (
    <div className="flex flex-col h-full space-y-3">
      <div className="relative shrink-0">
        <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
        <Input
          type="search"
          placeholder={t("lookup.search")}
          aria-label={t("lookup.search")}
          className="pl-8"
          value={f.q}
          onChange={(e) => patch({ q: e.target.value })}
        />
      </div>

      <div
        className="shrink-0 space-y-2"
        role="group"
        aria-label={t("lookup.classLabel")}
      >
        <div className="flex gap-1">
          {CLASSES.map(([value, key]) => (
            <Button
              key={key}
              type="button"
              size="sm"
              variant={f.cls === value ? "default" : "outline"}
              aria-pressed={f.cls === value}
              className="flex-1 h-7 text-[11px] px-1"
              onClick={() => patch({ cls: value })}
            >
              {t(`lookup.${key}`)}
            </Button>
          ))}
        </div>
        <div className="grid grid-cols-2 gap-2">
          <select
            aria-label={t("lookup.level")}
            className={selectClass}
            value={f.level}
            onChange={(e) => patch({ level: e.target.value })}
          >
            <option value="">{t("lookup.anyLevel")}</option>
            {[0, 1, 2, 3, 4, 5, 6, 7, 8, 9].map((n) => (
              <option key={n} value={n}>
                {t("lookup.levelShort", { level: n })}
              </option>
            ))}
          </select>
          <Input
            aria-label={t("lookup.school")}
            placeholder={t("lookup.schoolPlaceholder")}
            className="h-8 text-xs"
            value={f.school}
            onChange={(e) => patch({ school: e.target.value })}
          />
          <select
            aria-label={t("lookup.save")}
            className={selectClass}
            value={f.save}
            onChange={(e) => patch({ save: e.target.value })}
          >
            <option value="">{t("lookup.anySave")}</option>
            {SAVES.map(([value, key]) => (
              <option key={value} value={value}>
                {t(`lookup.${key}`)}
              </option>
            ))}
          </select>
          <select
            aria-label={t("lookup.castingTime")}
            className={selectClass}
            value={f.castingTime}
            onChange={(e) => patch({ castingTime: e.target.value })}
          >
            <option value="">{t("lookup.anyTime")}</option>
            {CASTING_TIMES.map((u) => (
              <option key={u} value={u}>
                {t(`lookup.${u}`)}
              </option>
            ))}
          </select>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-[10px] font-bold uppercase text-muted-foreground">
            {t("lookup.components")}
          </span>
          {COMPONENTS.map((c) => {
            const on = f.components.includes(c);
            return (
              <Button
                key={c}
                type="button"
                size="sm"
                variant={on ? "default" : "outline"}
                aria-pressed={on}
                aria-label={`${t("lookup.components")} ${c}`}
                className="h-6 w-7 px-0 text-[11px]"
                onClick={() =>
                  patch({
                    components: on
                      ? f.components.filter((x) => x !== c)
                      : [...f.components, c],
                  })
                }
              >
                {c}
              </Button>
            );
          })}
          <label className="ml-auto flex items-center gap-1 text-[10px] font-bold uppercase text-muted-foreground">
            {t("lookup.casterLevel")}
            <Input
              type="number"
              min={1}
              aria-label={t("lookup.casterLevel")}
              placeholder={t("lookup.casterLevelHint")}
              className="h-6 w-32 text-[11px] normal-case"
              value={f.casterLevel}
              onChange={(e) => patch({ casterLevel: e.target.value })}
            />
          </label>
        </div>
        {(filtered || f.q) && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-6 text-[11px]"
            onClick={() => setF(NO_FILTERS)}
          >
            {t("lookup.reset")}
          </Button>
        )}
      </div>

      <div className="flex-1 overflow-y-auto min-h-0 pr-1 space-y-3 scrollbar-thin">
        {loading && results.length === 0 ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        ) : results.length > 0 ? (
          results.map((spell) => (
            <Card
              key={`${spell.class}:${spell.name}`}
              className="overflow-hidden"
              data-testid="spell-result"
              data-class={spell.class}
            >
              <CardHeader className="py-2 px-3 bg-muted/30">
                <div className="flex items-center justify-between">
                  <CardTitle className="text-sm font-black uppercase tracking-tight">
                    {spellName(t, spellId(spell.name), spell.name)}
                  </CardTitle>
                  <div className="flex gap-1">
                    <Badge
                      variant="secondary"
                      className="text-[10px] py-0"
                      data-testid="spell-class"
                    >
                      {spell.class}
                    </Badge>
                    <Badge className="text-[10px] py-0">
                      {t("lookup.levelShort", { level: spell.level })}
                    </Badge>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="p-3 text-[11px] space-y-2">
                <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-muted-foreground">
                  <div className="flex gap-1">
                    <span className="font-bold uppercase text-[9px]">
                      {t("lookup.school")}
                    </span>
                    <span className="text-foreground truncate">
                      {spell.school}
                    </span>
                  </div>
                  <div className="flex gap-1">
                    <span className="font-bold uppercase text-[9px]">
                      {t("lookup.time")}
                    </span>
                    <span className="text-foreground">
                      <span data-testid="v-time">
                        {formatValue(
                          spell.values?.casting_time,
                          spell.casting_time,
                        )}
                      </span>
                    </span>
                  </div>
                  <div className="flex gap-1">
                    <span className="font-bold uppercase text-[9px]">
                      {t("lookup.range")}
                    </span>
                    <span className="text-foreground" data-testid="v-range">
                      {formatValue(spell.values?.range, spell.range)}
                    </span>
                  </div>
                  <div className="flex gap-1">
                    <span className="font-bold uppercase text-[9px]">
                      {t("lookup.duration")}
                    </span>
                    <span className="text-foreground" data-testid="v-duration">
                      {formatValue(spell.values?.duration, spell.duration)}
                    </span>
                  </div>
                  {spell.area_of_effect && (
                    <div className="flex gap-1 col-span-2">
                      <span className="font-bold uppercase text-[9px]">
                        {t("lookup.area")}
                      </span>
                      <span className="text-foreground" data-testid="v-area">
                        {formatArea(spell)}
                      </span>
                    </div>
                  )}
                </div>
                {spell.caster_level ? (
                  <div className="text-[10px] text-muted-foreground">
                    {t("lookup.atLevel", { level: spell.caster_level })}
                  </div>
                ) : null}
                <details className="text-[10px] text-muted-foreground">
                  <summary className="cursor-pointer font-bold uppercase">
                    {t("lookup.phbText")}
                  </summary>
                  <dl className="mt-1 grid grid-cols-[auto_1fr] gap-x-2">
                    <dt>{t("lookup.time")}</dt>
                    <dd>{spell.casting_time}</dd>
                    <dt>{t("lookup.range")}</dt>
                    <dd>{spell.range}</dd>
                    <dt>{t("lookup.duration")}</dt>
                    <dd>{spell.duration}</dd>
                    <dt>{t("lookup.area")}</dt>
                    <dd>{spell.area_of_effect}</dd>
                    <dt>{t("lookup.componentsRaw")}</dt>
                    <dd>{spell.components}</dd>
                    <dt>{t("lookup.saveRaw")}</dt>
                    <dd>{spell.saving_throw}</dd>
                  </dl>
                </details>

                <div className="pt-2 border-t text-muted-foreground leading-relaxed italic">
                  {spell.description.length > 200
                    ? spell.description.substring(0, 200) + "..."
                    : spell.description}
                </div>
              </CardContent>
            </Card>
          ))
        ) : filtered ? (
          <div className="text-center py-8 text-muted-foreground text-sm italic">
            {t("lookup.filtersEmptyPrompt")}
          </div>
        ) : query.length > 1 ? (
          <div className="text-center py-8 text-muted-foreground text-sm italic">
            {t("lookup.noResults", { query })}
          </div>
        ) : (
          <div className="text-center py-8 text-muted-foreground text-sm italic">
            {t("lookup.prompt")}
          </div>
        )}
      </div>
    </div>
  );
}
