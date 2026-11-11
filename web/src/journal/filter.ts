// The journal filter's values, in a module of their own: the display settings (lib/settings.ts)
// save the choice and must not import the journal's registry.

/** The journal filter: without setup rows (the default), everything, or rolls only. */
export type LogFilter = "hideSetup" | "all" | "rolls";

export const LOG_FILTERS: readonly LogFilter[] = ["hideSetup", "all", "rolls"];
