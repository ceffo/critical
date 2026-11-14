import { useId, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogTitle,
} from "@/components/ui/dialog";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { localized } from "@/i18n/localized";
import { useSpellName } from "@/i18n/spellName";
import { castMenu, planAsks, type AskForm } from "@/lib/castInputs";
import { askedEffects } from "@/lib/itemAbilities";
import { splitText } from "@/lib/slotSplit";
import { modeName } from "@/lib/spellMode";
import type {
  Item,
  ItemAbility,
  SpellAnswer,
  UseAbilityBody,
} from "@/types/items";
import type { CastOn } from "@/types/spells";
import { cn } from "@/lib/utils";

/** One answer as typed for a spell effect: the cast inputs of a slot, plus who the spell lands on. */
interface Typed extends AskForm {
  on?: CastOn;
}

function Labelled({
  label,
  children,
}: {
  label: string;
  children: (id: string) => ReactNode;
}) {
  const id = useId();
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <label htmlFor={id} className="text-label text-muted-foreground">
        {label}
      </label>
      {children(id)}
    </div>
  );
}

/**
 * The Use dialog of an ability with a spell effect that asks something (ADR 0049 steps 9 and 10):
 * the target AC of a to-hit, the item a spell spends, the split of the slots it grants, the cast-time
 * mode, who a buff lands on, each from what the server serves for that effect, in the cast's own
 * fields. It only collects: the server validates the answers and decides everything else, and a
 * refusal comes back as it does for a cast. Names wrap, never cut (ADR 0022 step 1).
 */
export function UseAbilityDialog({
  row,
  onClose,
  onUse,
}: {
  /** The ability being used; null: closed. */
  row: { item: Item; ability: ItemAbility } | null;
  onClose: () => void;
  onUse: (body: UseAbilityBody) => void;
}) {
  const { t, i18n } = useTranslation();
  const spellName = useSpellName();
  const [typed, setTyped] = useState<Record<number, Typed>>({});
  const asked = row ? askedEffects(row.ability) : [];
  const plans = asked.map(({ index, effect }) => {
    const form = typed[index] ?? {};
    return {
      index,
      effect,
      form,
      plan: planAsks(effect.asks, form),
    };
  });
  const blocked = plans.some((p) => p.plan.missingAc);
  const itemName = row ? localized(i18n.language, row.item.name) : "";
  const abilityName = row?.ability.name
    ? localized(i18n.language, row.ability.name)
    : "";
  const name = abilityName ? `${itemName} · ${abilityName}` : itemName;

  const set = (index: number, patch: Typed) =>
    setTyped((all) => ({ ...all, [index]: { ...all[index], ...patch } }));
  const close = () => {
    setTyped({});
    onClose();
  };
  const submit = () => {
    const spells: SpellAnswer[] = plans.map(({ index, effect, form, plan }) => {
      const menu = castMenu(effect.asks?.cast_target);
      const picked = form.on;
      return {
        effect: index,
        ...plan.answer,
        // Only a target other than where the spell lands by default is sent.
        ...(picked && menu.length > 1 && picked !== menu[0]
          ? { onSelf: picked === "self" }
          : {}),
      };
    });
    setTyped({});
    onUse({ spells });
  };

  return (
    <Dialog open={row !== null} onOpenChange={(open) => !open && close()}>
      <DialogContent
        className="max-w-[40rem] gap-3 overflow-y-auto"
        data-testid="use-ability-dialog"
      >
        <DialogTitle className="text-title break-words">
          {t("actions.items.ask.title", { name })}
        </DialogTitle>
        {plans.map(({ index, effect, form, plan }) => {
          const asks = effect.asks;
          const spellId = effect.spellId ?? "";
          const menu = castMenu(asks?.cast_target);
          return (
            <section
              key={index}
              className="flex flex-col gap-2"
              aria-label={t("actions.items.ask.spell", {
                spell: spellName(spellId, spellId),
                level: effect.level ?? 0,
              })}
              data-testid="use-ability-spell"
            >
              <h3 className="text-body font-medium break-words">
                {t("actions.items.ask.spell", {
                  spell: spellName(spellId, spellId),
                  level: effect.level ?? 0,
                })}
              </h3>
              <div className="flex flex-wrap items-end gap-x-3 gap-y-2">
                {asks?.attack_roll && (
                  <Labelled label={t("spellSlots.targetAc")}>
                    {(id) => (
                      <input
                        id={id}
                        className={cn(fieldClass, "w-20")}
                        inputMode="numeric"
                        placeholder={t("attack.acShort")}
                        value={form.acText ?? ""}
                        onChange={(e) => set(index, { acText: e.target.value })}
                        data-testid="use-ability-ac"
                      />
                    )}
                  </Labelled>
                )}
                {plan.spendable.length > 1 && (
                  <Labelled label={t("spellSlots.itemSpent")}>
                    {(id) => (
                      <select
                        id={id}
                        className={cn(fieldClass, "w-auto")}
                        value={plan.itemId}
                        onChange={(e) =>
                          set(index, { itemPick: e.target.value })
                        }
                        data-testid="use-ability-item"
                      >
                        {plan.spendable.map((i) => (
                          <option key={i.id} value={i.id}>
                            {localized(i18n.language, i.name)} ({i.count})
                          </option>
                        ))}
                      </select>
                    )}
                  </Labelled>
                )}
                {plan.modes.length > 1 && (
                  <Labelled label={t("spellSlots.modeChoice")}>
                    {(id) => (
                      <select
                        id={id}
                        className={cn(fieldClass, "w-auto")}
                        value={plan.mode?.key}
                        onChange={(e) => set(index, { mode: e.target.value })}
                        data-testid="use-ability-mode"
                      >
                        {plan.modes.map((m) => (
                          <option key={m.key} value={m.key}>
                            {modeName(t, spellId, m.key)}
                          </option>
                        ))}
                      </select>
                    )}
                  </Labelled>
                )}
                {plan.slotSplits.length > 1 && (
                  <Labelled label={t("spellSlots.splitChoice")}>
                    {(id) => (
                      <select
                        id={id}
                        className={cn(fieldClass, "w-auto max-w-full")}
                        value={plan.split}
                        onChange={(e) => set(index, { split: e.target.value })}
                        data-testid="use-ability-split"
                      >
                        {plan.slotSplits.map((levels) => (
                          <option
                            key={levels.join(",")}
                            value={levels.join(",")}
                          >
                            {splitText(t, levels)}
                          </option>
                        ))}
                      </select>
                    )}
                  </Labelled>
                )}
                {menu.length > 1 && (
                  <Labelled label={t("actions.items.ask.target")}>
                    {(id) => (
                      <select
                        id={id}
                        className={cn(fieldClass, "w-auto")}
                        value={form.on ?? menu[0]}
                        onChange={(e) =>
                          set(index, { on: e.target.value as CastOn })
                        }
                        data-testid="use-ability-target"
                      >
                        {menu.map((on) => (
                          <option key={on} value={on}>
                            {on === "self"
                              ? t("spellSlots.castOnSelf")
                              : t("spellSlots.castOnAlly")}
                          </option>
                        ))}
                      </select>
                    )}
                  </Labelled>
                )}
              </div>
            </section>
          );
        })}
        <div className="flex justify-end gap-2">
          <DialogClose render={<Button variant="ghost" />}>
            {t("actions.items.ask.cancel")}
          </DialogClose>
          <Button
            disabled={blocked}
            onClick={submit}
            data-testid="use-ability-confirm"
          >
            {t("actions.items.use")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
