import type { AttackForm, Weapon } from "@/types/combat";

/**
 * How the Weapons tab offers the other weapons (ADR 0027 step 7.1): the previous weapon (the one the
 * server stores in the attack form) as a one-click switch, and every other weapon in the drop-down,
 * in the served order. The previous weapon is offered only while it still exists and is not the
 * one in hand.
 */
export function weaponSwitch(
  weapons: Weapon[],
  form: Pick<AttackForm, "weaponId" | "previousWeaponId">,
): { previous: Weapon | null; others: Weapon[] } {
  const previous =
    form.previousWeaponId && form.previousWeaponId !== form.weaponId
      ? (weapons.find((w) => w.id === form.previousWeaponId) ?? null)
      : null;
  const others = weapons.filter(
    (w) => w.id !== form.weaponId && w.id !== previous?.id,
  );
  return { previous, others };
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
