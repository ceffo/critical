// The psp_recovery entry's view (kinds/pspRecovery.tsx) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/psp_recovery.json";

describeKind("psp_recovery", fixtures, {
  resting: {
    en: {
      text: "3 h resting: PSPs recovered 18",
      short: "PSPs recovered",
      card: ['data-testid="info-value"'],
    },
    fr: {
      text: "3 h repos : PFP récupérés 18",
      short: "PFP récupérés",
    },
    value: { text: "+18", tone: "heal" },
    rolled: false,
  },
  cappedSleep: {
    en: {
      text: "8 h sleeping: PSPs recovered 10",
      short: "PSPs recovered",
      card: ["+96 requested, capped at max PSPs"],
    },
    fr: {
      text: "8 h sommeil : PFP récupérés 10",
      short: "PFP récupérés",
      card: ["+96 demandés, plafonnés aux PFP max"],
    },
    value: { text: "+10", tone: "heal" },
  },
  exertion: {
    en: {
      text: "2 h exertion: PSPs recovered 0",
      short: "PSPs recovered",
    },
    fr: {
      text: "2 h effort : PFP récupérés 0",
      short: "PFP récupérés",
    },
    value: { text: "0", tone: "heal" },
  },
  stoppedAndRested: {
    en: {
      text: "2 h resting: PSPs recovered 12 — powers ended: Aura Sight — holds ended: Grey Ogre",
      short: "PSPs recovered",
    },
    fr: {
      text: "2 h repos : PFP récupérés 12 — pouvoirs terminés : Vision de l'aura — emprises terminées : Grey Ogre",
      short: "PFP récupérés",
    },
    value: { text: "+12", tone: "heal" },
  },
});
