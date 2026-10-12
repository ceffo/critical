import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useCombatStore } from "@/stores/combat";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

const WEAPON_IDS = ["longsword", "shortbow"];

export function CombatPanel() {
  const { t } = useTranslation();
  const { lastResult, isRolling, rollAttack, error } = useCombatStore();
  const [weaponId, setWeaponId] = useState("longsword");
  const [targetAc, setTargetAc] = useState(5);

  const handleRoll = () => {
    rollAttack(weaponId, targetAc);
  };

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label htmlFor="weapon">{t("combat.weapon")}</Label>
          <Select
            value={weaponId}
            onValueChange={(val) => setWeaponId(val || "")}
          >
            <SelectTrigger id="weapon">
              <SelectValue placeholder={t("combat.selectWeapon")} />
            </SelectTrigger>
            <SelectContent>
              {WEAPON_IDS.map((id) => (
                <SelectItem key={id} value={id}>
                  {t(`combat.weapons.${id}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label htmlFor="targetAc">{t("combat.targetAc")}</Label>
          <Input
            id="targetAc"
            type="number"
            value={targetAc}
            onChange={(e) => setTargetAc(parseInt(e.target.value) || 0)}
          />
        </div>
      </div>

      <Button className="w-full" onClick={handleRoll} disabled={isRolling}>
        {isRolling ? t("combat.rolling") : t("combat.rollAttack")}
      </Button>

      {error && (
        <div className="p-2 text-sm text-destructive bg-destructive/10 rounded">
          {error}
        </div>
      )}

      {lastResult && (
        <Card
          className={
            lastResult.hit
              ? "border-green-500/50 bg-green-500/5"
              : "border-red-500/50 bg-red-500/5"
          }
        >
          <CardHeader className="pb-2">
            <div className="flex justify-between items-center">
              <CardTitle className="text-2xl font-bold">
                {lastResult.hit ? (
                  <span className="text-green-600 dark:text-green-400">
                    {t("combat.hit")}
                  </span>
                ) : (
                  <span className="text-red-600 dark:text-red-400">
                    {t("combat.miss")}
                  </span>
                )}
                {lastResult.isCrit && (
                  <Badge variant="destructive" className="ml-2 animate-bounce">
                    {t("combat.crit")}
                  </Badge>
                )}
              </CardTitle>
              <div className="text-right">
                <div className="text-xs text-muted-foreground uppercase tracking-wider">
                  {t("combat.totalDamage")}
                </div>
                <div className="text-3xl font-black">
                  {lastResult.hit ? lastResult.totalDamage : 0}
                </div>
              </div>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-2 gap-2 text-sm">
              <div className="bg-muted/50 p-2 rounded">
                <div className="text-xs text-muted-foreground">
                  {t("combat.d20Roll")}
                </div>
                <div className="font-mono text-lg">{lastResult.d20Roll}</div>
              </div>
              <div className="bg-muted/50 p-2 rounded">
                <div className="text-xs text-muted-foreground">
                  {t("combat.critRoll")}
                </div>
                <div className="font-mono text-lg">
                  {lastResult.d100CritRoll}
                </div>
              </div>
            </div>

            <div className="space-y-1">
              <div className="text-xs font-semibold uppercase text-muted-foreground">
                {t("combat.attackBreakdown")}
              </div>
              <div className="text-sm font-mono bg-background/50 p-2 rounded border">
                {t("combat.thac0")} {lastResult.baseThac0}
                {lastResult.modifiers.map((m, i) => (
                  <span key={i} className="text-blue-500">
                    {" "}
                    - {m.value} ({m.source})
                  </span>
                ))}
                {" = "}
                <span className="font-bold">{lastResult.effectiveThac0}</span>
                <br />
                {t("combat.targetNeed", {
                  ac: targetAc,
                  need: lastResult.effectiveThac0 - targetAc,
                })}
              </div>
            </div>

            {lastResult.hit && (
              <div className="space-y-1">
                <div className="text-xs font-semibold uppercase text-muted-foreground">
                  {t("combat.damageBreakdown")}
                </div>
                <div className="text-sm font-mono bg-background/50 p-2 rounded border">
                  {lastResult.damageBreakdown}
                  <div className="mt-1 text-[10px] text-muted-foreground">
                    {t("combat.damageDetail", {
                      base: lastResult.baseDamage,
                      spells: lastResult.spellsBuff,
                      weapon: lastResult.weaponBuff,
                      mult: lastResult.multipliers,
                      str: lastResult.strengthBonus,
                    })}
                  </div>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
