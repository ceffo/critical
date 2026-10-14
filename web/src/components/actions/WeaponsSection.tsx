import { useTranslation } from "react-i18next";
import { useState } from "react";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { useItemsStore } from "@/stores/items";
import { useTableStore } from "@/stores/table";
import { deleteWeapon, patchWeapon, postCreateWeapon } from "@/api/combat";
import {
  ammoKindSuggestions,
  draftFromWeapon,
  emptyWeaponDraft,
} from "@/lib/weaponForm";
import { formatSigned } from "@/lib/format";
import { WeaponForm } from "./WeaponForm";
import type { Weapon } from "@/types/combat";

/**
 * Weapons list of the Inventory tab (critical-qki.32): ✎ edits a weapon in place, 🗑 removes it,
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
      className="flex max-w-2xl flex-col gap-2"
      aria-label={t("weapons.title")}
    >
      <h3 className="text-sm font-semibold">{t("weapons.title")}</h3>
      {form === "add" ? (
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
      ) : (
        <div>
          <Button
            size="sm"
            variant="outline"
            disabled={busy}
            onClick={() => setForm("add")}
          >
            <Plus /> {t("weapons.form.openAdd")}
          </Button>
        </div>
      )}
      {weapons.length === 0 && (
        <p className="text-sm text-muted-foreground">{t("weapons.none")}</p>
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
      className="flex items-center gap-2 rounded-lg border px-2.5 py-1.5 text-sm"
      data-testid={`weapon-${w.id}`}
    >
      <div className="flex min-w-0 flex-1 flex-col">
        <span className="min-w-0 truncate font-medium">{w.name}</span>
        <span
          className="text-xs text-muted-foreground"
          data-testid="weapon-detail"
        >
          {t("weapons.detail", { sm: w.damage_sm, l: w.damage_l })}
          {w.two_handed ? ` · ${t("weapons.twoHanded")}` : ""}
          {w.ammo_kind ? ` · ${w.ammo_kind}` : ""}
        </span>
      </div>
      <Badge
        variant="outline"
        title={t("weapons.form.magicToHit")}
        data-testid="weapon-to-hit"
      >
        {formatSigned(w.magic_to_hit)}
      </Badge>
      <Badge
        variant="outline"
        title={t("weapons.form.magicDamage")}
        data-testid="weapon-damage"
      >
        {formatSigned(w.magic_damage)}
      </Badge>
      <Badge variant="secondary" data-testid="weapon-kind">
        {t(`weapons.kind.${w.kind}`)}
      </Badge>
      <Button
        size="icon-sm"
        variant="ghost"
        disabled={busy}
        onClick={onEdit}
        aria-label={t("weapons.form.edit", { name: w.name })}
      >
        <Pencil />
      </Button>
      <Button
        size="icon-sm"
        variant="ghost"
        disabled={busy}
        onClick={onRemove}
        aria-label={t("weapons.form.remove", { name: w.name })}
      >
        <Trash2 />
      </Button>
    </li>
  );
}
