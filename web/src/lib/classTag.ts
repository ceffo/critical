import type { TFunction } from "i18next";
import type { ClassEntry } from "@/types/character";

/** The class tag of a multi-class character: "Fighter 4 + Thief 5"; none for a single class. */
export function classTag(
  entries: ClassEntry[] | undefined,
  name: (code: string) => string,
  t: TFunction,
): string | undefined {
  const active = (entries ?? []).filter((e) => e.state === "active");
  if (active.length < 2) return undefined;
  return active
    .map((e) =>
      t("skillsActivity.classTag", { class: name(e.class), level: e.level }),
    )
    .join(" + ");
}
