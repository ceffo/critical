// The power_check entry's view (kinds/powerCheck.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/power_check.json";

describeKind("power_check", fixtures, {
  success: {
    en: {
      text: "Power check (Mind Thrust): d20 9 vs 13, success, PSPs −7 (INT 18 −4, +1 relearned, −2 the DM, kept up: 3 PSPs every 2 rounds, second power this round)",
      short: "Power check · success",
    },
    fr: {
      text: "Jet de pouvoir (Poussée mentale) : d20 9 contre 13, réussi, PFP −7 (INT 18 −4, +1 réappris, −2 the DM, maintenu : 3 PFP tous les 2 tours, deuxième pouvoir ce tour)",
      short: "Jet de pouvoir · réussi",
    },
    value: { text: "9", tone: "pass" },
    rolled: true,
  },
  failure: {
    en: {
      text: "Power check (Mind Thrust): d20 17 vs 14, failed, PSPs −4 (INT 18 −4, paid 4 of 7 PSPs)",
      short: "Power check · failed",
    },
    fr: {
      text: "Jet de pouvoir (Poussée mentale) : d20 17 contre 14, échec, PFP −4 (INT 18 −4, 4 PFP payés sur 7)",
      short: "Jet de pouvoir · échec",
    },
    value: { text: "17", tone: "fail" },
  },
  enteredScore: {
    en: {
      text: "Power check (Teleport): d20 11 vs 11, power score, PSPs −12 (score 11 entered)",
      short: "Power check · power score",
    },
    fr: {
      text: "Jet de pouvoir (Téléportation) : d20 11 contre 11, score de pouvoir, PFP −12 (score 11 saisi)",
      short: "Jet de pouvoir · score de pouvoir",
    },
    value: { text: "11", tone: "pass" },
  },
  // ADR 0044 step 9: the contact power on a level 7 bird, 10 miles away, resisting.
  contact: {
    en: {
      text: "Power check (Contact): d20 5 vs 7, success, PSPs −8 (WIS 15 0, on Bird, target level 7, 10 miles −3, bird −3, resisting −2, kept up: 1 PSPs every 1 rounds, contact made)",
      short: "Power check · success",
    },
    fr: {
      text: "Jet de pouvoir (Contact) : d20 5 contre 7, réussi, PFP −8 (SAG 15 0, sur Bird, cible de niveau 7, 10 milles −3, oiseau −3, résiste −2, maintenu : 1 PFP tous les 1 tours, contact établi)",
      short: "Jet de pouvoir · réussi",
    },
    value: { text: "5", tone: "pass" },
    rolled: true,
  },
  // ADR 0044 step 10: a power paid on contact, on a mind held from an earlier round.
  costIsContact: {
    en: {
      text: "Power check (Inflict Pain): d20 8 vs 12, success, PSPs 0 (CON 16 −4, on Orc, kept up: 2 PSPs every 1 rounds)",
      short: "Power check · success",
    },
    fr: {
      text: "Jet de pouvoir (Infliger la douleur) : d20 8 contre 12, réussi, PFP 0 (CON 16 −4, sur Orc, maintenu : 2 PFP tous les 1 tours)",
      short: "Jet de pouvoir · réussi",
    },
    value: { text: "8", tone: "pass" },
    rolled: true,
  },
});
