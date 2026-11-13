import { useId, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { localized } from "@/i18n/localized";
import { Minus, Plus } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { cn } from "@/lib/utils";
import {
  changedFields,
  draftToBody,
  hasField,
  holdsSpell,
  ratesStrength,
  type DraftErrors,
  type ItemDraft,
  type ItemField,
} from "@/lib/itemForm";
import type { ApiError } from "@/api/http";
import {
  AMMO_KINDS,
  ARMOR_ROWS,
  CATEGORIES,
  COMPONENTS,
  DAMAGE_TYPES,
  MAGIC_KINDS,
  SHIELD_KINDS,
  SIZES,
  SKILL_COLUMNS,
  type Category,
  type Component,
  type Item,
  type ItemBehaviour,
  type ItemBody,
  type MagicKind,
} from "@/types/items";
import { WEAPON_TYPES, type WeaponType } from "@/types/combat";
import { hasPlus, MAX_PLUS } from "@/lib/powerEditor";
import { formatSigned } from "@/lib/format";
import { PowersEditor } from "./PowersEditor";
import { useSpellNamesStore } from "@/stores/spellNames";

const BEHAVIOURS: ItemBehaviour[] = ["stock", "charges", "daily"];
/** The shared fields folded under "More options" (every category has them). */
const EXTRA_FIELDS: ItemField[] = [
  "behaviour",
  "maxCount",
  "effectDice",
  "effectSave",
  "component",
  "notes",
  "powers",
];
/** Save categories an item effect may allow (the server rejects anything else). */
const SAVES = [
  "para_poison",
  "rod_staff_wand",
  "petri_poly",
  "breath",
  "spell",
];

interface Props {
  /** Starting values. */
  initial: ItemDraft;
  /** The piece being edited; undefined when adding. */
  editing?: Item;
  busy: boolean;
  /** Save a body; resolves the server's rejection, or null when saved. */
  onSave: (body: ItemBody) => Promise<ApiError | null>;
  onCancel: () => void;
}

/**
 * Add / edit form of a piece of equipment (ADR 0033 step 1: one editor for every category): the
 * category first, then the shared fields and the category's own (ADR 0033 steps 3 to 5).
 * Validation is the server's: its message, or the wording of its code, is shown under the field it
 * names. Only number parsing happens here.
 */
export function ItemForm({ initial, editing, busy, onSave, onCancel }: Props) {
  const { t, i18n } = useTranslation();
  const id = useId();
  const [draft, setDraft] = useState<ItemDraft>(initial);
  const [errors, setErrors] = useState<Partial<Record<ItemField, string>>>({});
  // Adding is two steps: what it is (name, category), then that category's fields. Editing shows
  // both at once, in a dialog.
  const [step, setStep] = useState<1 | 2>(1);
  const identity = !!editing || step === 1;
  const details = !!editing || step === 2;
  // The shared extras (behaviour, effect, note, powers) fold under "More options" unless the piece
  // is being edited or its category is about powers.
  const [moreOpen, setMoreOpen] = useState(!!editing);
  const set = <K extends ItemField>(k: K, v: ItemDraft[K]) => {
    setDraft((d) => ({ ...d, [k]: v }));
    setErrors((e) => ({ ...e, [k]: undefined }));
  };
  const shows = (f: ItemField) => details && hasField(draft.category, f);
  const spellNames = useSpellNamesStore((s) => s.names);
  // The catalogue's spells by name in the UI language, for a scroll's spell (ADR 0033 step 2).
  const spellOptions = Object.values(spellNames)
    .map((s) => ({ id: s.id, name: localized(i18n.language, s.name) || s.id }))
    .sort((a, b) => a.name.localeCompare(b.name, i18n.language));
  if (draft.spellId && !spellNames[draft.spellId])
    spellOptions.unshift({ id: draft.spellId, name: draft.spellId });

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!details) {
      setMoreOpen((o) => o || draft.category === "magic");
      setStep(2);
      return;
    }
    const { body, errors: parseErrors } = draftToBody(draft, i18n.language);
    const local = Object.fromEntries(
      Object.entries(parseErrors as DraftErrors).map(([k, v]) => [
        k,
        t(`items.form.${v}`),
      ]),
    );
    if (Object.keys(local).length > 0) {
      setErrors(local);
      if (Object.keys(local).some((k) => EXTRA_FIELDS.includes(k as ItemField)))
        setMoreOpen(true);
      return;
    }
    const toSend = editing ? changedFields(editing, body, i18n.language) : body;
    if (editing && Object.keys(toSend).length === 0) {
      onCancel();
      return;
    }
    const rejected = await onSave(toSend);
    // The server's message, or the wording of its code (ADR 0023: not_allowed).
    if (rejected?.field) {
      if (EXTRA_FIELDS.includes(rejected.field as ItemField)) setMoreOpen(true);
      if (rejected.field === "name" || rejected.field === "category")
        setStep(1);
      setErrors({
        [rejected.field]: t(`equipment.form.errors.${rejected.message}`, {
          defaultValue: rejected.message,
        }),
      });
    }
  }

  const field = (
    name: ItemField,
    label: string,
    input: (props: object) => ReactNode,
    className?: string,
  ) => {
    const err = errors[name];
    const inputId = `${id}-${name}`;
    return (
      <div
        className={cn("flex min-w-0 flex-col gap-1", className)}
        data-field={name}
      >
        <label htmlFor={inputId} className="text-label text-muted-foreground">
          {label}
        </label>
        {input({
          id: inputId,
          "aria-invalid": err ? true : undefined,
          "aria-describedby": err ? `${inputId}-error` : undefined,
        })}
        {err && (
          <p
            id={`${inputId}-error`}
            role="alert"
            className="text-label text-destructive"
            data-testid="field-error"
          >
            {err}
          </p>
        )}
      </div>
    );
  };
  const text =
    (name: ItemField, extra: object = {}) =>
    (props: object) => (
      <input
        {...props}
        {...extra}
        className={cn(fieldClass, errors[name] && "border-destructive")}
        value={draft[name] as string}
        onChange={(e) => set(name, e.target.value as never)}
      />
    );
  const select =
    (
      name: ItemField,
      options: readonly string[],
      label: (o: string) => string,
    ) =>
    (props: object) => (
      <select
        {...props}
        className={cn(fieldClass, errors[name] && "border-destructive")}
        value={draft[name] as string}
        onChange={(e) => set(name, e.target.value as never)}
      >
        {options.map((o) => (
          <option key={o} value={o}>
            {label(o)}
          </option>
        ))}
      </select>
    );
  const check = (name: "twoHanded", hint?: string) =>
    function Check(props: object) {
      return (
        <input
          {...props}
          type="checkbox"
          className="size-5"
          title={hint}
          checked={draft[name]}
          onChange={(e) => set(name, e.target.checked)}
        />
      );
    };

  return (
    <form
      onSubmit={(e) => void submit(e)}
      noValidate
      className="flex flex-col gap-2 rounded-lg bg-raised p-3"
      aria-label={
        editing
          ? t("items.form.editTitle", {
              name: localized(i18n.language, editing.name),
            })
          : t("items.form.addTitle")
      }
      data-testid="item-form"
    >
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        {!identity && (
          <p
            className="col-span-2 flex min-w-0 flex-col break-words sm:col-span-4"
            data-testid="item-summary"
          >
            <span className="font-medium">{draft.name}</span>
            <span className="text-label text-muted-foreground">
              {t(`equipment.category.${draft.category}`)}
            </span>
          </p>
        )}
        {identity &&
          field("name", t("items.form.name"), text("name"), "col-span-2")}
        {identity &&
          field("category", t("equipment.form.category"), (props) => (
            <select
              {...props}
              className={cn(
                fieldClass,
                errors.category && "border-destructive",
              )}
              value={draft.category}
              onChange={(e) => {
                const c = e.target.value as Category;
                set("category", c);
                if (c === "magic" && draft.magicKind === "")
                  set("magicKind", "misc");
              }}
            >
              {CATEGORIES.map((c) => (
                <option key={c} value={c}>
                  {t(`equipment.category.${c}`)}
                </option>
              ))}
            </select>
          ))}
        {shows("magicKind") &&
          field(
            "magicKind",
            t("equipment.form.magicKind"),
            select("magicKind", MAGIC_KINDS, (k) =>
              t(`equipment.magicKinds.${k as MagicKind}`),
            ),
          )}
        {details &&
          holdsSpell(draft) &&
          field(
            "spellId",
            t("equipment.form.spell"),
            (props) => (
              <select
                {...props}
                className={cn(
                  fieldClass,
                  errors.spellId && "border-destructive",
                )}
                title={t("equipment.form.spellHint")}
                value={draft.spellId}
                onChange={(e) => set("spellId", e.target.value)}
              >
                <option value="">{t("equipment.form.noSpell")}</option>
                {spellOptions.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </select>
            ),
            // A spell name is long: the picker takes the whole row, never cut.
            "col-span-2 sm:col-span-4",
          )}

        {/* Weapon (ADR 0033 step 3). */}
        {shows("weaponKind") &&
          field("weaponKind", t("weapons.form.kind"), (props) => (
            <select
              {...props}
              className={fieldClass}
              value={draft.weaponKind}
              onChange={(e) =>
                set("weaponKind", e.target.value as "melee" | "missile")
              }
            >
              <option value="melee">{t("weapons.kind.melee")}</option>
              <option value="missile">{t("weapons.kind.missile")}</option>
            </select>
          ))}
        {shows("weaponType") &&
          field("weaponType", t("weapons.form.weaponType"), (props) => (
            <select
              {...props}
              className={cn(
                fieldClass,
                errors.weaponType && "border-destructive",
              )}
              title={t("weapons.form.weaponTypeHint")}
              value={draft.weaponType}
              onChange={(e) => set("weaponType", e.target.value as WeaponType)}
            >
              <option value="">{t("weapons.types.none")}</option>
              {WEAPON_TYPES.map((wt) => (
                <option key={wt} value={wt}>
                  {t(`weapons.types.${wt}`)}
                </option>
              ))}
            </select>
          ))}
        {shows("damageSm") &&
          field(
            "damageSm",
            t("weapons.form.damageSm"),
            text("damageSm", { placeholder: "1d8" }),
          )}
        {shows("damageL") &&
          field(
            "damageL",
            t("weapons.form.damageL"),
            text("damageL", { placeholder: "1d12" }),
          )}
        {shows("magicToHit") &&
          field(
            "magicToHit",
            t("weapons.form.magicToHit"),
            text("magicToHit", { inputMode: "numeric" }),
          )}
        {shows("magicDamage") &&
          field(
            "magicDamage",
            t("weapons.form.magicDamage"),
            text("magicDamage", { inputMode: "numeric" }),
          )}
        {shows("speed") &&
          field(
            "speed",
            t("equipment.form.speed"),
            text("speed", { inputMode: "numeric" }),
          )}
        {shows("size") &&
          field("size", t("equipment.form.size"), (props) => (
            <select
              {...props}
              className={fieldClass}
              value={draft.size}
              onChange={(e) => set("size", e.target.value)}
            >
              <option value="">{t("equipment.form.noSize")}</option>
              {SIZES.map((s) => (
                <option key={s} value={s}>
                  {t(`equipment.sizes.${s}`)}
                </option>
              ))}
            </select>
          ))}
        {shows("damageTypes") && (
          <fieldset
            className="col-span-2 flex min-w-0 flex-col gap-1"
            data-field="damageTypes"
          >
            <legend className="text-label text-muted-foreground">
              {t("equipment.form.damageTypes")}
            </legend>
            <div className="flex flex-wrap gap-3">
              {DAMAGE_TYPES.map((dt) => (
                <label key={dt} className="flex items-center gap-1 text-body">
                  <input
                    type="checkbox"
                    className="size-4"
                    checked={draft.damageTypes.includes(dt)}
                    onChange={(e) =>
                      set(
                        "damageTypes",
                        e.target.checked
                          ? DAMAGE_TYPES.filter(
                              (x) => x === dt || draft.damageTypes.includes(x),
                            )
                          : draft.damageTypes.filter((x) => x !== dt),
                      )
                    }
                  />
                  {t(`equipment.damageTypes.${dt}`)}
                </label>
              ))}
            </div>
            {errors.damageTypes && (
              <p role="alert" className="text-label text-destructive">
                {errors.damageTypes}
              </p>
            )}
          </fieldset>
        )}
        {shows("twoHanded") &&
          field("twoHanded", t("weapons.form.twoHanded"), check("twoHanded"))}
        {shows("fires") &&
          draft.weaponKind === "missile" &&
          field("fires", t("equipment.form.fires"), (props) => (
            <select
              {...props}
              className={cn(fieldClass, errors.fires && "border-destructive")}
              value={draft.fires}
              onChange={(e) => set("fires", e.target.value)}
            >
              <option value="">{t("equipment.form.firesNone")}</option>
              {AMMO_KINDS.map((k) => (
                <option key={k} value={k}>
                  {t(`equipment.ammoKinds.${k}`)}
                </option>
              ))}
            </select>
          ))}
        {/* A bow's Strength rating (ADR 0033 step 3): a strength bow; empty for an ordinary one. */}
        {shows("strengthRating") &&
          ratesStrength(draft) &&
          field(
            "strengthRating",
            t("weapons.form.strengthRating"),
            text("strengthRating", {
              placeholder: "18/50",
              title: t("weapons.form.strengthRatingHint"),
            }),
          )}

        {/* Ammunition (ADR 0033 step 4). */}
        {shows("ammoKind") &&
          field(
            "ammoKind",
            t("items.form.ammoKind"),
            select("ammoKind", AMMO_KINDS, (k) =>
              t(`equipment.ammoKinds.${k}`),
            ),
          )}
        {shows("magicalBonus") &&
          field(
            "magicalBonus",
            t("items.magicalBonus"),
            text("magicalBonus", { inputMode: "numeric" }),
          )}

        {/* Armour and shield (ADR 0033 step 5). */}
        {shows("baseAc") &&
          field(
            "baseAc",
            t("equipment.form.baseAc"),
            text("baseAc", { inputMode: "numeric" }),
          )}
        {shows("skillColumn") &&
          field(
            "skillColumn",
            t("equipment.form.skillColumn"),
            select("skillColumn", SKILL_COLUMNS, (c) =>
              t(`equipment.skillColumns.${c}`),
            ),
          )}
        {shows("armorRow") &&
          field(
            "armorRow",
            t("equipment.form.armorRow"),
            select("armorRow", ARMOR_ROWS, (r) =>
              t(`equipment.armorRows.${r}`),
            ),
          )}
        {shows("shieldKind") &&
          field(
            "shieldKind",
            t("equipment.form.shieldKind"),
            select("shieldKind", SHIELD_KINDS, (k) =>
              t(`equipment.shieldKinds.${k}`),
            ),
          )}
        {shows("acMelee") &&
          field(
            "acMelee",
            t("equipment.form.acMelee"),
            text("acMelee", { inputMode: "numeric" }),
          )}
        {shows("acMissile") &&
          field(
            "acMissile",
            t("equipment.form.acMissile"),
            text("acMissile", { inputMode: "numeric" }),
          )}
        {/* The armour's or shield's "+N" (ADR 0033 step 17): its magic power, ac -N. */}
        {details && hasPlus(draft.category) && (
          <div
            role="group"
            aria-labelledby={`${id}-plus`}
            className="flex min-w-0 flex-col gap-1"
            data-field="plus"
          >
            <span
              id={`${id}-plus`}
              className="text-label text-muted-foreground"
            >
              {t("equipment.form.plus")}
            </span>
            <span className="flex items-center gap-1">
              <Button
                size="icon-sm"
                type="button"
                disabled={draft.plus <= 0}
                aria-label={t("equipment.form.plusLess")}
                title={t("equipment.form.plusLess")}
                onClick={() => set("plus", draft.plus - 1)}
              >
                <Minus />
              </Button>
              <span
                className="min-w-10 text-center font-semibold tabular-nums"
                data-testid="item-plus"
              >
                {formatSigned(draft.plus)}
              </span>
              <Button
                size="icon-sm"
                type="button"
                disabled={draft.plus >= MAX_PLUS}
                aria-label={t("equipment.form.plusMore")}
                title={t("equipment.form.plusMore")}
                onClick={() => set("plus", draft.plus + 1)}
              >
                <Plus />
              </Button>
            </span>
          </div>
        )}

        {/* Shared (ADR 0016 steps 1 to 5): the count always; the rest under More options. */}
        {details && (
          <>
            {field(
              "count",
              t("items.form.count"),
              text("count", { inputMode: "numeric" }),
            )}
          </>
        )}
        {details && (
          <details
            className="col-span-2 sm:col-span-4"
            open={moreOpen}
            onToggle={(e) => setMoreOpen(e.currentTarget.open)}
            data-testid="item-more"
          >
            <summary className="cursor-pointer text-label text-muted-foreground">
              {t("items.form.more")}
            </summary>
            <div className="mt-2 grid grid-cols-2 gap-2 sm:grid-cols-4">
              {field("behaviour", t("items.form.behaviour"), (props) => (
                <select
                  {...props}
                  className={fieldClass}
                  value={draft.behaviour}
                  onChange={(e) =>
                    set("behaviour", e.target.value as ItemBehaviour)
                  }
                >
                  {BEHAVIOURS.map((b) => (
                    <option key={b} value={b}>
                      {t(`items.behaviour.${b}`)}
                    </option>
                  ))}
                </select>
              ))}
              {draft.behaviour === "daily" &&
                field(
                  "maxCount",
                  t("items.form.maxCount"),
                  text("maxCount", { inputMode: "numeric" }),
                )}
              {field(
                "effectDice",
                t("items.form.effectDice"),
                text("effectDice", { placeholder: "2d6, 1d8+2" }),
              )}
              {field("effectSave", t("items.form.effectSave"), (props) => (
                <select
                  {...props}
                  className={fieldClass}
                  value={draft.effectSave}
                  onChange={(e) => set("effectSave", e.target.value)}
                >
                  <option value="">{t("items.form.noSave")}</option>
                  {SAVES.map((s) => (
                    <option key={s} value={s}>
                      {t(`saves.categories.${s}`)}
                    </option>
                  ))}
                </select>
              ))}
              {field("component", t("equipment.form.component"), (props) => (
                <select
                  {...props}
                  className={cn(
                    fieldClass,
                    errors.component && "border-destructive",
                  )}
                  title={t("equipment.form.componentHint")}
                  value={draft.component}
                  onChange={(e) =>
                    set("component", e.target.value as Component | "")
                  }
                >
                  <option value="">{t("equipment.form.noComponent")}</option>
                  {COMPONENTS.map((c) => (
                    <option key={c} value={c}>
                      {t(`equipment.components.${c}`)}
                    </option>
                  ))}
                </select>
              ))}
              {field(
                "notes",
                t("items.form.notes"),
                text("notes"),
                "col-span-2 sm:col-span-4",
              )}
              <PowersEditor
                powers={draft.powers}
                onChange={(powers) => set("powers", powers)}
                others={hasPlus(draft.category)}
                templates={draft.category === "magic"}
                error={errors.powers}
              />
            </div>
          </details>
        )}
      </div>
      <div className="flex gap-2">
        <Button type="submit" size="sm" disabled={busy}>
          {editing
            ? t("items.form.save")
            : step === 1
              ? t("items.form.next")
              : t("items.form.add")}
        </Button>
        {!editing && step === 2 && (
          <Button type="button" size="sm" onClick={() => setStep(1)}>
            {t("items.form.back")}
          </Button>
        )}
        <Button type="button" size="sm" onClick={onCancel}>
          {t("items.form.cancel")}
        </Button>
      </div>
    </form>
  );
}
