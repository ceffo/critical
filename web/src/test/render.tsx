import type { ReactElement } from "react";
import { render } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import i18next from "i18next";
import { I18nextProvider } from "react-i18next";
import { resources } from "@/i18n";

export type Language = "en" | "fr";

/** A fresh i18n instance with the app's catalogues, so a test can't leak its language into another. */
export async function testI18n(lng: Language = "en") {
  const i = i18next.createInstance();
  await i.init({
    lng,
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  return i;
}

/**
 * Renders `ui` inside the app's i18n provider (real catalogues, English unless `lng` says otherwise)
 * and returns Testing Library's queries plus a `user` for typing and clicking. The zustand stores are
 * module singletons: seed them with `useXStore.setState(...)` before rendering, reset in `afterEach`.
 * Mock `@/api/*` with `vi.mock` when the component fetches.
 */
export async function renderApp(
  ui: ReactElement,
  { lng = "en" }: { lng?: Language } = {},
) {
  const i18n = await testI18n(lng);
  const user = userEvent.setup();
  const view = render(<I18nextProvider i18n={i18n}>{ui}</I18nextProvider>);
  return { user, i18n, ...view };
}
