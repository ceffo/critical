import { useTranslation } from "react-i18next";
import { localized } from "@/i18n/localized";
import { useState } from "react";
import { PencilSimple, Trash } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useItemsStore } from "@/stores/items";
import { useTableStore } from "@/stores/table";
import { deleteWeapon, patchWeapon, postCreateWeapon } from "@/api/combat";
import {
  ammoKindSuggestions,
  draftFromWeapon,
  emptyWeaponDraft,
} from "@/lib/weaponForm";
import { formatSigned } from "@/lib/format";
import { cn } from "@/lib/utils";
import { WeaponForm } from "./WeaponForm";
import { InventoryCard, ROW_ACTIONS } from "./InventoryCard";
import type { Weapon } from "@/types/combat";

/**
 * Armes grid, built like the Objets one (ADR 0027 step 14.1): name (kind · two-handed · ammo) |
 * damage S-M · L | to-hit / damage | spare width | edit / delete on hover; the header and the rows
 * share the columns through `subgrid`.
 */
const WEAPON_COLS =
  "grid grid-cols-[minmax(8rem,max-content)_auto_auto_minmax(0,1fr)_auto] items-center gap-x-3";
const WEAPON_ROW = "col-span-full grid grid-cols-subgrid items-center";

/**
 * Armes card of the Inventaire workspace: ✎ edits a weapon in place, 🗑 removes it,
 * "Add" opens a form. Every change is a log entry the Combat Log can pop (the newest only).
 */
export function WeaponsSection() {
  const { t, i18n } = useTranslation();
  const weapons = useItemsStore((s) => s.weapons);
  const items = useItemsStore((s) => s.items);
  const run = useTableStore((s) => s.run);
  const submit = useTableStore((s) => s.submit);
  const busy = useTableStore((s) => s.busy);
  // "add", a weapon id being edited, or nothing.
  const [form, setForm] = useState<"add" | string | null>(null);
  const ammoKinds = ammoKindSuggestions(
    weapons,
    items.map((i) => i.ammoKind),
  );
  const close = () => setForm(null);

  return (
    <InventoryCard
      title={t("weapons.title")}
      label={t("weapons.title")}
      addLabel={t("weapons.form.openAdd")}
      showAdd={form !== "add"}
      busy={busy}
      onAdd={() => setForm("add")}
    >
      {form === "add" && (
        <div className="mb-2">
          <WeaponForm
            initial={emptyWeaponDraft}
            ammoKinds={ammoKinds}
            busy={busy}
            onCancel={close}
            onSave={async (body) => {
              const rejected = await submit(() => postCreateWeapon(body));
              if (!rejected) close();
              return rejected;
            }}
          />
        </div>
      )}
      {weapons.length === 0 ? (
        <p className="text-body text-muted-foreground">{t("weapons.none")}</p>
      ) : (
        <div className={WEAPON_COLS}>
          <div
            className={cn(
              WEAPON_ROW,
              "px-2 pb-1 text-label text-muted-foreground",
            )}
            aria-hidden
          >
            <span>{t("weapons.columns.name")}</span>
            <span>{t("weapons.columns.damage")}</span>
            <span>{t("weapons.columns.bonus")}</span>
          </div>
          <ul
            className={cn(WEAPON_ROW, "gap-y-1")}
            aria-label={t("weapons.list")}
          >
            {weapons.map((w) =>
              form === w.id ? (
                <li key={w.id} className="col-span-full">
                  <WeaponForm
                    initial={draftFromWeapon(w, i18n.language)}
                    editing={w}
                    ammoKinds={ammoKinds}
                    busy={busy}
                    onCancel={close}
                    onSave={async (body) => {
                      const rejected = await submit(() =>
                        patchWeapon(w.id, body),
                      );
                      if (!rejected) close();
                      return rejected;
                    }}
                  />
                </li>
              ) : (
                <WeaponRow
                  key={w.id}
                  weapon={w}
                  busy={busy}
                  onEdit={() => setForm(w.id)}
                  onRemove={() => void run(() => deleteWeapon(w.id))}
                />
              ),
            )}
          </ul>
        </div>
      )}
    </InventoryCard>
  );
}

function WeaponRow({
  weapon: w,
  busy,
  onEdit,
  onRemove,
}: {
  weapon: Weapon;
  busy: boolean;
  onEdit: () => void;
  onRemove: () => void;
}) {
  const { t, i18n } = useTranslation();
  return (
    <li
      className={cn(
        WEAPON_ROW,
        "group/row min-h-7 rounded-lg bg-raised px-2 py-1.5 text-body",
      )}
      data-testid={`weapon-${w.id}`}
    >
      <div className="flex min-w-0 flex-col">
        <span className="min-w-0 break-words font-medium">
          {localized(i18n.language, w.name)}
        </span>
        <span className="min-w-0 break-words text-label text-muted-foreground">
          <span data-testid="weapon-kind">{t(`weapons.kind.${w.kind}`)}</span>
          {w.two_handed ? ` · ${t("weapons.twoHanded")}` : ""}
          {w.ammo_kind ? ` · ${w.ammo_kind}` : ""}
        </span>
      </div>
      <span
        className="min-w-0 break-words tabular-nums"
        data-testid="weapon-detail"
      >
        {t("weapons.detail", { sm: w.damage_sm, l: w.damage_l })}
      </span>
      <span className="tabular-nums">
        <span title={t("weapons.form.magicToHit")} data-testid="weapon-to-hit">
          {formatSigned(w.magic_to_hit)}
        </span>
        <span className="text-muted-foreground"> / </span>
        <span title={t("weapons.form.magicDamage")} data-testid="weapon-damage">
          {formatSigned(w.magic_damage)}
        </span>
      </span>
      <span aria-hidden />
      <div className={ROW_ACTIONS}>
        <Button
          size="icon-sm"
          variant="ghost"
          className="text-muted-foreground"
          disabled={busy}
          onClick={onEdit}
          aria-label={t("weapons.form.edit", {
            name: localized(i18n.language, w.name),
          })}
        >
          <PencilSimple />
        </Button>
        <Button
          size="icon-sm"
          variant="ghost"
          className="text-muted-foreground"
          disabled={busy}
          onClick={onRemove}
          aria-label={t("weapons.form.remove", {
            name: localized(i18n.language, w.name),
          })}
        >
          <Trash />
        </Button>
      </div>
    </li>
  );
}
