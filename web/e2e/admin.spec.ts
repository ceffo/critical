import { request as pwRequest } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import {
  chooseLanguage,
  chooseSetting,
  openDashboard,
  openSettings,
} from "./helpers";
import { expectNoClippedText } from "./noClip";

// The admin section's one flow only a browser proves (ADR 0029 steps 1.3 and 2): the cog menu's
// Admin link opens the page, an invite typed in the form shows as pending, Cancel removes it. The
// refusals, the root admin's protection and disabling are Go tests (composition admin_test.go);
// the no-cut audit in French is in layout.spec.ts.
test("admin invites an email, sees it pending, cancels it", async ({
  page,
}) => {
  await openDashboard(page);
  const settings = await openSettings(page);
  await settings.getByRole("link", { name: /^Open the admin section/ }).click();
  await expect(page).toHaveURL(/\/admin$/);

  const invites = page.getByTestId("admin-invites");
  const users = page.getByTestId("admin-users");
  await expect(users.getByTestId("admin-user")).toHaveCount(1);
  await expect(users).toContainText("Root admin");
  await expect(invites).toContainText("No pending invite.");

  const email = invites.getByRole("textbox", { name: "Email address" });
  await email.fill("not-an-email");
  await expect(
    invites.getByRole("button", { name: "Invite this email address" }),
  ).toBeDisabled();
  await email.fill("Cid@Example.com");
  // The mail's language starts on the UI's (English); French is chosen and kept as the invite's tag.
  const en = invites.getByRole("button", {
    name: "Send the invite email in English",
  });
  await expect(en).toHaveAttribute("aria-pressed", "true");
  await invites
    .getByRole("button", { name: "Send the invite email in French" })
    .click();
  await invites.getByRole("button", { name: "Invite Cid@Example.com" }).click();
  const pending = invites.getByRole("list", { name: "Pending invites" });
  await expect(pending.getByRole("listitem")).toHaveCount(1);
  await expect(pending).toContainText("cid@example.com");
  await expect(pending).toContainText("FR");
  await expect(email).toHaveValue("");
  // The e2e server mails nothing for real (its sender is a stub that succeeds): the hint says sent, and Resend answers the same way.
  const hint = invites.getByTestId("admin-mail-hint");
  await expect(hint).toHaveText("Invitation sent to cid@example.com.");
  await snap(page, "invite-pending");
  await pending
    .getByRole("button", {
      name: "Send the invite email to cid@example.com again",
    })
    .click();
  await expect(hint).toHaveAttribute("data-outcome", "sent");
  await snap(page, "invite-resent");

  await pending
    .getByRole("button", { name: "Cancel the invite of cid@example.com" })
    .click();
  await expect(invites).toContainText("No pending invite.");
  await page.reload();
  await expect(invites).toContainText("No pending invite.");
  await snap(page, "invite-cancelled");
});

interface Account {
  id: string;
  email: string;
  role: string;
  root: boolean;
  disabled: boolean;
}

// The admin section's refusals and edge cases through the API as a player and as the root admin
// (ADR 0029 steps 1.3, 2, 4.4, 5.2): every admin route and the crit table are admin_only to a
// player; invites are lower-cased, a duplicate or an existing user is refused; the root admin
// cannot be changed, whatever the fields, and a refused change writes nothing; a cancelled invite
// cannot be cancelled again.
test("admin API: player refused, invite edge cases, root admin protected", async ({
  stack,
}) => {
  const root = await pwRequest.newContext({
    baseURL: stack.apiUrl,
    extraHTTPHeaders: {
      Cookie: `${stack.session.name}=${stack.session.value}`,
    },
  });
  const player = await pwRequest.newContext({ baseURL: stack.apiUrl });
  const users = async () =>
    (
      (await (await root.get("/api/admin/users")).json()) as {
        users: Account[];
      }
    ).users;
  const rootUser = (await users()).find((u) => u.root)!;

  // 1. A player: every admin route and the crit table are 403 admin_only.
  const pl = await (
    await player.post("/api/auth/dev", { data: { email: "pat@example.com" } })
  ).json();
  expect(
    (
      await root.patch(`/api/admin/users/${pl.id}`, {
        data: { role: "player" },
      })
    ).ok(),
  ).toBe(true);
  const refused = [
    await player.get("/api/admin/users"),
    await player.patch(`/api/admin/users/${rootUser.id}`, {
      data: { disabled: true },
    }),
    await player.get("/api/admin/invites"),
    await player.post("/api/admin/invites", {
      data: { email: "x@example.com" },
    }),
    await player.delete(`/api/admin/invites/${rootUser.id}`),
    await player.put("/api/ruleset/crit-table", { data: [] }),
  ];
  for (const r of refused) {
    expect(r.status(), r.url()).toBe(403);
    expect((await r.json()).code).toBe("admin_only");
  }

  // 2. Invites: lower-cased and trimmed, a duplicate and an existing user refused.
  const inv = await root.post("/api/admin/invites", {
    data: { email: "  Foo@Bar.com " },
  });
  expect(inv.status()).toBe(201);
  const invite = await inv.json();
  expect(invite.email).toBe("foo@bar.com");
  const dup = await root.post("/api/admin/invites", {
    data: { email: "FOO@bar.com" },
  });
  expect(dup.status()).toBe(409);
  expect((await dup.json()).code).toBe("already_invited");
  const has = await root.post("/api/admin/invites", {
    data: { email: "Pat@Example.com" },
  });
  expect(has.status()).toBe(409);
  expect((await has.json()).code).toBe("email_has_user");

  // 5. Cancel twice: the second is 404 invite_not_found.
  expect((await root.delete(`/api/admin/invites/${invite.id}`)).ok()).toBe(
    true,
  );
  const again = await root.delete(`/api/admin/invites/${invite.id}`);
  expect(again.status()).toBe(404);
  expect((await again.json()).code).toBe("invite_not_found");

  // 3. The root admin: each field alone, both together, and nothing: refused, nothing written.
  const before = JSON.stringify(await users());
  for (const [data, status] of [
    [{ disabled: true, role: "player" }, 409],
    [{ disabled: true }, 409],
    [{ role: "player" }, 409],
    [{}, 400],
  ] as const) {
    const r = await root.patch(`/api/admin/users/${rootUser.id}`, { data });
    expect(r.status(), JSON.stringify(data)).toBe(status);
  }
  expect(JSON.stringify(await users())).toBe(before);
  expect((await root.get("/api/admin/users")).ok()).toBe(true);
});

