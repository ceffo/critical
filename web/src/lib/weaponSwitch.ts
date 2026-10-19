import type { AttackForm, Weapon } from "@/types/combat";

/**
 * How the Weapons tab offers the other weapons (ADR 0027 step 7.1, ADR 0035 item 12): the previous
 * weapon (the one the server stores in the attack form) as a one-click switch, and every other
 * weapon in the drop-down, the equipped ones first, then the others (picking one of those equips
 * it, on the server), each group in the served order. The previous weapon is offered only while it
 * still exists, is equipped and is not the one in hand.
 */
export function weaponSwitch(
  weapons: Weapon[],
  form: Pick<AttackForm, "weaponId" | "previousWeaponId">,
): { previous: Weapon | null; equipped: Weapon[]; unequipped: Weapon[] } {
  const found = form.previousWeaponId
    ? weapons.find((w) => w.id === form.previousWeaponId)
    : undefined;
  const previous =
    found && found.wielded && found.id !== form.weaponId ? found : null;
  const others = weapons.filter(
    (w) => w.id !== form.weaponId && w.id !== previous?.id,
  );
  return {
    previous,
    equipped: others.filter((w) => w.wielded),
    unequipped: others.filter((w) => !w.wielded),
  };
}

/**
 * The target modifier that is set behind the toggle (ADR 0027 step 1.5): a situational modifier
 * other than 0. It shows as a tag on the target bar while it is set. The size is not one: its chips
 * are always on the line.
 */
export function setModifiers(form: Pick<AttackForm, "situational">): {
  situational: { value: number; label: string } | null;
} {
  return {
    situational: form.situational.value !== 0 ? form.situational : null,
  };
}
