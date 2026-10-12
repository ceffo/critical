import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus, X, Zap } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useBuffsStore } from "@/stores/buffs";
import { useTableStore } from "@/stores/table";
import { captionClass, fieldClass } from "./fieldStyles";

const EFFECT_KEYS = ["weapon_buff", "spells_buff", "multiplier"] as const;
const EFFECT_LABEL = {
  weapon_buff: "buffs.effectWeapon",
  spells_buff: "buffs.effectSpells",
  multiplier: "buffs.effectMultiplier",
} as const;

/** Active buffs as badges, plus a compact form to add one. (Buff rules and logging: critical-qki.8.) */
export function BuffStrip() {
  const { t } = useTranslation();
  const { buffs, triggerBuff, dismissBuff } = useBuffsStore();
  const run = useTableStore((s) => s.run);
  const [adding, setAdding] = useState(false);

  return (
    <div
      className="flex min-h-9 flex-wrap items-center gap-2"
      role="group"
      aria-label={t("buffs.title")}
    >
      <span className={captionClass}>{t("buffs.title")}</span>
      <ul className="flex flex-wrap items-center gap-1.5">
        {buffs.map((b) => (
          <li key={b.id}>
            <Badge variant="outline" className="h-7 gap-1.5 pr-1 text-sm">
              <span className="font-medium">{b.name}</span>
              <span className="font-mono text-xs text-muted-foreground">
                {b.type === "charge"
                  ? t("buffs.charges", { count: b.charges })
                  : t("buffs.rounds", { count: b.duration })}
              </span>
              {b.type === "charge" && (
                <Button
                  size="icon-xs"
                  variant="ghost"
                  aria-label={t("buffs.useCharge", { name: b.name })}
                  title={t("buffs.useCharge", { name: b.name })}
                  onClick={() => void run(() => triggerBuff(b.id))}
                >
                  <Zap className="text-yellow-500" />
                </Button>
              )}
              <Button
                size="icon-xs"
                variant="ghost"
                aria-label={t("buffs.dismiss", { name: b.name })}
                title={t("buffs.dismiss", { name: b.name })}
                onClick={() => void run(() => dismissBuff(b.id))}
              >
                <X />
              </Button>
            </Badge>
          </li>
        ))}
        {buffs.length === 0 && (
          <li className="text-sm italic text-muted-foreground">
            {t("buffs.none")}
          </li>
        )}
      </ul>
      {adding ? (
        <AddBuffForm onDone={() => setAdding(false)} />
      ) : (
        <Button size="sm" variant="outline" onClick={() => setAdding(true)}>
          <Plus />
          {t("buffs.add")}
        </Button>
      )}
    </div>
  );
}

function AddBuffForm({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation();
  const activateBuff = useBuffsStore((s) => s.activateBuff);
  const { run, busy } = useTableStore();
  const [name, setName] = useState("");
  const [type, setType] = useState<"duration" | "charge">("duration");
  const [count, setCount] = useState("1");
  const [effectKey, setEffectKey] =
    useState<(typeof EFFECT_KEYS)[number]>("weapon_buff");
  const [effectValue, setEffectValue] = useState("1");

  const n = Number.parseInt(count, 10);
  const v = Number.parseFloat(effectValue);
  const valid = name.trim() !== "" && n > 0 && !Number.isNaN(v);

  async function submit() {
    if (!valid) return;
    const ok = await run(() =>
      activateBuff({
        name: name.trim(),
        type,
        charges: type === "charge" ? n : 0,
        duration: type === "duration" ? n : 0,
        effects: { [effectKey]: v },
      }),
    );
    if (ok) onDone();
  }

  return (
    <form
      className="flex flex-wrap items-center gap-1.5 rounded-lg border bg-muted/40 p-1.5"
      aria-label={t("buffs.add")}
      onSubmit={(e) => {
        e.preventDefault();
        void submit();
      }}
    >
      <input
        className={`${fieldClass} w-40`}
        aria-label={t("buffs.name")}
        placeholder={t("buffs.name")}
        value={name}
        onChange={(e) => setName(e.target.value)}
        autoFocus
      />
      <select
        className={`${fieldClass} w-28`}
        aria-label={t("buffs.kind")}
        value={type}
        onChange={(e) => setType(e.target.value as "duration" | "charge")}
      >
        <option value="duration">{t("buffs.typeRounds")}</option>
        <option value="charge">{t("buffs.typeCharges")}</option>
      </select>
      <input
        className={`${fieldClass} w-16`}
        type="number"
        min={1}
        aria-label={t("buffs.count")}
        value={count}
        onChange={(e) => setCount(e.target.value)}
      />
      <select
        className={`${fieldClass} w-32`}
        aria-label={t("buffs.effect")}
        value={effectKey}
        onChange={(e) =>
          setEffectKey(e.target.value as (typeof EFFECT_KEYS)[number])
        }
      >
        {EFFECT_KEYS.map((k) => (
          <option key={k} value={k}>
            {t(EFFECT_LABEL[k])}
          </option>
        ))}
      </select>
      <input
        className={`${fieldClass} w-16`}
        type="number"
        step="0.1"
        aria-label={t("buffs.effectValue")}
        value={effectValue}
        onChange={(e) => setEffectValue(e.target.value)}
      />
      <Button size="sm" type="submit" disabled={!valid || busy}>
        {t("buffs.activate")}
      </Button>
      <Button size="sm" type="button" variant="ghost" onClick={onDone}>
        {t("buffs.cancel")}
      </Button>
    </form>
  );
}
