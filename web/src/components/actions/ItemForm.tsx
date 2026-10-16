import { useId, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { localized } from "@/i18n/localized";
import { Button } from "@/components/ui/button";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { cn } from "@/lib/utils";
import {
  changedFields,
  draftToBody,
  type DraftErrors,
  type ItemDraft,
  type ItemField,
} from "@/lib/itemForm";
import type { ApiError } from "@/api/http";
import type { Item, ItemBehaviour, ItemBody } from "@/types/items";

const BEHAVIOURS: ItemBehaviour[] = ["stock", "charges", "daily"];
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
  /** The item being edited; undefined when adding. */
  editing?: Item;
  /** Existing item types, offered as suggestions. */
  types: string[];
  busy: boolean;
  /** Save a body; resolves the server's rejection, or null when saved. */
  onSave: (body: ItemBody) => Promise<ApiError | null>;
  onCancel: () => void;
}

/**
 * Add / edit form of an inventory item. Validation is the server's: its message
 * is shown under the field it names. Only number parsing happens here.
 */
export function ItemForm({
  initial,
  editing,
  types,
  busy,
  onSave,
  onCancel,
}: Props) {
  const { t, i18n } = useTranslation();
  const id = useId();
  const [draft, setDraft] = useState<ItemDraft>(initial);
  const [errors, setErrors] = useState<Partial<Record<ItemField, string>>>({});
  const set = <K extends ItemField>(k: K, v: ItemDraft[K]) => {
    setDraft((d) => ({ ...d, [k]: v }));
    setErrors((e) => ({ ...e, [k]: undefined }));
  };

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const { body, errors: parseErrors } = draftToBody(draft, i18n.language);
    const local = Object.fromEntries(
      Object.entries(parseErrors as DraftErrors).map(([k, v]) => [
        k,
        t(`items.form.${v}`),
      ]),
    );
    if (Object.keys(local).length > 0) {
      setErrors(local);
      return;
    }
    const toSend = editing ? changedFields(editing, body, i18n.language) : body;
    if (editing && Object.keys(toSend).length === 0) {
      onCancel();
      return;
    }
    const rejected = await onSave(toSend);
    if (rejected?.field) setErrors({ [rejected.field]: rejected.message });
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
        value={draft[name]}
        onChange={(e) => set(name, e.target.value as never)}
      />
    );

  return (
    <form
      onSubmit={(e) => void submit(e)}
      noValidate
      className="flex flex-col gap-2 rounded-lg bg-neu-900/50 p-3"
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
        {field("name", t("items.form.name"), text("name"), "col-span-2")}
        {field(
          "type",
          t("items.form.type"),
          (props) => (
            <>
              <input
                {...props}
                list={`${id}-types`}
                className={fieldClass}
                value={draft.type}
                onChange={(e) => set("type", e.target.value)}
              />
              <datalist id={`${id}-types`}>
                {types.map((ty) => (
                  <option key={ty} value={ty} />
                ))}
              </datalist>
            </>
          ),
          "col-span-2",
        )}
        {field("behaviour", t("items.form.behaviour"), (props) => (
          <select
            {...props}
            className={fieldClass}
            value={draft.behaviour}
            onChange={(e) => set("behaviour", e.target.value as ItemBehaviour)}
          >
            {BEHAVIOURS.map((b) => (
              <option key={b} value={b}>
                {t(`items.behaviour.${b}`)}
              </option>
            ))}
          </select>
        ))}
        {field(
          "count",
          t("items.form.count"),
          text("count", { inputMode: "numeric" }),
        )}
        {draft.behaviour === "daily" &&
          field(
            "maxCount",
            t("items.form.maxCount"),
            text("maxCount", { inputMode: "numeric" }),
          )}
        {field(
          "magicalBonus",
          t("items.magicalBonus"),
          text("magicalBonus", { inputMode: "numeric" }),
        )}
        {field(
          "ammoKind",
          t("items.form.ammoKind"),
          text("ammoKind", { placeholder: "arrow" }),
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
        {field(
          "notes",
          t("items.form.notes"),
          text("notes"),
          "col-span-2 sm:col-span-4",
        )}
      </div>
      <div className="flex gap-2">
        <Button type="submit" size="sm" disabled={busy}>
          {editing ? t("items.form.save") : t("items.form.add")}
        </Button>
        <Button type="button" size="sm" variant="outline" onClick={onCancel}>
          {t("items.form.cancel")}
        </Button>
      </div>
    </form>
  );
}
