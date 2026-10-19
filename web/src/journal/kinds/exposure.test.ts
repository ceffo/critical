// The exposure entry's view (kinds/exposure.tsx) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/exposure.json";

describeKind("exposure", fixtures, {
  onMe: {
    en: {
      text: "Cloudkill exposure, round 2: 7 poison damage to me",
      short: "Cloudkill",
      subtitle: "Cloudkill",
      card: ['data-testid="exposure-result"'],
    },
    fr: {
      text: "Cloudkill, exposition au tour 2 : 7 dégâts (poison) sur moi",
      short: "Cloudkill",
      subtitle: "Cloudkill",
      card: ['data-testid="exposure-target"'],
    },
    value: {
      text: "−7",
      tone: "damage",
    },
    rolled: true,
  },
  forDm: {
    en: {
      text: "Wall of Fire exposure, round 1: 19 fire damage · Passing through",
      short: "Wall of Fire",
      subtitle: "Wall of Fire",
      card: ['data-testid="exposure-mode"'],
    },
    fr: {
      text: "Wall of Fire, exposition au tour 1 : 19 dégâts (feu) · En la traversant",
      short: "Wall of Fire",
      subtitle: "Wall of Fire",
    },
    value: {
      text: "19",
      tone: "ink",
    },
    rolled: true,
  },
  floored: {
    en: {
      text: "Cloudkill exposure, round 3: 10 poison damage to me · −10 requested, stopped at the HP floor",
      short: "Cloudkill",
      subtitle: "Cloudkill",
      card: ['data-testid="exposure-floored"'],
    },
    fr: {
      text: "Cloudkill, exposition au tour 3 : 10 dégâts (poison) sur moi · −10 demandés, arrêtés au plancher de PV",
      short: "Cloudkill",
      subtitle: "Cloudkill",
    },
    value: {
      text: "−2",
      tone: "damage",
    },
    rolled: true,
  },
});
