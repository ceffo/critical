/**
 * The activities of the rail, in its order (ADR 0022 step 2.4). One is focused at a time and fills
 * the stage; the header card, the character card and the dice footer stay put beside every one.
 * A psionicist's powers are played in Act and browsed under Sorts (ADR 0047 step 12).
 */
export const ACTIVITIES = ["act", "spells", "inventory", "xp"] as const;
export type Activity = (typeof ACTIVITIES)[number];
