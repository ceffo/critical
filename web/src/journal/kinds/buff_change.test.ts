// The buff_change entry's view (kinds/buff.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/buff_change.json";

describeKind("buff_change", fixtures, {
  activate: {
    en: {
      text: "Activated Haste (5 rd)",
    },
    fr: {
      text: "Effet activé : Haste (5 tr.)",
    },
    value: null,
    rolled: false,
  },
  absorb: {
    en: {
      text: "Stoneskin absorbed a hit (3 left)",
    },
    fr: {
      text: "Stoneskin a absorbé un coup (reste 3)",
    },
  },
  noteEdited: {
    en: {
      text: "Bless: note “+1 morale”",
    },
    fr: {
      text: "Bless : note « +1 morale »",
    },
  },
  // ADR 0043 step 16: stopping a maintained power.
  powerStopped: {
    en: { text: "Stopped maintaining Mind Thrust" },
    fr: { text: "Maintien arrêté : Poussée mentale" },
  },
  // ADR 0044 step 3: a tangent or contact row has a target, not a power.
  letGo: {
    en: { text: "Let go of Orc" },
    fr: { text: "Emprise lâchée : Orc" },
  },
  nextRoundHolds: {
    en: {
      text: "Round 3 → 4 — expired: Goblin shaman — maintenance (PSPs): Orc 1 — holds ended, not enough PSPs: Goblin shaman",
    },
    fr: {
      text: "Tour 3 → 4 — expirés : Goblin shaman — maintien (PFP) : Orc 1 — emprises terminées, PFP insuffisants : Goblin shaman",
    },
  },
});
