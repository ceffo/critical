import { useId } from "react";
import { useTranslation } from "react-i18next";
import { CaretDown, Plus, Trash, X } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { cn } from "@/lib/utils";
import {
  newPowerDraft,
  POWER_EFFECT_KEYS,
  POWER_TEMPLATES,
  templateDefault,
  templatePower,
  type EffectDraft,
  type PowerDraft,
  type PowerEffectKey,
} from "@/lib/powerEditor";
import type { WeaponKind } from "@/types/buffs";

interface Props {
  powers: PowerDraft[];
  onChange: (powers: PowerDraft[]) => void;
  /** An armour or a shield: its "+N" is a field of its own, these are its other powers. */
  others: boolean;
  /** Offer the ready-made rows (a magic piece, ADR 0035 item 11). */
  templates: boolean;
  /** The server's refusal on field powers, worded. */
  error?: string;
}

/**
 * A piece's powers in the equipment editor (ADR 0033 step 17): each power is effects from the list
 * of ADR 0016 step 9 plus base_ac, an optional trigger (STR at least N) and its conditions (melee or
 * missile only; without magic armour). They are on while the piece is equipped (step 9). What each
 * one adds is the server's; the form only holds what was typed.
 */
export function PowersEditor({
  powers,
  onChange,
  others,
  templates,
  error,
}: Props) {
  const { t } = useTranslation();
  const id = useId();
  const setPower = (i: number, patch: Partial<PowerDraft>) =>
    onChange(powers.map((p, j) => (j === i ? { ...p, ...patch } : p)));
  const setEffect = (i: number, k: number, patch: Partial<EffectDraft>) =>
    setPower(i, {
      effects: powers[i].effects.map((e, j) =>
        j === k ? { ...e, ...patch } : e,
      ),
    });

  return (
    <section
      className="col-span-2 flex min-w-0 flex-col gap-2 sm:col-span-4"
      aria-labelledby={`${id}-title`}
      data-testid="item-powers"
    >
      <div className="flex flex-wrap items-baseline justify-between gap-2 group-heading px-3">
        <span id={`${id}-title`} className="section-label">
          {others ? t("equipment.powers.others") : t("equipment.powers.title")}
        </span>
        <span className="text-label text-muted-foreground">
          {others
            ? t("equipment.powers.othersHint")
            : t("equipment.powers.hint")}
        </span>
      </div>
      {powers.length === 0 && (
        <p className="text-body text-muted-foreground">
          {t("equipment.powers.none")}
        </p>
      )}
      {powers.map((p, i) => (
        <div
          key={i}
          className="flex flex-wrap items-center gap-x-4 gap-y-2 rounded-md group-band px-3 py-2"
          data-testid="power-row"
          role="group"
          aria-label={t("equipment.powers.power", { n: i + 1 })}
        >
          {p.effects.map((e, k) => (
            <span key={k} className="flex items-center gap-1">
              <select
                className={cn(fieldClass, "w-auto")}
                aria-label={t("equipment.powers.effect")}
                value={e.key}
                onChange={(ev) =>
                  setEffect(i, k, { key: ev.target.value as PowerEffectKey })
                }
                data-testid="power-effect"
              >
                {POWER_EFFECT_KEYS.map((key) => (
                  <option key={key} value={key}>
                    {t(`buffs.effects.${key}`)}
                  </option>
                ))}
              </select>
              <input
                className={cn(
                  fieldClass,
                  "w-16",
                  error && "border-destructive",
                )}
                inputMode="decimal"
                aria-label={t("equipment.powers.value")}
                value={e.value}
                onChange={(ev) => setEffect(i, k, { value: ev.target.value })}
                data-testid="power-value"
              />
              <Button
                size="icon-xs"
                type="button"
                variant="ghost"
                aria-label={t("equipment.powers.removeEffect")}
                title={t("equipment.powers.removeEffect")}
                onClick={() =>
                  setPower(i, {
                    effects: p.effects.filter((_, j) => j !== k),
                  })
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
              setPower(i, {
                effects: [...p.effects, { key: "save_bonus", value: "1" }],
              })
            }
          >
            <Plus /> {t("equipment.powers.addEffect")}
          </Button>
          <label className="flex items-center gap-1">
            <span className="text-label text-muted-foreground">
              {t("equipment.powers.minStr")}
            </span>
            <input
              className={cn(fieldClass, "w-14")}
              inputMode="numeric"
              title={t("equipment.powers.minStrHint")}
              value={p.minStr}
              onChange={(ev) => setPower(i, { minStr: ev.target.value })}
              data-testid="power-min-str"
            />
          </label>
          <select
            className={cn(fieldClass, "w-auto")}
            aria-label={t("equipment.powers.weaponKind")}
            title={t("equipment.powers.weaponKind")}
            value={p.weaponKind}
            onChange={(ev) =>
              setPower(i, { weaponKind: ev.target.value as WeaponKind | "" })
            }
          >
            <option value="">{t("equipment.powers.everyRoll")}</option>
            <option value="melee">{t("equipment.powers.meleeOnly")}</option>
            <option value="missile">{t("equipment.powers.missileOnly")}</option>
          </select>
          <label
            className="flex items-center gap-1 text-body"
            title={t("equipment.powers.withoutMagicArmourHint")}
          >
            <input
              type="checkbox"
              className="size-4"
              checked={p.withoutMagicArmour}
              onChange={(ev) =>
                setPower(i, { withoutMagicArmour: ev.target.checked })
              }
              data-testid="power-without-magic-armour"
            />
            {t("equipment.powers.withoutMagicArmour")}
          </label>
          <Button
            size="icon-xs"
            type="button"
            variant="ghost"
            className="ml-auto"
            aria-label={t("equipment.powers.remove", { n: i + 1 })}
            title={t("equipment.powers.remove", { n: i + 1 })}
            onClick={() => onChange(powers.filter((_, j) => j !== i))}
            data-testid="power-remove"
          >
            <X />
          </Button>
        </div>
      ))}
      {error && (
        <p
          role="alert"
          className="text-label text-destructive"
          data-testid="field-error"
        >
          {error}
        </p>
      )}
      <div className="flex flex-wrap gap-2">
        <Button
          size="sm"
          type="button"
          title={t("equipment.powers.addHint")}
          onClick={() => onChange([...powers, newPowerDraft()])}
          data-testid="power-add"
        >
          <Plus /> {t("equipment.powers.add")}
        </Button>
        {templates && (
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button
                  size="sm"
                  type="button"
                  title={t("equipment.powers.templatesHint")}
                  data-testid="power-templates"
                />
              }
            >
              {t("equipment.powers.templates")} <CaretDown aria-hidden />
            </DropdownMenuTrigger>
            <DropdownMenuContent>
              {POWER_TEMPLATES.map((tpl) => (
                <DropdownMenuItem
                  key={tpl}
                  onClick={() =>
                    onChange([
                      ...powers,
                      templatePower(tpl, templateDefault(tpl)),
                    ])
                  }
                  data-testid={`power-template-${tpl}`}
                >
                  <span className="flex flex-col">
                    <span>{t(`equipment.powers.template.${tpl}`)}</span>
                    <span className="text-label text-muted-foreground">
                      {t(`equipment.powers.templateHint.${tpl}`)}
                    </span>
                  </span>
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>
    </section>
  );
}
