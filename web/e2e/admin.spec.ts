import { test, expect, snap } from "./fixtures";
import { openDashboard, openSettings } from "./helpers";

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
  await invites.getByRole("button", { name: "Invite Cid@Example.com" }).click();
  const pending = invites.getByRole("list", { name: "Pending invites" });
  await expect(pending.getByRole("listitem")).toHaveCount(1);
  await expect(pending).toContainText("cid@example.com");
  await expect(email).toHaveValue("");
  await snap(page, "invite-pending");

  await pending
    .getByRole("button", { name: "Cancel the invite of cid@example.com" })
    .click();
  await expect(invites).toContainText("No pending invite.");
  await page.reload();
  await expect(invites).toContainText("No pending invite.");
  await snap(page, "invite-cancelled");
});
