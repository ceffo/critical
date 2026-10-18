import { del, patch, post, request } from "./http";
import type { AdminUser, Invite, UserChange } from "@/types/admin";

/** Every user, with last login and their characters' names. */
export async function fetchUsers(): Promise<AdminUser[]> {
  return (await request<{ users: AdminUser[] }>("/api/admin/users")).users;
}

/** The pending invites, oldest first. */
export async function fetchInvites(): Promise<Invite[]> {
  return (await request<{ invites: Invite[] }>("/api/admin/invites")).invites;
}

/** Invite an email (step 2.1): refused when it already has a user or a pending invite. */
export function createInvite(email: string): Promise<Invite> {
  return post<Invite>("/api/admin/invites", { email });
}

/** Cancel a pending invite (step 2.3); the answer is the pending invites left. */
export async function cancelInvite(id: string): Promise<Invite[]> {
  return (
    await del<{ invites: Invite[] }>(
      `/api/admin/invites/${encodeURIComponent(id)}`,
    )
  ).invites;
}

/** Disable / re-enable a user or change their role; the answer is the user as changed. */
export function changeUser(id: string, change: UserChange): Promise<AdminUser> {
  return patch<AdminUser>(`/api/admin/users/${encodeURIComponent(id)}`, change);
}
