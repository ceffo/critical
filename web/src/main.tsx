import { StrictMode, type ReactNode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import "./i18n";
import { startSettings } from "./stores/settings";
import App from "./App.tsx";
import { readSession } from "./api/auth";
import { LoginPage } from "./components/login/LoginPage";
import { startFor } from "./lib/session";
import { useSessionStore } from "./stores/session";

// Theme variables and language before the first React render.
startSettings();

function render(node: ReactNode) {
  createRoot(document.getElementById("root")!).render(
    <StrictMode>{node}</StrictMode>,
  );
}

// Only the login page shows without a session (ADR 0029 step 6): read it before the app's first request.
void readSession().then((session) => {
  const start = startFor(location.pathname, location.search, session);
  if ("go" in start) {
    location.replace(start.go);
    return;
  }
  if (start.show === "login") {
    render(<LoginPage error={start.error} />);
    return;
  }
  if (session.kind === "in") useSessionStore.getState().setMe(session.me);
  render(<App />);
});
