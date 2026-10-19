import { createElement } from "react";
import type { LogEntry } from "@/types/log";
import { entryIcon } from "./entry";

/** The entry kind's icon (its view's), decorative: the kind's label goes beside it. */
export function EntryIcon({
  entry,
  className,
}: {
  entry: LogEntry;
  className?: string;
}) {
  return createElement(entryIcon(entry), { "aria-hidden": true, className });
}
