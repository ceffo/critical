import { useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { useSpellsDbStore } from "@/stores/spells_db";
import { spellId, spellName } from "@/i18n/spellName";
import { Search, Loader2 } from "lucide-react";

export function LookupPanel() {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const { results, search, loading } = useSpellsDbStore();

  useEffect(() => {
    const timer = setTimeout(() => {
      search(query);
    }, 300);
    return () => clearTimeout(timer);
  }, [query, search]);

  return (
    <div className="flex flex-col h-full space-y-4">
      <div className="relative shrink-0">
        <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
        <Input
          type="search"
          placeholder={t("lookup.search")}
          className="pl-8"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
      </div>

      <div className="flex-1 overflow-y-auto min-h-0 pr-1 space-y-3 scrollbar-thin">
        {loading && results.length === 0 ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        ) : results.length > 0 ? (
          results.map((spell) => (
            <Card key={spell.name} className="overflow-hidden">
              <CardHeader className="py-2 px-3 bg-muted/30">
                <div className="flex items-center justify-between">
                  <CardTitle className="text-sm font-black uppercase tracking-tight">
                    {spellName(t, spellId(spell.name), spell.name)}
                  </CardTitle>
                  <div className="flex gap-1">
                    <Badge variant="secondary" className="text-[10px] py-0">
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
                      {spell.casting_time}
                    </span>
                  </div>
                  <div className="flex gap-1">
                    <span className="font-bold uppercase text-[9px]">
                      {t("lookup.range")}
                    </span>
                    <span className="text-foreground">{spell.range}</span>
                  </div>
                  <div className="flex gap-1">
                    <span className="font-bold uppercase text-[9px]">
                      {t("lookup.duration")}
                    </span>
                    <span className="text-foreground">{spell.duration}</span>
                  </div>
                </div>

                <div className="pt-2 border-t text-muted-foreground leading-relaxed italic">
                  {spell.description.length > 200
                    ? spell.description.substring(0, 200) + "..."
                    : spell.description}
                </div>
              </CardContent>
            </Card>
          ))
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
