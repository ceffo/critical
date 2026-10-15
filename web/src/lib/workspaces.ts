/** Workspaces of the rail; the Fil and Moi columns stay beside every one of them. */
export const WORKSPACES = ["act", "inventory", "spells"] as const;
export type Workspace = (typeof WORKSPACES)[number];
