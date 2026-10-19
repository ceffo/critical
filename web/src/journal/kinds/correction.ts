// The correction entry (actionlog.CorrectionStep, ADR 0010 step 6): a correction made after a roll
// (target AC, save, skill or ability adjustment, the DM's verdict), its own step on the action stack.
import type { TFunction } from "i18next";
import { PencilSimple } from "@phosphor-icons/react";
import { formatSigned } from "@/lib/format";
import type { LogEntry, LogKind } from "@/types/log";
import type { KindView } from "../kindView";

/** from/to are the field's value before and after (null: an unknown AC, no verdict). */
export interface CorrectionLogPayload {
  entryId: number;
  entryKind: LogKind;
  field:
    | "targetAc"
    | "saveAdjustment"
    | "skillAdjustment"
    | "abilityAdjustment"
    | "verdict"
    | string;
  from: number | string | null;
  to: number | string | null;
}

export function decodeCorrection(e: LogEntry): CorrectionLogPayload | null {
  const p = e.payload as Partial<CorrectionLogPayload> | null;
  return p && typeof p.entryId === "number" && typeof p.field === "string"
    ? (p as CorrectionLogPayload)
    : null;
}

function correctionValue(
  field: string,
  v: number | string | null,
  t: TFunction,
): string {
  switch (field) {
    case "targetAc":
      return typeof v === "number" ? String(v) : t("log.correction.unknown");
    case "saveAdjustment":
    case "skillAdjustment":
    case "abilityAdjustment":
      return typeof v === "number" ? formatSigned(v) : t("log.correction.none");
    case "verdict":
      return v === "hit" || v === "miss"
        ? t(`result.${v}`)
        : t("log.correction.none");
    default:
      return v === null ? t("log.correction.none") : String(v);
  }
}

const FIELDS = [
  "targetAc",
  "saveAdjustment",
  "skillAdjustment",
  "abilityAdjustment",
  "verdict",
];

export const correctionKind: KindView<CorrectionLogPayload> = {
  decode: decodeCorrection,
  icon: PencilSimple,
  text(p, { t }) {
    if (!FIELDS.includes(p.field))
      return t("log.correction.other", { id: p.entryId });
    return t(`log.correction.${p.field}`, {
      from: correctionValue(p.field, p.from, t),
      to: correctionValue(p.field, p.to, t),
    });
  },
};
