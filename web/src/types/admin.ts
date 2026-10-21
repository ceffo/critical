/** A user in the admin section (`GET /api/admin/users`, ADR 0029 step 1.3): their characters' names, never the sheets. */
export interface AdminUser {
  id: string;
  email: string;
  role: "admin" | "player";
  /** The root admin: never disabled nor demoted (step 5.2). */
  root: boolean;
  disabled: boolean;
  /** RFC 3339, UTC; null before the first login. */
  lastLogin: string | null;
  createdAt: string;
  /** The names of their characters not retired, in creation order. */
  characters: string[];
}

/** The language of an invite's mails (step 2.6). */
export type InviteLanguage = "en" | "fr";

/** A pending invite (`GET /api/admin/invites`, step 2.1): the email, who invited (a user id), when, and the language of its mails. */
export interface Invite {
  id: string;
  email: string;
  invitedBy: string;
  createdAt: string;
  language: InviteLanguage;
}

/** What became of an invite's mail (step 2.8); the invite exists whichever it is. */
export type MailOutcome = "sent" | "failed" | "not_configured";

/** The answer of creating or resending an invite: the invite and the outcome of its mail. */
export interface InviteSent extends Invite {
  mail: MailOutcome;
}

/** An admin's change to a user (`PATCH /api/admin/users/{id}`): a field left out stays as it is. */
export interface UserChange {
  disabled?: boolean;
  role?: "admin" | "player";
}
