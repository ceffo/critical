import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { useCharacterStore } from "@/stores/character";
import { rollSavingThrow } from "@/api/roll";
import type { RollResult } from "@/types/combat";

export function CharacterPanel() {
  const { t } = useTranslation();
  const { character, loadCharacter, loading } = useCharacterStore();
  const [lastResult, setLastResult] = useState<RollResult | null>(null);

  useEffect(() => {
    loadCharacter();
  }, [loadCharacter]);

  if (loading || !character)
    return <div className="p-4">{t("character.loading")}</div>;

  const handleRollSave = async (category: string) => {
    try {
      const res = await rollSavingThrow(category);
      setLastResult(res);
    } catch (e: unknown) {
      alert((e as Error).message);
    }
  };

  const attributes = [
    { key: "str", label: t("attr.str"), value: character.str },
    { key: "dex", label: t("attr.dex"), value: character.dex },
    { key: "con", label: t("attr.con"), value: character.con },
    { key: "int", label: t("attr.int"), value: character.int },
    { key: "wis", label: t("attr.wis"), value: character.wis },
    { key: "cha", label: t("attr.cha"), value: character.cha },
  ];

  const savingThrows = [
    {
      id: "para_poison",
      label: t("character.saves.para_poison"),
      value: character.saveParaPoison,
    },
    {
      id: "rod_staff_wand",
      label: t("character.saves.rod_staff_wand"),
      value: character.saveRodStaffWand,
    },
    {
      id: "petri_poly",
      label: t("character.saves.petri_poly"),
      value: character.savePetriPoly,
    },
    {
      id: "breath",
      label: t("character.saves.breath"),
      value: character.saveBreath,
    },
    {
      id: "spell",
      label: t("character.saves.spell"),
      value: character.saveSpell,
    },
  ];

  return (
    <div className="space-y-4">
      {/* Attributes Grid */}
      <div className="grid grid-cols-3 gap-2">
        {attributes.map((attr) => (
          <Card key={attr.key} className="bg-muted/50">
            <CardContent className="p-2 flex flex-col items-center">
              <span className="text-xs font-bold text-muted-foreground uppercase">
                {attr.label}
              </span>
              <span className="text-2xl font-black">{attr.value}</span>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Saving Throws List */}
      <Card>
        <CardHeader className="py-3">
          <CardTitle className="text-sm">
            {t("character.savingThrows")}
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0 border-t">
          <ul className="divide-y">
            {savingThrows.map((save) => (
              <li
                key={save.id}
                className="flex items-center justify-between p-3 hover:bg-muted/30 transition-colors group"
              >
                <div className="flex flex-col">
                  <span className="text-sm font-medium">{save.label}</span>
                  <span className="text-xs text-muted-foreground italic">
                    {t("character.target", { value: save.value })}
                  </span>
                </div>
                <div className="flex items-center gap-3">
                  <Badge variant="outline" className="font-mono text-lg px-2">
                    {save.value}
                  </Badge>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => handleRollSave(save.id)}
                    className="opacity-0 group-hover:opacity-100 transition-opacity"
                  >
                    {t("character.roll")}
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        </CardContent>
      </Card>

      {/* Roll Result Display */}
      {lastResult && (
        <Card
          className={`border-2 ${lastResult.hit ? "border-green-500 bg-green-50/5" : "border-red-500 bg-red-50/5"}`}
        >
          <CardContent className="p-3">
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs font-bold uppercase text-muted-foreground">
                {t("character.result")}
              </span>
              <Badge
                variant={lastResult.hit ? "default" : "destructive"}
                className="uppercase"
              >
                {lastResult.hit ? t("character.passed") : t("character.failed")}
              </Badge>
            </div>
            <div className="text-center mb-2">
              <div className="text-5xl font-black font-mono tracking-tighter">
                {lastResult.d20Roll}
              </div>
              <div className="text-xs font-medium text-muted-foreground mt-1">
                {t("character.margin", {
                  margin: `${lastResult.hitMargin > 0 ? "+" : ""}${lastResult.hitMargin}`,
                })}
              </div>
            </div>
            <div className="bg-muted p-2 rounded text-[10px] font-mono break-all leading-tight">
              {lastResult.damageBreakdown}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
