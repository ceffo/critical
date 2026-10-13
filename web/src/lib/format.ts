// Display formatting only: no game rules here (docs/all.md, "Core Architectural Rule").

/** A signed integer for display: "+3", "−5" (U+2212 minus), "0". */
export function formatSigned(n: number): string {
  if (n > 0) return `+${n}`;
  if (n < 0) return `−${Math.abs(n)}`;
  return "0";
}

/** A multiplier for display: "×2", "×1.5". */
export function formatFactor(n: number): string {
  return `×${n}`;
}

/** HP bar fill as a 0–100 percentage of max, clamped. */
export function hpPercent(current: number, max: number): number {
  if (max <= 0) return 0;
  return Math.max(0, Math.min(100, (current / max) * 100));
}

/** HP bar colour per docs/frontend.md: green > 50 %, yellow 25–50 %, red < 25 %. */
export function hpTone(pct: number): "good" | "warn" | "bad" {
  if (pct > 50) return "good";
  if (pct >= 25) return "warn";
  return "bad";
}

/** HH:MM in the UI language. */
export function formatTime(iso: string, lang: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleTimeString(lang, { hour: "2-digit", minute: "2-digit" });
}
