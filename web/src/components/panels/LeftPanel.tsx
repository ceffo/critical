import { useEffect, useState } from "react";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { useHpStore } from "@/stores/hp";
import { useBuffsStore } from "@/stores/buffs";
import { X, Zap } from "lucide-react";

function HpBar({ current, max }: { current: number; max: number }) {
  const pct = max > 0 ? Math.max(0, Math.min(100, (current / max) * 100)) : 0;
  const color =
    pct > 50 ? "bg-green-500" : pct >= 25 ? "bg-yellow-400" : "bg-red-500";
  return (
    <div>
      <div className="flex justify-between text-sm font-mono mb-1">
        <span>
          {current} / {max}
        </span>
        <span>{Math.round(pct)}%</span>
      </div>
      <div className="w-full h-4 bg-muted rounded overflow-hidden">
        <div
          className={`h-full transition-all duration-300 ${color}`}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}

export function LeftPanel() {
  const { currentHp, maxHp, events, fetchHp, appendEvent, undoLastEvent } =
    useHpStore();
  const { buffs, fetchBuffs, activateBuff, dismissBuff, triggerBuff } =
    useBuffsStore();

  const [damageInput, setDamageInput] = useState("");
  const [damageSource, setDamageSource] = useState("");
  const [healInput, setHealInput] = useState("");
  const [healSource, setHealSource] = useState("");
  const [loading, setLoading] = useState(false);

  const [newBuffName, setNewBuffName] = useState("");
  const [newBuffType, setNewBuffType] = useState<"charge" | "duration">(
    "duration",
  );
  const [newBuffValue, setNewBuffValue] = useState(1);
  const [newBuffEffectKey, setNewBuffEffectKey] = useState("weapon_buff");
  const [newBuffEffectValue, setNewBuffEffectValue] = useState(1);

  useEffect(() => {
    fetchHp();
    fetchBuffs();
  }, [fetchHp, fetchBuffs]);

  async function handleAddBuff() {
    if (!newBuffName.trim()) return;
    await activateBuff({
      name: newBuffName.trim(),
      type: newBuffType,
      charges: newBuffType === "charge" ? newBuffValue : 0,
      duration: newBuffType === "duration" ? newBuffValue : 0,
      effects: { [newBuffEffectKey]: newBuffEffectValue },
    });
    setNewBuffName("");
  }

  async function handleDamage() {
    const delta = parseInt(damageInput, 10);
    if (isNaN(delta) || delta <= 0 || !damageSource.trim()) return;
    setLoading(true);
    try {
      await appendEvent(-delta, damageSource.trim());
      setDamageInput("");
      setDamageSource("");
    } finally {
      setLoading(false);
    }
  }

  async function handleHeal() {
    const delta = parseInt(healInput, 10);
    if (isNaN(delta) || delta <= 0 || !healSource.trim()) return;
    setLoading(true);
    try {
      await appendEvent(delta, healSource.trim());
      setHealInput("");
      setHealSource("");
    } finally {
      setLoading(false);
    }
  }

  async function handleUndo() {
    setLoading(true);
    try {
      await undoLastEvent();
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex flex-col gap-3 p-3 overflow-y-auto h-full">
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Hit Points</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <HpBar current={currentHp} max={maxHp} />

          {/* Damage entry */}
          <div className="flex flex-col gap-1">
            <label className="text-xs text-muted-foreground uppercase tracking-wide">
              Damage
            </label>
            <div className="flex gap-1">
              <input
                type="number"
                min="1"
                placeholder="Amount"
                value={damageInput}
                onChange={(e) => setDamageInput(e.target.value)}
                className="w-16 px-2 py-1 text-sm border rounded bg-background"
              />
              <input
                type="text"
                placeholder="Source"
                value={damageSource}
                onChange={(e) => setDamageSource(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleDamage()}
                className="flex-1 px-2 py-1 text-sm border rounded bg-background"
              />
            </div>
            <Button
              size="sm"
              variant="destructive"
              disabled={loading || !damageInput || !damageSource}
              onClick={handleDamage}
            >
              Apply Damage
            </Button>
          </div>

          {/* Heal entry */}
          <div className="flex flex-col gap-1">
            <label className="text-xs text-muted-foreground uppercase tracking-wide">
              Heal
            </label>
            <div className="flex gap-1">
              <input
                type="number"
                min="1"
                placeholder="Amount"
                value={healInput}
                onChange={(e) => setHealInput(e.target.value)}
                className="w-16 px-2 py-1 text-sm border rounded bg-background"
              />
              <input
                type="text"
                placeholder="Source"
                value={healSource}
                onChange={(e) => setHealSource(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleHeal()}
                className="flex-1 px-2 py-1 text-sm border rounded bg-background"
              />
            </div>
            <Button
              size="sm"
              disabled={loading || !healInput || !healSource}
              onClick={handleHeal}
            >
              Apply Heal
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Active Buffs */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Active Buffs</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-2">
          <ul className="flex flex-col gap-2">
            {buffs.map((buff) => (
              <li
                key={buff.id}
                className="flex items-center justify-between group"
              >
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium">{buff.name}</span>
                  <Badge
                    variant="outline"
                    className="text-[10px] h-5 px-1 font-mono"
                  >
                    {buff.type === "charge"
                      ? `${buff.charges}ch`
                      : `${buff.duration}rd`}
                  </Badge>
                </div>
                <div className="flex gap-1">
                  {buff.type === "charge" && (
                    <Button
                      size="icon"
                      variant="ghost"
                      className="h-6 w-6"
                      onClick={() => triggerBuff(buff.id)}
                      title="Use Charge"
                    >
                      <Zap className="h-3 w-3 text-yellow-500" />
                    </Button>
                  )}
                  <Button
                    size="icon"
                    variant="ghost"
                    className="h-6 w-6 opacity-0 group-hover:opacity-100 transition-opacity"
                    onClick={() => dismissBuff(buff.id)}
                    title="Dismiss"
                  >
                    <X className="h-3 w-3 text-destructive" />
                  </Button>
                </div>
              </li>
            ))}
            {buffs.length === 0 && (
              <li className="text-xs text-muted-foreground italic text-center py-2">
                No active buffs
              </li>
            )}
          </ul>

          <div className="flex flex-col gap-2 mt-2 border-t pt-3">
            <div className="flex gap-1">
              <input
                type="text"
                placeholder="Buff Name"
                value={newBuffName}
                onChange={(e) => setNewBuffName(e.target.value)}
                className="flex-1 px-2 py-1 text-sm border rounded bg-background"
              />
              <input
                type="number"
                min="1"
                value={newBuffValue}
                onChange={(e) => setNewBuffValue(parseInt(e.target.value) || 1)}
                className="w-12 px-1 py-1 text-sm border rounded bg-background"
              />
            </div>

            <div className="flex gap-1">
              <select
                value={newBuffType}
                onChange={(e) =>
                  setNewBuffType(e.target.value as "charge" | "duration")
                }
                className="flex-1 px-1 py-1 text-sm border rounded bg-background"
              >
                <option value="duration">Rounds</option>
                <option value="charge">Charges</option>
              </select>
              <select
                value={newBuffEffectKey}
                onChange={(e) => setNewBuffEffectKey(e.target.value)}
                className="flex-1 px-1 py-1 text-sm border rounded bg-background"
              >
                <option value="weapon_buff">Wpn +N</option>
                <option value="spells_buff">Spells xN</option>
                <option value="multiplier">Dmg xN</option>
              </select>
              <input
                type="number"
                step="0.1"
                value={newBuffEffectValue}
                onChange={(e) =>
                  setNewBuffEffectValue(parseFloat(e.target.value) || 0)
                }
                className="w-12 px-1 py-1 text-sm border rounded bg-background"
              />
            </div>

            <Button size="sm" onClick={handleAddBuff} disabled={!newBuffName}>
              Activate
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Recent events */}
      {events.length > 0 && (
        <Card>
          <CardHeader className="pb-2 flex flex-row items-center justify-between space-y-0">
            <CardTitle className="text-base">Recent Events</CardTitle>
            <Button
              variant="ghost"
              size="xs"
              onClick={handleUndo}
              disabled={loading}
              className="h-7 px-2 text-xs"
            >
              Undo
            </Button>
          </CardHeader>
          <CardContent>
            <ul className="flex flex-col gap-1">
              {events.map((e) => (
                <li key={e.id} className="flex justify-between text-sm">
                  <span className="text-muted-foreground truncate max-w-[140px]">
                    {e.source}
                  </span>
                  <span
                    className={e.delta < 0 ? "text-red-500" : "text-green-500"}
                  >
                    {e.delta > 0 ? "+" : ""}
                    {e.delta}
                  </span>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
