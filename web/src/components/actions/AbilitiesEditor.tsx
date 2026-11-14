import { useId, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Plus, Trash, X } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { cn } from "@/lib/utils";
import { BuffEffectFields } from "./BuffEffectFields";
import {
  LIMIT_KINDS,
  newAbilityDraft,
  newEffectDraft,
  SAVE_CATEGORIES,
  type AbilityDraft,
  type DiceDraft,
} from "@/lib/abilityEditor";

/** What a dice field takes, as an example (a format, not a text in a language). */
const DICE_EXAMPLE = "2d6, 1d8+2";

interface Props {
  abilities: AbilityDraft[];
  onChange: (abilities: AbilityDraft[]) => void;
  /** Whether the piece has a quantity: only then may an ability be spent with the item. */
  hasQuantity: boolean;
  /** The server's refusal on field abilities, worded. */
  error?: string;
}

/** A labelled control that wraps with its neighbours and never cuts its label (ADR 0022 step 1). */
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
 * An item's abilities in the equipment editor (ADR 0049 steps 4, 5, 13 and 16): each ability has a
 * name (optional: none shows the item's), a usage limit with its numbers, a note and the dice it
 * rolls with the save the effect allows. Every number is validated by the server (and its bounds
 * are its own); the form only holds what was typed.
 */
export function AbilitiesEditor({
  abilities,
  onChange,
  hasQuantity,
  error,
}: Props) {
  const { t } = useTranslation();
  const id = useId();
  const set = (i: number, patch: Partial<AbilityDraft>) =>
    onChange(abilities.map((a, j) => (j === i ? { ...a, ...patch } : a)));
  // A new ability is full: while left still equals the max it follows the max typed.
  const setMax = (i: number, max: string) =>
    set(i, {
      max,
      left: abilities[i].left === abilities[i].max ? max : abilities[i].left,
    });
  // A stored with_item ability keeps its option even when the rules say the piece has no quantity
  // (or could not be read), so a save never drops it; the server judges it.
  const kindsFor = (current: AbilityDraft["kind"]) =>
    LIMIT_KINDS.filter(
      (k) => k !== "with_item" || hasQuantity || current === "with_item",
    );

  return (
    <section
      className="col-span-2 flex min-w-0 flex-col gap-2 sm:col-span-4"
      aria-labelledby={`${id}-title`}
      data-testid="item-abilities"
    >
      <div className="group-heading flex flex-wrap items-baseline justify-between gap-2 px-3">
        <span id={`${id}-title`} className="section-label">
          {t("equipment.abilities.title")}
        </span>
        <span className="text-label text-muted-foreground">
          {t("equipment.abilities.hint")}
        </span>
      </div>
      {abilities.length === 0 && (
        <p className="text-body text-muted-foreground">
          {t("equipment.abilities.none")}
        </p>
      )}
      {abilities.map((a, i) => {
        const counted = a.kind === "per_day" || a.kind === "charges";
        return (
          <div
            key={i}
            className="group-band flex flex-col gap-2 rounded-md px-3 py-2"
            data-testid="ability-row"
            role="group"
            aria-label={t("equipment.abilities.ability", { n: i + 1 })}
          >
            <div className="flex flex-wrap items-end gap-x-3 gap-y-2">
              <Labelled
                label={t("equipment.abilities.name")}
                className="min-w-48 flex-1"
              >
                {(fid) => (
                  <input
                    id={fid}
                    className={fieldClass}
                    value={a.name}
                    placeholder={t("equipment.abilities.namePlaceholder")}
                    onChange={(e) => set(i, { name: e.target.value })}
                    data-testid="ability-name"
                  />
                )}
              </Labelled>
              <Labelled label={t("equipment.abilities.limit")}>
                {(fid) => (
                  <select
                    id={fid}
                    className={cn(fieldClass, "w-auto")}
                    value={a.kind}
                    onChange={(e) =>
                      set(i, {
                        kind: e.target.value as AbilityDraft["kind"],
                      })
                    }
                    data-testid="ability-kind"
                  >
                    {kindsFor(a.kind).map((k) => (
                      <option key={k} value={k}>
                        {t(`equipment.abilities.kinds.${k}`)}
                      </option>
                    ))}
                  </select>
                )}
              </Labelled>
              {counted && (
                <Labelled
                  label={t(
                    a.kind === "per_day"
                      ? "equipment.abilities.perDay"
                      : "equipment.abilities.total",
                  )}
                >
                  {(fid) => (
                    <input
                      id={fid}
                      className={cn(
                        fieldClass,
                        "w-16",
                        error && "border-destructive",
                      )}
                      inputMode="numeric"
                      value={a.max}
                      onChange={(e) => setMax(i, e.target.value)}
                      data-testid="ability-max"
                    />
                  )}
                </Labelled>
              )}
              {counted && (
                <Labelled label={t("equipment.abilities.left")}>
                  {(fid) => (
                    <input
                      id={fid}
                      className={cn(fieldClass, "w-16")}
                      inputMode="numeric"
                      title={t("equipment.abilities.leftHint")}
                      value={a.left}
                      onChange={(e) => set(i, { left: e.target.value })}
                      data-testid="ability-left"
                    />
                  )}
                </Labelled>
              )}
              {a.kind === "charges" && (
                <Labelled label={t("equipment.abilities.cost")}>
                  {(fid) => (
                    <input
                      id={fid}
                      className={cn(fieldClass, "w-16")}
                      inputMode="numeric"
                      title={t("equipment.abilities.costHint")}
                      value={a.cost}
                      onChange={(e) => set(i, { cost: e.target.value })}
                      data-testid="ability-cost"
                    />
                  )}
                </Labelled>
              )}
              <Button
                size="icon-xs"
                type="button"
                variant="ghost"
                className="ml-auto"
                aria-label={t("equipment.abilities.remove", { n: i + 1 })}
                title={t("equipment.abilities.remove", { n: i + 1 })}
                onClick={() => onChange(abilities.filter((_, j) => j !== i))}
                data-testid="ability-remove"
              >
                <X />
              </Button>
            </div>
            <Labelled label={t("equipment.abilities.note")}>
              {(fid) => (
                <input
                  id={fid}
                  className={fieldClass}
                  value={a.note}
                  onChange={(e) => set(i, { note: e.target.value })}
                  data-testid="ability-note"
                />
              )}
            </Labelled>
            {a.effects.map((e, k) => {
              const heal = e.kind === "heal";
              const setEffect = (patch: Partial<DiceDraft>) =>
                set(i, {
                  effects: a.effects.map((x, j) =>
                    j === k ? { ...x, ...patch } : x,
                  ),
                });
              if (e.kind === "spell")
                return (
                  <div
                    key={k}
                    className="flex flex-wrap items-center gap-x-3 gap-y-2"
                    role="group"
                    aria-label={t("equipment.abilities.spellEffect", {
                      n: k + 1,
                    })}
                    data-testid="ability-effect"
                    data-kind="spell"
                  >
                    <span className="text-sm">
                      {t("equipment.abilities.spellEffectValue", {
                        spell: e.spellId,
                        level: e.level,
                      })}
                    </span>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      title={t("equipment.abilities.removeEffect", {
                        n: k + 1,
                      })}
                      onClick={() =>
                        set(i, { effects: a.effects.filter((_, j) => j !== k) })
                      }
                    >
                      <X />
                    </Button>
                  </div>
                );
              if (e.kind === "buff")
                return (
                  <BuffEffectFields
                    key={k}
                    effect={e}
                    n={k + 1}
                    onChange={setEffect}
                    onRemove={() =>
                      set(i, { effects: a.effects.filter((_, j) => j !== k) })
                    }
                  />
                );
              return (
                <div
                  key={k}
                  className="flex flex-wrap items-end gap-x-3 gap-y-2"
                  role="group"
                  aria-label={t(
                    heal
                      ? "equipment.abilities.healEffect"
                      : "equipment.abilities.effect",
                    { n: k + 1 },
                  )}
                  data-testid="ability-effect"
                  data-kind={e.kind}
                >
                  <Labelled
                    label={t(
                      heal
                        ? "equipment.abilities.healDice"
                        : "equipment.abilities.dice",
                    )}
                  >
                    {(fid) => (
                      <input
                        id={fid}
                        className={cn(fieldClass, "w-28")}
                        placeholder={DICE_EXAMPLE}
                        title={t(
                          heal
                            ? "equipment.abilities.healDiceHint"
                            : "equipment.abilities.diceHint",
                        )}
                        value={e.full ? "" : e.dice}
                        disabled={heal && e.full}
                        onChange={(ev) => setEffect({ dice: ev.target.value })}
                        data-testid="ability-dice"
                      />
                    )}
                  </Labelled>
                  {heal ? (
                    <Labelled label={t("equipment.abilities.healFull")}>
                      {(fid) => (
                        <input
                          id={fid}
                          type="checkbox"
                          className="size-5 accent-primary"
                          title={t("equipment.abilities.healFullHint")}
                          checked={e.full}
                          onChange={(ev) =>
                            setEffect({ full: ev.target.checked })
                          }
                          data-testid="ability-heal-full"
                        />
                      )}
                    </Labelled>
                  ) : (
                    <Labelled label={t("equipment.abilities.save")}>
                      {(fid) => (
                        <select
                          id={fid}
                          className={cn(fieldClass, "w-auto")}
                          value={e.save}
                          onChange={(ev) =>
                            setEffect({ save: ev.target.value })
                          }
                          data-testid="ability-save"
                        >
                          <option value="">{t("items.form.noSave")}</option>
                          {SAVE_CATEGORIES.map((s) => (
                            <option key={s} value={s}>
                              {t(`saves.categories.${s}`)}
                            </option>
                          ))}
                        </select>
                      )}
                    </Labelled>
                  )}
                  <Button
                    size="icon-xs"
                    type="button"
                    variant="ghost"
                    aria-label={t("equipment.abilities.removeEffect", {
                      n: k + 1,
                    })}
                    title={t("equipment.abilities.removeEffect", { n: k + 1 })}
                    onClick={() =>
                      set(i, { effects: a.effects.filter((_, j) => j !== k) })
                    }
                  >
                    <Trash />
                  </Button>
                </div>
              );
            })}
            <div className="flex flex-wrap gap-2 self-start">
              <Button
                size="sm"
                type="button"
                variant="ghost"
                title={t("equipment.abilities.addDiceHint")}
                aria-label={t("equipment.abilities.addDiceHint")}
                onClick={() =>
                  set(i, {
                    effects: [...a.effects, newEffectDraft("dice")],
                  })
                }
                data-testid="ability-add-dice"
              >
                <Plus /> {t("equipment.abilities.addDice")}
              </Button>
              <Button
                size="sm"
                type="button"
                variant="ghost"
                title={t("equipment.abilities.addHealHint")}
                aria-label={t("equipment.abilities.addHealHint")}
                onClick={() =>
                  set(i, {
                    effects: [...a.effects, newEffectDraft("heal")],
                  })
                }
                data-testid="ability-add-heal"
              >
                <Plus /> {t("equipment.abilities.addHeal")}
              </Button>
              <Button
                size="sm"
                type="button"
                variant="ghost"
                title={t("equipment.abilities.addBuffHint")}
                aria-label={t("equipment.abilities.addBuffHint")}
                onClick={() =>
                  set(i, {
                    effects: [...a.effects, newEffectDraft("buff")],
                  })
                }
                data-testid="ability-add-buff"
              >
                <Plus /> {t("equipment.abilities.addBuff")}
              </Button>
            </div>
          </div>
        );
      })}
      {error && (
        <p
          role="alert"
          className="text-label text-destructive"
          data-testid="field-error"
        >
          {error}
        </p>
      )}
      <div>
        <Button
          size="sm"
          type="button"
          title={t("equipment.abilities.addHint")}
          aria-label={t("equipment.abilities.addHint")}
          onClick={() =>
            onChange([
              ...abilities,
              newAbilityDraft(hasQuantity ? "with_item" : "per_day"),
            ])
          }
          data-testid="ability-add"
        >
          <Plus /> {t("equipment.abilities.add")}
        </Button>
      </div>
    </section>
  );
}
