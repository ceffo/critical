import { useTranslation } from "react-i18next";
import { useState } from "react";
import { PencilSimple, Plus, Trash } from "@phosphor-icons/react";
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
import type { Weapon } from "@/types/combat";

/** Armes grid: name (kind · two-handed · ammo) | damage S-M · L | to-hit / damage | edit / delete. */
const WEAPON_COLS =
  "grid grid-cols-[minmax(0,1fr)_minmax(8.5rem,auto)_5.5rem_4rem] items-center gap-x-3";

/**
 * Armes card of the Inventaire workspace: ✎ edits a weapon in place, 🗑 removes it,
 * "Add weapon" opens a form. Every change is a log entry the Combat Log can pop (the newest only).
 */
export function WeaponsSection() {
  const { t } = useTranslation();
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
    <section
      className="flex flex-col gap-1 rounded-xl bg-card px-3 py-3"
      aria-label={t("weapons.title")}
    >
      <div className="mb-1.5 flex items-center justify-between gap-3">
        <h2 className="text-title">{t("weapons.title")}</h2>
        {form !== "add" && (
          <Button size="sm" disabled={busy} onClick={() => setForm("add")}>
            <Plus /> {t("weapons.form.openAdd")}
          </Button>
        )}
      </div>
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
        <div
          className={cn(
            WEAPON_COLS,
            "px-2 pb-1 text-label text-muted-foreground",
          )}
          aria-hidden
        >
          <span>{t("weapons.columns.name")}</span>
          <span>{t("weapons.columns.damage")}</span>
          <span>{t("weapons.columns.bonus")}</span>
          <span />
        </div>
      )}
      <ul className="flex flex-col gap-1" aria-label={t("weapons.list")}>
        {weapons.map((w) =>
          form === w.id ? (
            <li key={w.id}>
              <WeaponForm
                initial={draftFromWeapon(w)}
                editing={w}
                ammoKinds={ammoKinds}
                busy={busy}
                onCancel={close}
                onSave={async (body) => {
                  const rejected = await submit(() => patchWeapon(w.id, body));
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
    </section>
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
  const { t } = useTranslation();
  return (
    <li
      className={cn(
        WEAPON_COLS,
        "min-h-7 rounded-lg bg-neu-900/40 px-2 py-1.5 text-body",
      )}
      data-testid={`weapon-${w.id}`}
    >
      <div className="flex min-w-0 flex-col">
        <span className="min-w-0 truncate font-medium">{w.name}</span>
        <span className="min-w-0 truncate text-label text-muted-foreground">
          <span data-testid="weapon-kind">{t(`weapons.kind.${w.kind}`)}</span>
          {w.two_handed ? ` · ${t("weapons.twoHanded")}` : ""}
          {w.ammo_kind ? ` · ${w.ammo_kind}` : ""}
        </span>
      </div>
      <span
        className="min-w-0 truncate tabular-nums"
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
      <div className="flex justify-end">
        <Button
          size="icon-sm"
          variant="ghost"
          className="text-muted-foreground"
          disabled={busy}
          onClick={onEdit}
          aria-label={t("weapons.form.edit", { name: w.name })}
        >
          <PencilSimple />
        </Button>
        <Button
          size="icon-sm"
          variant="ghost"
          className="text-muted-foreground"
          disabled={busy}
          onClick={onRemove}
          aria-label={t("weapons.form.remove", { name: w.name })}
        >
          <Trash />
        </Button>
      </div>
    </li>
  );
}
