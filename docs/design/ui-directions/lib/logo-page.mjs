// The logo page: every mark option at header size (mark + Cinzel wordmark), at 32 and 16 px as a
// favicon, in dark and in light, next to the current mark for reference.
import { logos, logoSvg } from "./logos.mjs";
import { esc } from "./ui.mjs";

const cell = (id, theme) => `<div class="${theme} theme-cell">
  <div class="hstack" style="gap:1.25rem;align-items:flex-start"><span class="fav">${logoSvg(id, 'width="64" height="64"')}<span class="label">64</span></span><div class="brand" style="margin-top:1rem">${logoSvg(id)}<span class="wordmark">Critical<em>!</em></span></div></div>
  <div class="hstack" style="gap:1rem;margin-top:.75rem;align-items:flex-end">
    <span class="fav">${logoSvg(id, 'width="32" height="32"')}<span class="label">32</span></span>
    <span class="fav">${logoSvg(id, 'width="16" height="16"')}<span class="label">16</span></span>
    <span class="tabstrip"><span class="tabchip">${logoSvg(id, 'width="16" height="16"')}<span>Critical!</span></span></span>
  </div>
</div>`;

export const logoPage = () => `<!doctype html>
<html lang="en" class="theme-dark">
<head>
<meta charset="utf-8">
<title>Critical! logo options</title>
<link rel="stylesheet" href="../shared/tokens.css">
<link rel="stylesheet" href="../shared/base.css">
<style>
html, body { height: auto; overflow: visible; }
body { padding: 1.5rem; }
.grid { display: grid; grid-template-columns: 14rem 1fr 1fr; gap: .75rem; align-items: stretch; }
.grid > div { min-width: 0; }
.theme-cell { background: var(--ground); color: var(--ink); border: 1px solid var(--border); border-radius: .5rem; padding: 1rem; }
.theme-cell .brand svg { width: 1.75rem; height: 1.75rem; }
.fav { display: inline-flex; flex-direction: column; align-items: center; gap: .25rem; }
.tabstrip { display: inline-flex; align-items: center; background: var(--neu-900); border-radius: .5rem; padding: .375rem .5rem; }
.tabchip { display: inline-flex; align-items: center; gap: .375rem; background: var(--surface); border: 1px solid var(--border); border-radius: .375rem; padding: .25rem .625rem; font-size: .6875rem; }
.concept { padding: 1rem; }
.head { display: grid; grid-template-columns: 14rem 1fr 1fr; gap: .75rem; margin-bottom: .5rem; }
</style>
</head>
<body>
<div class="hstack" style="margin-bottom:1rem"><span class="title" style="font-size:1.125rem">Critical! mark options</span><span class="muted">at 64 px · header size · favicon 32 and 16 px · a browser tab · dark and light (Arcane)</span></div>
<div class="head"><span class="label">Option</span><span class="label">Dark</span><span class="label">Light</span></div>
<div class="grid">
${Object.entries(logos)
  .map(
    ([id, l]) =>
      `<div class="panel concept"><div class="title">${esc(l.name)}</div><p class="muted" style="margin-top:.25rem">${esc(l.concept)}</p><p class="label" style="margin-top:.5rem;text-transform:none;letter-spacing:0">logo/${id}.svg</p></div>${cell(id, "theme-dark")}${cell(id, "theme-light")}`,
  )
  .join("\n")}
</div>
</body>
</html>
`;
