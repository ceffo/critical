import type { InviteLanguage, MailOutcome } from "@/types/admin";

/** The languages an invite's mail can be in (ADR 0029 step 2.6), in the order the form offers them. */
export const INVITE_LANGUAGES: readonly InviteLanguage[] = ["en", "fr"];

/** The mail language the form starts on: the admin's UI language when mails exist in it, else English. */
export function defaultInviteLanguage(uiLanguage: string): InviteLanguage {
  const base = uiLanguage.toLowerCase().split("-")[0];
  return INVITE_LANGUAGES.find((l) => l === base) ?? "en";
}

/** The body of `POST /api/admin/invites`: the email as typed (the server normalises it) and the mail's language. */
export function invitePayload(
  email: string,
  language: InviteLanguage,
): { email: string; language: InviteLanguage } {
  return { email: email.trim(), language };
}

/** The i18n key of the hint worded for what became of an invite's mail (ADR 0029 step 2.8). */
export function mailHintKey(outcome: MailOutcome): string {
  switch (outcome) {
    case "sent":
      return "admin.mail.sent";
    case "failed":
      return "admin.mail.failed";
    case "not_configured":
      return "admin.mail.notConfigured";
  }
}
