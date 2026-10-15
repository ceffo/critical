import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import "./i18n";
import { startSettings } from "./stores/settings";
import App from "./App.tsx";

// Theme variables and language before the first React render.
startSettings();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
