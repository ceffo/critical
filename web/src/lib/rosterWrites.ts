import type { CharacterList, CharacterSummary } from "@/types/character";

/** A dialog the roster opened, on one character; `token` tells two openings apart. */
export interface RosterDialog {
  kind: "rename" | "retire";
  character: CharacterSummary;
  /** The one in play: retiring it makes the server pick the next one, which the lead says. */
  current: boolean;
  token: number;
}

/** A write the roster sent (a switch, a rename or a retire), and the dialog it came from. */
export interface RosterWrite {
  id: number;
  dialog: number | null;
}

/**
 * The roster's list, its open dialog and its write in flight. One write at a time: while one is
 * pending no dialog opens and no other write starts, so an answer can neither close a dialog opened
 * after it nor replace the list with an older one.
 */
export interface RosterState {
  list: CharacterList | null;
  opened: RosterDialog | null;
  pending: RosterWrite | null;
  /** The last token or id handed out. */
  seq: number;
}

export type RosterAction =
  | { type: "loaded"; list: CharacterList }
  | {
      type: "open";
      kind: RosterDialog["kind"];
      character: CharacterSummary;
    }
  | { type: "close" }
  /** A write starts, from the open dialog when `fromDialog`; refused while one is pending. */
  | { type: "start"; fromDialog: boolean }
  | { type: "done"; id: number; list: CharacterList }
  | { type: "failed"; id: number };

export const initialRoster: RosterState = {
  list: null,
  opened: null,
  pending: null,
  seq: 0,
};

export function rosterReducer(s: RosterState, a: RosterAction): RosterState {
  switch (a.type) {
    case "loaded":
      return { ...s, list: a.list };
    case "open":
      if (s.pending) return s;
      return {
        ...s,
        seq: s.seq + 1,
        opened: {
          kind: a.kind,
          character: a.character,
          current: a.character.id === s.list?.currentId,
          token: s.seq + 1,
        },
      };
    case "close":
      return { ...s, opened: null };
    case "start":
      if (s.pending) return s;
      return {
        ...s,
        seq: s.seq + 1,
        pending: {
          id: s.seq + 1,
          dialog: a.fromDialog ? (s.opened?.token ?? null) : null,
        },
      };
    case "done": {
      if (s.pending?.id !== a.id) return s;
      const own = s.opened !== null && s.opened.token === s.pending.dialog;
      return {
        ...s,
        list: a.list,
        pending: null,
        opened: own ? null : s.opened,
      };
    }
    case "failed":
      if (s.pending?.id !== a.id) return s;
      return { ...s, pending: null };
  }
}

/** The id of the write `start` just began in `s`, or null when it was refused. */
export function startedWrite(
  before: RosterState,
  after: RosterState,
): number | null {
  return after.pending && after.pending !== before.pending
    ? after.pending.id
    : null;
}
