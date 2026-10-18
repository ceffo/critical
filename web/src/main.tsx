import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import "./i18n";
import { startSettings } from "./stores/settings";
import App from "./App.tsx";
import { ensureSession } from "./api/auth";

// Theme variables and language before the first React render.
startSettings();

// Every API call needs a session (ADR 0029): get one before the app's first request.
void ensureSession().then((ready) => {
  if (!ready) return;
  createRoot(document.getElementById("root")!).render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
});