// A player's browser: no Admin link in the cog menu, the crit table editor hidden. A fresh player
// has no character, so the root admin's sheet is shown with /api/auth/me answering role "player"
// (the API's refusal of a real player is the test above).
test("a player sees no Admin link and no crit table editor", async ({
  page,
}) => {
  await page.route("**/api/auth/me", async (route) => {
    const res = await route.fetch();
    await route.fulfill({
      response: res,
      json: { ...(await res.json()), role: "player" },
    });
  });
  await openDashboard(page);
  await page.getByRole("tab", { name: "Weapons" }).click();
  await expect(
    page.getByRole("button", { name: /Attack/ }).first(),
  ).toBeVisible();
  await expect(page.getByTestId("crit-table")).toHaveCount(0);
  const settings = await openSettings(page);
  await expect(settings.getByTestId("settings-account")).toBeVisible();
  await expect(
    settings.getByRole("link", { name: /^Open the admin section/ }),
  ).toHaveCount(0);
});

// Disabling a logged-in user ends their session at once: their next request is 401 and the UI
// says so; enabled again, they log in again. The cancelled invite's row leaves without a flash.
test("admin disables a logged-in user: session ends, enable, log in again", async ({
  browser,
  page,
  stack,
}) => {
  const player = await browser.newContext();
  const pp = await player.newPage();
  const login = await player.request.post(`${stack.webUrl}/api/auth/dev`, {
    data: { email: "dan@example.com" },
  });
  expect(login.ok()).toBe(true);
  // Dan is a second admin, with the admin page open.
  await pp.goto("/admin");
  await expect(pp.getByTestId("admin-user")).toHaveCount(2);
  const invite = pp.getByRole("textbox", { name: "Email address" });
  await invite.fill("eve@example.com");

  await openDashboard(page);
  await page.goto("/admin");
  const row = page
    .getByTestId("admin-user")
    .filter({ hasText: "dan@example.com" });
  await row.getByRole("button", { name: /^Disable dan@example.com/ }).click();
  await expect(row).toContainText("Disabled");

  const next = await player.request.get(`${stack.webUrl}/api/characters`);
  expect(next.status()).toBe(401);
  // The open page's next request ends the session visibly.
  await pp.getByRole("button", { name: "Invite eve@example.com" }).click();
  await expect(pp).toHaveURL(/\/login/);
  await expect(pp.getByTestId("login-error")).toContainText(
    "session has ended",
  );
  await snap(pp, "session-ended");

  await row.getByRole("button", { name: /^Enable dan@example.com/ }).click();
  await expect(row).not.toContainText("Disabled");
  const again = await player.request.post(`${stack.webUrl}/api/auth/dev`, {
    data: { email: "dan@example.com" },
  });
  expect(again.ok()).toBe(true);
  await pp.goto("/");
  await expect(pp).not.toHaveURL(/\/login/);
  await player.close();
});

// /admin in the dark theme, French and English, 1280 and 2560, with long emails and many names.
test("admin page in dark: no cut text, fr and en, 1280 and 2560", async ({
  page,
  stack,
}) => {
  test.setTimeout(60_000);
  const api = async (method: string, url: string, body: unknown) => {
    const r = await fetch(`${stack.apiUrl}${url}`, {
      method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    expect(r.ok, `${method} ${url}: ${r.status}`).toBe(true);
    return (await r.json()) as { id: string };
  };
  const long = await api("POST", "/api/auth/dev", {
    email:
      "aurelie.de-la-tour-dauvergne@compagnie-des-lames-et-des-arcanes.example.com",
  });
  await api("PATCH", `/api/admin/users/${long.id}`, { role: "player" });
  await api("POST", "/api/admin/invites", {
    email:
      "maximilien.vandenbroucke-lefebvre@guilde-des-arcanistes.example.org",
  });

  await page.setViewportSize({ width: 1280, height: 800 });
  await openDashboard(page);
  await chooseSetting(page, "Dark");
  for (const lang of ["fr", "en"] as const) {
    await chooseLanguage(page, lang);
    await page.goto("/admin");
    await expect(page.getByTestId("admin-user")).toHaveCount(2);
    expect(
      await page.evaluate(() =>
        document.documentElement.classList.contains("dark"),
      ),
    ).toBe(true);
    for (const vp of [
      { width: 1280, height: 800 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      const where = `dark, ${lang}, ${vp.width}x${vp.height}`;
      await expectNoClippedText(
        page.getByTestId("admin-invites"),
        `invites, ${where}`,
      );
      await expectNoClippedText(
        page.getByTestId("admin-users"),
        `users, ${where}`,
      );
      await snap(page, `admin-dark-${lang}-${vp.width}x${vp.height}`);
    }
    await page.goto("/");
  }
});
