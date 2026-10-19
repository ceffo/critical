// Display formatting only: no game rules here (docs/all.md, "Core Architectural Rule").

/** A signed integer for display: "+3", "−5" (U+2212 minus), "0". */
export function formatSigned(n: number): string {
  if (n > 0) return `+${n}`;
  if (n < 0) return `−${Math.abs(n)}`;
  return "0";
}

/** An armour class for display: "5", "−2" (U+2212 minus), never a plus sign. */
export function formatAc(n: number): string {
  return n < 0 ? `−${Math.abs(n)}` : String(n);
}

/** A number in the UI language: 1.5 → "1.5" (en), "1,5" (fr). */
export function formatNumber(n: number, lang?: string): string {
  return new Intl.NumberFormat(lang, { maximumFractionDigits: 4 }).format(n);
}

/** A multiplier for display: "×2", "×1.5" (en), "×1,5" (fr). */
export function formatFactor(n: number, lang?: string): string {
  return `×${formatNumber(n, lang)}`;
}

/** HP bar fill as a 0–100 percentage of max, clamped. */
export function hpPercent(current: number, max: number): number {
  if (max <= 0) return 0;
  return Math.max(0, Math.min(100, (current / max) * 100));
}

/** HP bar tone per docs/frontend.md: good > 50 %, warn 25–50 %, bad < 25 %. */
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

/** Day, month, year and HH:MM in the UI language, in the browser's time zone; "" for a bad date. */
export function formatDateTime(iso: string, lang: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleString(lang, {
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

/**
 * A Strength score in the PHB's language-neutral notation (ADR 0030 step 9): an 18 with an
 * exceptional percentile is 18/05, 18/50 or 18/00 (100); otherwise the plain score.
 */
export function formatStrength(score: number, exceptional = 0): string {
  if (exceptional <= 0) return String(score);
  if (exceptional >= 100) return `${score}/00`;
  return `${score}/${String(exceptional).padStart(2, "0")}`;
}
