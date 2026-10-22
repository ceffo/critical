// The psp_change entry's view (kinds/pspChange.tsx) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/psp_change.json";

describeKind("psp_change", fixtures, {
  spent: {
    en: {
      text: "mind thrust",
      short: "PSPs spent",
      card: ['data-testid="info-value"', "mind thrust"],
    },
    fr: {
      text: "mind thrust",
      short: "PFP dépensés",
    },
    value: {
      text: "−7",
      tone: "damage",
    },
    rolled: false,
  },
  cappedGain: {
    en: {
      text: "PSPs recovered 30",
      short: "PSPs recovered",
      card: ["+50 requested, capped at max PSPs"],
    },
    fr: {
      text: "PFP récupérés 30",
      short: "PFP récupérés",
      card: ["+50 demandés, plafonnés aux PFP max"],
    },
    value: {
      text: "+30",
      tone: "heal",
    },
  },
  flooredLoss: {
    en: {
      text: "PSPs spent 4",
      short: "PSPs spent",
      card: ["−9 requested, stopped at 0 PSPs"],
    },
    fr: {
      text: "PFP dépensés 4",
      short: "PFP dépensés",
      card: ["−9 demandés, arrêtés à 0 PFP"],
    },
    value: {
      text: "−4",
      tone: "damage",
    },
  },
});
