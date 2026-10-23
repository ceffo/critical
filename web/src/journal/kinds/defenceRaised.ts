// A psionicist's defence mode raised (psionicsvc, ADR 0044 steps 4 and 5): no roll, the initial
// cost paid as the entry's PSP delta. A mode kept at no cost is a standing defence. The mode is
// named by the name the entry recorded, in the player's language (its id for an entry with none).
import { ShieldCheck } from "@phosphor-icons/react";
import { formatSigned } from "@/lib/format";
import type { LocalizedText } from "@/types/localized";
import type { KindView } from "../kindView";

/** The raise as logged (psionicsvc RaiseRecord). */
export interface DefenceRaisedPayload {
  power: string;
  /** The mode's catalogue name when logged, in every language. */
  name?: LocalizedText;
  /** How the row lasts: for the round, or standing. */
  defence: "raised" | "standing";
  cost: number;
}

export const defenceRaisedKind: KindView<DefenceRaisedPayload> = {
  decode(e) {
    const p = e.payload as Partial<DefenceRaisedPayload> | null;
    return p && typeof p.power === "string" && typeof p.cost === "number"
      ? (p as DefenceRaisedPayload)
      : null;
  },
  icon: ShieldCheck,
  // "Defence raised (Mind Blank): PSPs −1", "Defence raised (Mind Blank, standing): PSPs 0".
  text: (p, { t, names }, e) =>
    t(
      p.defence === "standing"
        ? "log.defenceRaisedStanding"
        : "log.defenceRaised",
      {
        power: names.text(p.name, p.power),
        spent: formatSigned(e.pspDelta ?? 0),
      },
    ),
  rolled: () => false,
  short: (p, { t, names }, _e, value) => ({
    value,
    what: t("log.kinds.defence_raised") + " · " + names.text(p.name, p.power),
    verdict: null,
  }),
};
