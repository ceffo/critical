// The skill_check entry's view (kinds/skillCheck.tsx) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/skill_check.json";

describeKind("skill_check", fixtures, {
  proficiency: {
    en: {
      text: "Musical instrument: d20 17 vs 16, failure",
      short: "Musical instrument · failed",
      subtitle: "Musical instrument",
      card: ['data-testid="skill-result"', "17 vs 16"],
    },
    fr: {
      text: "Instrument de musique : d20 17 contre 16, échec",
      short: "Instrument de musique · échec",
      subtitle: "Instrument de musique",
      card: ["17 contre 16"],
    },
    value: {
      text: "17",
      tone: "fail",
    },
    rolled: true,
  },
  bardAdjusted: {
    en: {
      text: "Climb walls: d100 42 vs 50 (40 +10 rope), success",
      short: "Climb walls · success",
      subtitle: "Climb walls",
      card: ['data-testid="skill-adjustment-line"', "42 vs 50%"],
    },
    fr: {
      text: "Escalade : d100 42 contre 50 (40 +10 rope), réussi",
      short: "Escalade · réussi",
      subtitle: "Escalade",
      card: ["42 contre 50"],
    },
    value: {
      text: "42",
      tone: "pass",
    },
    rolled: true,
  },
  // ADR 0037 step 20: the buff the success put on, named from the proficiency.
  tumblingBuff: {
    en: {
      text: "Tumbling: d20 12 vs 17, success; Tumbling on: AC −4, 1 rd",
      short: "Tumbling · success",
      subtitle: "Tumbling",
    },
    fr: {
      text: "Acrobatie : d20 12 contre 17, réussi ; Acrobatie actif : CA −4, 1 tr.",
      short: "Acrobatie · réussi",
      subtitle: "Acrobatie",
    },
    value: {
      text: "12",
      tone: "pass",
    },
    rolled: true,
  },
  tumblingRenewed: {
    en: {
      text: "Tumbling: d20 5 vs 17, success; Tumbling renewed: AC −4, 1 rd",
      short: "Tumbling · success",
      subtitle: "Tumbling",
    },
    fr: {
      text: "Acrobatie : d20 5 contre 17, réussi ; Acrobatie renouvelé : CA −4, 1 tr.",
      short: "Acrobatie · réussi",
      subtitle: "Acrobatie",
    },
    value: {
      text: "5",
      tone: "pass",
    },
    rolled: true,
  },
  // ADR 0037 step 20.8: the renewing success adjusted to a failure; the line in effect says no buff.
  tumblingAdjustedToFailure: {
    en: {
      text: "Tumbling: d20 15 vs 14 (17 −3), failure",
      short: "Tumbling · failed",
      subtitle: "Tumbling",
    },
    fr: {
      text: "Acrobatie : d20 15 contre 14 (17 −3), échec",
      short: "Acrobatie · échec",
      subtitle: "Acrobatie",
    },
    value: {
      text: "15",
      tone: "fail",
    },
    rolled: true,
  },
});
