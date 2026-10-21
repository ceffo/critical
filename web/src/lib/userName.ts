/**
 * What the app calls a user: the name Google gave (ADR 0029 step 3.8), else the email
 * (a user who has not logged in since the name was kept, or the dev login).
 */
export function userName(u: { displayName?: string }): string | undefined {
  return u.displayName?.trim() || undefined;
}

export function userLabel(u: { displayName?: string; email: string }): string {
  return userName(u) ?? u.email;
}
