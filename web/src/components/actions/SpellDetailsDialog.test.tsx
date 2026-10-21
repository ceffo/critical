import { describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import i18next from "i18next";
import { I18nextProvider } from "react-i18next";
import { resources } from "@/i18n";
import { SpellDetailsDialog } from "./SpellDetailsDialog";

// The popup renders in a portal, which static rendering skips: swap in plain wrappers.
vi.mock("@/components/ui/dialog", () => ({
  Dialog: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  DialogContent: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  DialogTitle: ({ children }: { children: React.ReactNode }) => (
    <h2>{children}</h2>
  ),
  DialogClose: () => <button />,
}));

async function render(lng: string) {
  const inst = i18next.createInstance();
  await inst.init({
    lng,
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  return renderToStaticMarkup(
    <I18nextProvider i18n={inst}>
      <SpellDetailsDialog
        spell={{ id: "magic_missile", name: "Magic missile", level: 1 }}
        onClose={() => {}}
      />
    </I18nextProvider>,
  );
}

describe("SpellDetailsDialog while the spell loads", () => {
  it("shows the translated loading text in English, never the raw key", async () => {
    const html = await render("en");
    expect(html).toContain("Loading...");
    expect(html).not.toMatch(/>loading</);
  });

  it("shows the translated loading text in French, never the raw key", async () => {
    const html = await render("fr");
    expect(html).toContain("Chargement...");
    expect(html).not.toMatch(/>loading</);
  });
});
