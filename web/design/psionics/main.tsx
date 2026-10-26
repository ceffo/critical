// Entry of the psionics mock-ups: `?screen=<id>&lang=fr|en&theme=<id>&light=1`, painted with the
// app's own theme code (ADR 0026), dark Arcane by default.
import { createRoot } from "react-dom/client";
import "./design.css";
import { applyTheme, isDarkTheme, isLightTheme } from "@/lib/theme";
import { LANG } from "./lang";
import { SCREENS } from "./screens";

const q = new URLSearchParams(location.search);
const theme = q.get("theme") ?? "arcane";
const light = q.get("light") === "1";
applyTheme(
  light ? "light" : "dark",
  isDarkTheme(theme) ? theme : "arcane",
  isLightTheme(theme) ? theme : "arcane",
);
document.documentElement.lang = LANG;

const id = q.get("screen") ?? "act-powers";
const Screen = SCREENS[id];
const root = document.getElementById("root")!;
if (Screen) createRoot(root).render(<Screen />);
else
  createRoot(root).render(
    <ul className="p-6">
      {Object.keys(SCREENS).map((s) => (
        <li key={s}>
          <a className="text-tint underline" href={`?screen=${s}`}>
            {s}
          </a>
        </li>
      ))}
    </ul>,
  );
document.body.dataset.ready = "1";
