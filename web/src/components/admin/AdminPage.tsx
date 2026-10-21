import { useEffect, useState, type FormEvent } from "react";
import { VersionBadge } from "@/components/shell/VersionBadge";
import { useTranslation } from "react-i18next";
import {
  ArrowLeft,
  Copy,
  EnvelopeSimple,
  PaperPlaneTilt,
  ShieldCheck,
  X,
} from "@phosphor-icons/react";
import {
  cancelInvite,
  changeUser,
  createInvite,
  fetchInvites,
  fetchUsers,
  resendInvite,
} from "@/api/admin";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { APP_NAME } from "@/lib/brand";
import { formatDateTime } from "@/lib/format";
import {
  defaultInviteLanguage,
  INVITE_LANGUAGES,
  invitePayload,
  mailHintKey,
} from "@/lib/invites";
import { cn } from "@/lib/utils";
import { useSessionStore } from "@/stores/session";
import type {
  AdminUser,
  Invite,
  InviteLanguage,
  InviteSent,
  MailOutcome,
  UserChange,
} from "@/types/admin";

const message = (e: unknown) => (e instanceof Error ? e.message : String(e));

/** The app's address, the login page an invited person opens (ADR 0029 step 2.2). */
const appLink = () => globalThis.location.origin;

/** The invite as listed: the mail outcome only matters at the moment it is answered. */
const plainInvite = (sent: InviteSent): Invite => ({
  id: sent.id,
  email: sent.email,
  invitedBy: sent.invitedBy,
  createdAt: sent.createdAt,
  language: sent.language,
});

/**
 * The invite form and the pending invites (ADR 0029 step 2): an email the browser checks as one, the
 * language of its mail, the Invite button and a hint saying what became of the mail (step 2.8), the
 * Copy button that copies the app's link (step 2.2), and each pending invite with who sent it, the
 * language of its mails, Resend and Cancel.
 */
