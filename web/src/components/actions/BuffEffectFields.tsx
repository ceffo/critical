import { useId, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Plus, Trash } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { cn } from "@/lib/utils";
import {
  BUFF_EFFECT_KEYS,
  type BuffModKey,
  type DiceDraft,
  type ModDraft,
} from "@/lib/abilityEditor";

interface Props {
  effect: DiceDraft;
  onChange: (patch: Partial<DiceDraft>) => void;
  onRemove: () => void;
  /** The 1-based place of the effect in its ability, for the accessible names. */
  n: number;
}

function Labelled({
  label,
  className,
  children,
}: {
  label: string;
  className?: string;
  children: (id: string) => ReactNode;
}) {
  const id = useId();
  return (
    <div className={cn("flex min-w-0 flex-col gap-1", className)}>
      <label htmlFor={id} className="text-label text-muted-foreground">
        {label}
      </label>
      {children(id)}
    </div>
  );
}

/**
 * A buff effect of an ability (ADR 0049 steps 9, 10 and 12): the buff Use puts on the list, with its
 * name (none: the ability's, else the item's), its rounds (0: until dismissed) and the modifiers it
 * carries, in the buffs' effect keys. What each one does is the server's; the form holds what was typed.
 */
export function BuffEffectFields({ effect, onChange, onRemove, n }: Props) {
  const { t } = useTranslation();
  const setMod = (k: number, patch: Partial<ModDraft>) =>
    onChange({
      mods: effect.mods.map((m, j) => (j === k ? { ...m, ...patch } : m)),
    });
  return (
    <div
      className="flex flex-col gap-2"
      role="group"
      aria-label={t("equipment.abilities.buffEffect", { n })}
      data-testid="ability-effect"
      data-kind="buff"
    >
      <div className="flex flex-wrap items-end gap-x-3 gap-y-2">
        <Labelled
          label={t("equipment.abilities.buffName")}
          className="min-w-48 flex-1"
        >
          {(fid) => (
            <input
              id={fid}
              className={fieldClass}
              value={effect.name}
              placeholder={t("equipment.abilities.buffNamePlaceholder")}
              onChange={(ev) => onChange({ name: ev.target.value })}
              data-testid="ability-buff-name"
            />
          )}
        </Labelled>
        <Labelled label={t("equipment.abilities.buffRounds")}>
          {(fid) => (
            <input
              id={fid}
              className={cn(fieldClass, "w-20")}
              inputMode="numeric"
              title={t("equipment.abilities.buffRoundsHint")}
              value={effect.rounds}
              onChange={(ev) => onChange({ rounds: ev.target.value })}
              data-testid="ability-buff-rounds"
            />
          )}
        </Labelled>
        <Button
          size="icon-xs"
          type="button"
          variant="ghost"
          className="ml-auto"
          aria-label={t("equipment.abilities.removeEffect", { n })}
          title={t("equipment.abilities.removeEffect", { n })}
          onClick={onRemove}
        >
          <Trash />
        </Button>
      </div>
      <div className="flex flex-wrap items-end gap-x-4 gap-y-2">
        {effect.mods.map((m, k) => (
          <span key={k} className="flex items-center gap-1">
            <select
              className={cn(fieldClass, "w-auto")}
              aria-label={t("equipment.powers.effect")}
              value={m.key}
              onChange={(ev) =>
                setMod(k, { key: ev.target.value as BuffModKey })
              }
              data-testid="ability-buff-mod-key"
            >
              {BUFF_EFFECT_KEYS.map((key) => (
                <option key={key} value={key}>
                  {t(`buffs.effects.${key}`)}
                </option>
              ))}
            </select>
            <input
              className={cn(fieldClass, "w-16")}
              inputMode="decimal"
              aria-label={t("equipment.powers.value")}
              value={m.value}
              onChange={(ev) => setMod(k, { value: ev.target.value })}
              data-testid="ability-buff-mod-value"
            />
            <Button
              size="icon-xs"
              type="button"
              variant="ghost"
              aria-label={t("equipment.powers.removeEffect")}
              title={t("equipment.powers.removeEffect")}
              onClick={() =>
                onChange({ mods: effect.mods.filter((_, j) => j !== k) })
              }
            >
              <Trash />
            </Button>
          </span>
        ))}
        <Button
          size="sm"
          type="button"
          variant="ghost"
          title={t("equipment.powers.addEffectHint")}
          onClick={() =>
            onChange({
              mods: [...effect.mods, { key: "save_bonus", value: "1" }],
            })
          }
          data-testid="ability-buff-add-mod"
        >
          <Plus /> {t("equipment.powers.addEffect")}
        </Button>
      </div>
    </div>
  );
}
