import { useId, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { localized } from "@/i18n/localized";
import { Button } from "@/components/ui/button";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { cn } from "@/lib/utils";
import {
  changedWeaponFields,
  weaponDraftToBody,
  type WeaponDraft,
  type WeaponField,
} from "@/lib/weaponForm";
import type { ApiError } from "@/api/http";
import type { Weapon, WeaponBody } from "@/types/combat";

interface Props {
  /** Starting values. */
  initial: WeaponDraft;
  /** The weapon being edited; undefined when adding. */
  editing?: Weapon;
  /** Ammo kinds offered as suggestions. */
  ammoKinds: string[];
  /** The character's class specializes (ADR 0030 step 12): the form offers the flag. */
  specializes?: boolean;
  busy: boolean;
  /** Save a body; resolves the server's rejection, or null when saved. */
  onSave: (body: WeaponBody) => Promise<ApiError | null>;
  onCancel: () => void;
}

/**
 * Add / edit form of a weapon. Validation is the server's: its message is shown
 * under the field it names. Only number parsing happens here.
 */
export function WeaponForm({
  initial,
  editing,
  ammoKinds,
  specializes = false,
  busy,
  onSave,
  onCancel,
}: Props) {
  const { t, i18n } = useTranslation();
  const id = useId();
  const [draft, setDraft] = useState<WeaponDraft>(initial);
  const [errors, setErrors] = useState<Partial<Record<WeaponField, string>>>(
    {},
  );
  const set = <K extends WeaponField>(k: K, v: WeaponDraft[K]) => {
    setDraft((d) => ({ ...d, [k]: v }));
    setErrors((e) => ({ ...e, [k]: undefined }));
  };

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const { body, errors: parseErrors } = weaponDraftToBody(
      draft,
      i18n.language,
    );
    const local = Object.fromEntries(
      Object.entries(parseErrors).map(([k, v]) => [k, t(`items.form.${v}`)]),
    );
    if (Object.keys(local).length > 0) {
      setErrors(local);
      return;
    }
    const toSend = editing
      ? changedWeaponFields(editing, body, i18n.language)
      : body;
    if (editing && Object.keys(toSend).length === 0) {
      onCancel();
      return;
    }
    const rejected = await onSave(toSend);
    // The server's message, or the wording of its code (ADR 0023: not_allowed).
    if (rejected?.field)
      setErrors({
        [rejected.field]: t(`weapons.form.errors.${rejected.message}`, {
          defaultValue: rejected.message,
        }),
      });
  }

  const field = (
    name: WeaponField,
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
    (
      name: "name" | "damage_sm" | "damage_l" | "magic_to_hit" | "magic_damage",
      extra: object = {},
    ) =>
    (props: object) => (
      <input
        {...props}
        {...extra}
        className={cn(fieldClass, errors[name] && "border-destructive")}
        value={draft[name]}
        onChange={(e) => set(name, e.target.value)}
      />
    );

  return (
    <form
      onSubmit={(e) => void submit(e)}
      noValidate
      className="flex flex-col gap-2 rounded-lg bg-raised p-3"
      aria-label={
        editing
          ? t("weapons.form.editTitle", {
              name: localized(i18n.language, editing.name),
            })
          : t("weapons.form.addTitle")
      }
      data-testid="weapon-form"
    >
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        {field(
          "name",
          t("weapons.form.name"),
          text("name"),
          "col-span-2 sm:col-span-4",
        )}
        {field(
          "damage_sm",
          t("weapons.form.damageSm"),
          text("damage_sm", { placeholder: "1d8" }),
        )}
        {field(
          "damage_l",
          t("weapons.form.damageL"),
          text("damage_l", { placeholder: "1d12" }),
        )}
        {field(
          "magic_to_hit",
          t("weapons.form.magicToHit"),
          text("magic_to_hit", { inputMode: "numeric" }),
        )}
        {field(
          "magic_damage",
          t("weapons.form.magicDamage"),
          text("magic_damage", { inputMode: "numeric" }),
        )}
        {field("kind", t("weapons.form.kind"), (props) => (
          <select
            {...props}
            className={fieldClass}
            value={draft.kind}
            onChange={(e) => set("kind", e.target.value as "melee" | "missile")}
          >
            <option value="melee">{t("weapons.kind.melee")}</option>
            <option value="missile">{t("weapons.kind.missile")}</option>
          </select>
        ))}
        {field("two_handed", t("weapons.form.twoHanded"), (props) => (
          <input
            {...props}
            type="checkbox"
            className="size-5"
            checked={draft.two_handed}
            onChange={(e) => set("two_handed", e.target.checked)}
          />
        ))}
        {(specializes || initial.specialized) &&
          field("specialized", t("weapons.form.specialized"), (props) => (
            <input
              {...props}
              type="checkbox"
              className="size-5"
              title={t("weapons.form.specializedHint")}
              checked={draft.specialized}
              onChange={(e) => set("specialized", e.target.checked)}
            />
          ))}
        {draft.kind === "missile" &&
          field(
            "ammo_kind",
            t("weapons.form.ammoKind"),
            (props) => (
              <>
                <input
                  {...props}
                  list={`${id}-ammo`}
                  placeholder={t("weapons.form.ammoNone")}
                  className={cn(
                    fieldClass,
                    errors.ammo_kind && "border-destructive",
                  )}
                  value={draft.ammo_kind}
                  onChange={(e) => set("ammo_kind", e.target.value)}
                />
                <datalist id={`${id}-ammo`}>
                  {ammoKinds.map((k) => (
                    <option key={k} value={k} />
                  ))}
                </datalist>
              </>
            ),
            "col-span-2",
          )}
      </div>
      <div className="flex gap-2">
        <Button type="submit" size="sm" disabled={busy}>
          {editing ? t("items.form.save") : t("items.form.add")}
        </Button>
        <Button type="button" size="sm" onClick={onCancel}>
          {t("items.form.cancel")}
        </Button>
      </div>
    </form>
  );
}
