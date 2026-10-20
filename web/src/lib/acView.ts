// The AC breakdown's rows in words (ADR 0033 step 10, ADR 0035 item 8). Display only: every number
// and every code is the server's (docs/all.md, "Core Architectural Rule").
import type { TFunction } from "i18next";
import type { AcRow, ArmourClass } from "@/types/ac";
import { formatAc, formatSigned } from "./format";

/** Names a row's source: a piece by id, a buff by its stored name and spell. */
export interface AcNamers {
  item: (id: string) => string;
  buff: (b: {
    name: string;
    spellId?: string;
    proficiencyId?: string;
  }) => string;
}

/** One row as shown: the term, its source, its value, and why it does not add (muted). */
export interface AcRowView {
  term: string;
  source: string;
  value: string;
  /** The shield's value against missiles, worded, when it differs. */
  missile: string | null;
  /** Why the row does not add; null when it adds. */
  off: string | null;
}

/** The source of a row with none: "no armour", "none". */
function noSource(t: TFunction, row: AcRow): string {
  return t(`ac.none.${row.term}`, { defaultValue: "" });
}

export function acRowView(
  t: TFunction,
  row: AcRow,
  names: AcNamers,
): AcRowView {
  const source = row.itemId
    ? names.item(row.itemId)
    : row.buff !== undefined
      ? names.buff({
          name: row.buff,
          spellId: row.spellId,
          proficiencyId: row.proficiencyId,
        })
      : row.term === "dexterity"
        ? String(row.dexterity ?? "")
        : noSource(t, row);
  const empty = !row.itemId && row.buff === undefined;
  const value =
    row.term === "base"
      ? formatAc(row.value)
      : empty && row.term !== "dexterity" && row.value === 0
        ? "—"
        : formatSigned(row.value);
  return {
    term: t(`ac.terms.${row.term}`),
    source,
    value,
    missile:
      row.missileValue !== undefined
        ? t("ac.missileValue", { value: formatSigned(row.missileValue) })
        : null,
    off: row.off ? t(`ac.off.${row.off}`) : null,
  };
}

/** The AC shown: the override while one is set (ADR 0033 step 13), else the derived AC. */
export function shownAc(ac: ArmourClass): number {
  return ac.override ? ac.override.value : ac.ac;
}
