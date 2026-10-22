/**
 * The activities of the rail, in its order (ADR 0022 step 2.4). One is focused at a time and fills
 * the stage; the header card, the character card and the dice footer stay put beside every one.
 * "powers" is a psionicist's: the rail lists it only for a sheet that carries known powers.
 */
export const ACTIVITIES = [
  "act",
  "spells",
  "powers",
  "inventory",
  "xp",
] as const;
export type Activity = (typeof ACTIVITIES)[number];
