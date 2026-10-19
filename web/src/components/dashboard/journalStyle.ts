// How the journal tones an entry's value; shared by the journal and its collapsed bar. The kind's
// icon is its view's (journal registry).
import type { RowValue } from "@/journal/kindView";

export const VALUE_TONES: Record<RowValue["tone"], string> = {
  damage: "text-danger",
  heal: "text-success",
  pass: "text-success",
  fail: "text-danger",
  ink: "text-foreground",
  crit: "text-highlight",
  muted: "text-muted-foreground",
};