function InvitesCard({
  invites,
  users,
  onInvited,
  onCancelled,
}: {
  invites: Invite[];
  users: AdminUser[];
  onInvited: (inv: Invite) => void;
  onCancelled: (left: Invite[]) => void;
}) {
  const { t, i18n } = useTranslation();
  const [email, setEmail] = useState("");
  const [valid, setValid] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [copy, setCopy] = useState<"copied" | "failed" | null>(null);
  const [cancelling, setCancelling] = useState<string | null>(null);
  const [resending, setResending] = useState<string | null>(null);
  const [language, setLanguage] = useState<InviteLanguage>(() =>
    defaultInviteLanguage(i18n.language),
  );
  const [sent, setSent] = useState<{
    email: string;
    mail: MailOutcome;
  } | null>(null);
  const langName = (l: InviteLanguage) => t(`admin.languageName.${l}`);
  const emailOf = (id: string) => users.find((u) => u.id === id)?.email;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!valid) {
      setError(t("admin.emailInvalid"));
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const p = invitePayload(email, language);
      const made = await createInvite(p.email, p.language);
      onInvited(plainInvite(made));
      setSent({ email: made.email, mail: made.mail });
      setEmail("");
      setValid(false);
    } catch (err) {
      setError(message(err));
    } finally {
      setBusy(false);
    }
  };

  const copyLink = async () => {
    try {
      await navigator.clipboard.writeText(appLink());
      setCopy("copied");
    } catch {
      setCopy("failed");
    }
  };

  const resend = async (inv: Invite) => {
    setResending(inv.id);
    setError(null);
    try {
      const again = await resendInvite(inv.id);
      setSent({ email: again.email, mail: again.mail });
    } catch (err) {
      setError(message(err));
    } finally {
      setResending(null);
    }
  };

  const cancel = async (inv: Invite) => {
    setCancelling(inv.id);
    setError(null);
    try {
      setSent(null);
      onCancelled(await cancelInvite(inv.id));
    } catch (err) {
      setError(message(err));
    } finally {
      setCancelling(null);
    }
  };

  return (
    <section
      aria-labelledby="admin-invites"
      className="zone-card flex min-w-0 flex-col"
      data-testid="admin-invites"
    >
      <div className="card-band flex items-center px-4 py-2">
        <h2 id="admin-invites" className="section-label">
          {t("admin.invites")}
        </h2>
      </div>
      <div className="flex flex-col gap-4 px-4 pt-3 pb-4">
        <p className="text-body text-muted-foreground">
          {t("admin.invitesLead")}
        </p>
        <form
          className="flex flex-col gap-2"
          noValidate
          onSubmit={(e) => void submit(e)}
        >
          <label
            htmlFor="admin-invite-email"
            className="text-label text-muted-foreground"
          >
            {t("admin.email")}
          </label>
          <div className="flex flex-wrap items-center gap-2">
            <Input
              id="admin-invite-email"
              type="email"
              autoComplete="off"
              className="min-w-[12rem] flex-1"
              placeholder={t("admin.emailPlaceholder")}
              value={email}
              aria-invalid={error != null && !valid ? true : undefined}
              onChange={(e) => {
                setEmail(e.target.value);
                setValid(
                  e.target.value.trim() !== "" && e.target.validity.valid,
                );
                setError(null);
              }}
            />
            <Button
              type="submit"
              variant="main"
              disabled={busy || !valid}
              aria-label={
                valid
                  ? t("admin.inviteHint", { email: email.trim() })
                  : t("admin.inviteHintEmpty")
              }
              title={
                valid
                  ? t("admin.inviteHint", { email: email.trim() })
                  : t("admin.inviteHintEmpty")
              }
            >
              <EnvelopeSimple aria-hidden />
              {t("admin.invite")}
            </Button>
          </div>
          <div
            role="group"
            aria-label={t("admin.language")}
            className="flex flex-wrap items-center gap-2"
          >
            <span className="text-label text-muted-foreground">
              {t("admin.language")}
            </span>
            {INVITE_LANGUAGES.map((l) => (
              <Button
                key={l}
                type="button"
                size="sm"
                aria-pressed={language === l}
                aria-label={t("admin.languageHint", { language: langName(l) })}
                title={t("admin.languageHint", { language: langName(l) })}
                onClick={() => setLanguage(l)}
              >
                {l.toUpperCase()}
              </Button>
            ))}
          </div>
          {error && (
            <p role="alert" className="text-body text-danger">
              {error}
            </p>
          )}
        </form>
        {sent && (
          <p
            role="status"
            data-testid="admin-mail-hint"
            data-outcome={sent.mail}
            className={cn(
              "text-body [overflow-wrap:anywhere]",
              sent.mail === "sent" ? "text-muted-foreground" : "text-danger",
            )}
          >
            {t(mailHintKey(sent.mail), { email: sent.email })}
          </p>
        )}
        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            aria-label={t("admin.copyHint", { url: appLink() })}
            title={t("admin.copyHint", { url: appLink() })}
            onClick={() => void copyLink()}
          >
            <Copy aria-hidden />
            {t("admin.copy")}
          </Button>
          {copy && (
            <span
              role="status"
              className="min-w-0 text-body text-muted-foreground [overflow-wrap:anywhere]"
            >
              {copy === "copied"
                ? t("admin.copied")
                : t("admin.copyFailed", { url: appLink() })}
            </span>
          )}
        </div>

        <div className="flex flex-col gap-2">
          <h3 className="text-label text-muted-foreground">
            {t("admin.pending")}
          </h3>
          {invites.length === 0 ? (
            <p className="text-body text-muted-foreground">
              {t("admin.noPending")}
            </p>
          ) : (
            <ul className="flex flex-col gap-1" aria-label={t("admin.pending")}>
              {invites.map((inv) => {
                const by = emailOf(inv.invitedBy);
                const date = formatDateTime(inv.createdAt, i18n.language);
                return (
                  <li
                    key={inv.id}
                    className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-md bg-raised px-3 py-2"
                  >
                    <div className="flex min-w-[10rem] flex-1 flex-col">
                      <span className="flex flex-wrap items-center gap-x-2 text-body">
                        <span className="min-w-0 [overflow-wrap:anywhere]">
                          {inv.email}
                        </span>
                        <Badge
                          title={t("admin.languageTag", {
                            language: langName(inv.language),
                          })}
                        >
                          {inv.language.toUpperCase()}
                        </Badge>
                      </span>
                      <span className="text-label text-muted-foreground [overflow-wrap:anywhere]">
                        {by
                          ? t("admin.invitedBy", { email: by, date })
                          : t("admin.invitedOn", { date })}
                      </span>
                    </div>
                    <Button
                      type="button"
                      disabled={resending === inv.id}
                      aria-label={t("admin.resendHint", { email: inv.email })}
                      title={t("admin.resendHint", { email: inv.email })}
                      onClick={() => void resend(inv)}
                    >
                      <PaperPlaneTilt aria-hidden />
                      {t("admin.resend")}
                    </Button>
                    <Button
                      type="button"
                      variant="destructive"
                      disabled={cancelling === inv.id}
                      aria-label={t("admin.cancelHint", { email: inv.email })}
                      title={t("admin.cancelHint", { email: inv.email })}
                      onClick={() => void cancel(inv)}
                    >
                      <X aria-hidden />
                      {t("admin.cancel")}
                    </Button>
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      </div>
    </section>
  );
}

/**
 * The users table (ADR 0029 step 1.3): email, role, last login, the names of their characters (never
 * the sheets), status, and per row Promote / Demote and Disable / Enable. The root admin's row has
 * no action (step 5.2).
 */
function UsersCard({
  users,
  meId,
  onChanged,
}: {
  users: AdminUser[];
  meId: string | undefined;
  onChanged: (u: AdminUser) => void;
}) {
  const { t, i18n } = useTranslation();
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const apply = async (u: AdminUser, change: UserChange) => {
    setBusy(u.id);
    setError(null);
    try {
      onChanged(await changeUser(u.id, change));
    } catch (err) {
      setError(message(err));
    } finally {
      setBusy(null);
    }
  };

  const th = "px-3 py-2 text-left font-normal text-label text-muted-foreground";
  const td = "px-3 py-2 align-top text-body";

  return (
    <section
      aria-labelledby="admin-users"
      className="zone-card flex min-w-0 flex-col"
      data-testid="admin-users"
    >
      <div className="card-band flex items-center px-4 py-2">
        <h2 id="admin-users" className="section-label">
          {t("admin.users")}
        </h2>
      </div>
      <div className="flex flex-col gap-2 px-1 pt-1 pb-3">
        {error && (
          <p role="alert" className="px-3 text-body text-danger">
            {error}
          </p>
        )}
        <table className="w-full border-separate border-spacing-0">
          <thead>
            <tr>
              <th scope="col" className={th}>
                {t("admin.col.email")}
              </th>
              <th scope="col" className={th}>
                {t("admin.col.role")}
              </th>
              <th scope="col" className={th}>
                {t("admin.col.lastLogin")}
              </th>
              <th scope="col" className={th}>
                {t("admin.col.characters")}
              </th>
              <th scope="col" className={th}>
                {t("admin.col.status")}
              </th>
              <th scope="col" className={th}>
                {t("admin.col.actions")}
              </th>
            </tr>
          </thead>
          <tbody>
            {users.map((u, i) => (
              <tr
                key={u.id}
                className={cn(i % 2 === 1 && "bg-raised")}
                data-testid="admin-user"
              >
                <th scope="row" className={cn(td, "text-left font-normal")}>
                  <span className="[overflow-wrap:anywhere]">{u.email}</span>
                  {u.id === meId && (
                    <span className="text-muted-foreground">
                      {" "}
                      ({t("admin.you")})
                    </span>
                  )}
                </th>
                <td className={td}>
                  <div className="flex flex-wrap items-center gap-1">
                    <span>{t(`admin.role.${u.role}`)}</span>
                    {u.root && (
                      <Badge title={t("admin.rootHint")}>
                        <ShieldCheck aria-hidden />
                        {t("admin.root")}
                      </Badge>
                    )}
                  </div>
                </td>
                <td className={td}>
                  {u.lastLogin
                    ? formatDateTime(u.lastLogin, i18n.language)
                    : t("admin.never")}
                </td>
                <td className={td}>
                  {u.characters.length === 0 ? (
                    <span className="text-muted-foreground">
                      {t("admin.noCharacters")}
                    </span>
                  ) : (
                    <span className="[overflow-wrap:anywhere]">
                      {u.characters.join(", ")}
                    </span>
                  )}
                </td>
                <td className={td}>
                  {u.disabled ? (
                    <span className="text-danger">{t("admin.disabled")}</span>
                  ) : (
                    t("admin.active")
                  )}
                </td>
                <td className={td}>
                  {!u.root && (
                    <div className="flex flex-wrap gap-1">
                      {u.role === "player" ? (
                        <Button
                          type="button"
                          disabled={busy === u.id}
                          aria-label={t("admin.promoteHint", {
                            email: u.email,
                          })}
                          title={t("admin.promoteHint", { email: u.email })}
                          onClick={() => void apply(u, { role: "admin" })}
                        >
                          {t("admin.promote")}
                        </Button>
                      ) : (
                        <Button
                          type="button"
                          disabled={busy === u.id}
                          aria-label={t("admin.demoteHint", { email: u.email })}
                          title={t("admin.demoteHint", { email: u.email })}
                          onClick={() => void apply(u, { role: "player" })}
                        >
                          {t("admin.demote")}
                        </Button>
                      )}
                      {u.disabled ? (
                        <Button
                          type="button"
                          disabled={busy === u.id}
                          aria-label={t("admin.enableHint", { email: u.email })}
                          title={t("admin.enableHint", { email: u.email })}
                          onClick={() => void apply(u, { disabled: false })}
                        >
                          {t("admin.enable")}
                        </Button>
                      ) : (
                        <Button
                          type="button"
                          variant="destructive"
                          disabled={busy === u.id}
                          aria-label={t("admin.disableHint", {
                            email: u.email,
                          })}
                          title={t("admin.disableHint", { email: u.email })}
                          onClick={() => void apply(u, { disabled: true })}
                        >
                          {t("admin.disable")}
                        </Button>
                      )}
                    </div>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

/**
 * The admin section (ADR 0029 step 1.3), a page of its own at /admin, reached from the cog menu by
 * admins only: the invites on the left, the users on the right. Back returns to the character sheet.
 */
export function AdminPage() {
  const { t } = useTranslation();
  const me = useSessionStore((s) => s.me);
  const [users, setUsers] = useState<AdminUser[] | null>(null);
  const [invites, setInvites] = useState<Invite[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([fetchUsers(), fetchInvites()])
      .then(([u, i]) => {
        setUsers(u);
        setInvites(i);
      })
      .catch((e: unknown) => setError(message(e)));
  }, []);
  useEffect(() => {
    document.title = `${t("admin.title")} · ${APP_NAME}`;
  }, [t]);

  return (
    <div
      className="mx-auto flex min-h-dvh w-full max-w-[120rem] flex-col gap-3 p-3"
      data-screen="admin"
    >
      <header className="zone-card flex items-center gap-3 px-4 py-2">
        <h1 className="section-label min-w-0 flex-1">{t("admin.title")}</h1>
        <VersionBadge />
        <a
          href="/"
          className={buttonVariants({ variant: "quiet" })}
          aria-label={t("admin.backHint")}
          title={t("admin.backHint")}
        >
          <ArrowLeft aria-hidden />
          {t("admin.back")}
        </a>
      </header>
      {error && (
        <p role="alert" className="zone-card px-4 py-3 text-body text-danger">
          {t("admin.loadFailed", { message: error })}
        </p>
      )}
      {users && (
        <main className="grid min-w-0 grid-cols-[minmax(20rem,26rem)_minmax(0,1fr)] items-start gap-3">
          <InvitesCard
            invites={invites}
            users={users}
            onInvited={(inv) => setInvites((is) => [...is, inv])}
            onCancelled={setInvites}
          />
          <UsersCard
            users={users}
            meId={me?.id}
            onChanged={(c) =>
              setUsers((us) => us?.map((u) => (u.id === c.id ? c : u)) ?? us)
            }
          />
        </main>
      )}
    </div>
  );
}
