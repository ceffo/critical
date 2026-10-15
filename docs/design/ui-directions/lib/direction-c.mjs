// Direction C, "Mosaic": one dense status strip (turn, effects, HP, last result) and a grid of
// six tiles that are all live at rest: Act, Result, Character / Spell lookup, Journal,
// Experience. Any tile expands to the whole canvas (a companion tile may stay pinned beside it);
// the others collapse into a dock of one-line summaries at the bottom. No modes, no rail.
import * as d from "./data.mjs";
import * as ui from "./ui.mjs";

export const id = "c-mosaic";
export const name = "C · Mosaic: tiles that expand and collapse";
export const logo = "blade";
export const css = "direction.css";

const strip = () => `<header class="strip">
  <div class="s-brand">${ui.brand(logo)}</div>
  <div class="s-turn hstack"><span class="label">Turn</span><span class="headline">${d.turn.number}</span><span class="muted" style="margin-left:.5rem">Initiative</span><span class="tag">${d.turn.initiative}</span><span class="muted">d10</span><span class="btn ghost xs icon">${ui.icon("reroll")}</span><span class="btn primary" style="margin-left:.5rem">Next turn ${ui.icon("arrow")}</span><span class="btn icon">${ui.icon("dots")}</span></div>
  <div class="s-effects hstack"><span class="label">Effects</span>${ui.effectsInline()}<span class="btn ghost xs">${ui.icon("plus")} Add</span></div>
  <div class="s-hp"><div class="hstack" style="gap:.375rem;align-items:baseline"><span class="label">HP</span><span class="headline">${d.character.hp.current}</span><span class="muted">/ ${d.character.hp.max}</span><span class="danger right">${d.character.hp.lastDelta} ${ui.esc(d.character.hp.lastSource)}</span></div>${ui.hpBar()}</div>
  <div class="s-result"><span class="label">Last result</span>${ui.resultLine()}</div>
</header>`;

const tileHead = (title, expanded = false, extra = "") =>
  `<div class="tile-head"><span class="title">${title}</span>${extra}<span class="right btn ghost xs icon" title="${expanded ? "Back to the overview" : "Expand"}">${ui.icon(expanded ? "collapse" : "expand")}</span></div>`;

const actTile = (tab, expanded = false) =>
  `<section class="panel tile t-act ${expanded ? "expanded" : ""}">${tileHead("Act", expanded, ui.actTabs(tab))}<div class="scroll-y grow">${ui.actBody(tab, { columns: expanded ? 2 : 1 })}</div></section>`;
const resultTile = (pinned = false) =>
  `<section class="panel tile t-result ${pinned ? "pinned" : ""}">${tileHead("Result", false, pinned ? '<span class="tag">pinned</span>' : '<span class="tag outline">Latest</span>')}<div class="scroll-y grow">${ui.resultCard({ details: pinned })}</div></section>`;
const characterTile = () =>
  `<section class="panel tile t-char">${tileHead("Character", false, `<span class="muted">${ui.esc(d.character.name)} · ${d.character.klass} ${d.character.level}</span>`)}<div class="stack scroll-y grow">${ui.abilities()}<div>${ui.hpChange()}</div>${ui.chargeCard()}<div><div class="label" style="margin-bottom:.25rem">Saving throws</div>${ui.saves()}</div></div></section>`;
const lookupTile = (expanded = false) =>
  expanded
    ? `<section class="panel tile t-lookup expanded">${tileHead("Spell lookup", true)}${ui.lookupFilter()}<div class="lookup-grid"><div class="scroll-y">${ui.lookupList()}</div><div class="detail">${ui.lookupDetail({ split: true })}</div></div></section>`
    : `<section class="panel tile t-lookup">${tileHead("Spell lookup", false, `<span class="muted">${d.lookup.count} spells</span>`)}<div class="hstack" style="gap:.5rem;margin-bottom:.5rem"><span class="field grow" style="display:flex">${ui.icon("search")}&nbsp;${d.lookup.query}</span><span class="seg"><span>Mine</span><span class="on">Wizard</span><span>Priest</span><span>All</span></span></div><div class="lookup-grid"><div class="scroll-y">${ui.lookupList()}</div><div class="detail scroll-y">${ui.lookupDetail({ split: false }).replace(/<div style="margin-top:1rem">[\s\S]*$/, `<div class="muted" style="margin-top:.75rem">▸ Description: expand the tile ${ui.icon("expand", "inline")}</div>`)}</div></div></section>`;
const journalTile = () =>
  `<section class="panel tile t-journal">${tileHead("Journal", false, '<span class="seg" style="height:1.5rem"><span class="on">All</span><span>Rolls</span></span><span class="btn xs" style="margin-left:.25rem">Undo</span>')}<div class="scroll-y grow">${ui.journalList()}</div><div class="hair" style="margin:.5rem 0"></div>${ui.diceComposer({ favourites: false })}</section>`;
const xpTile = (expanded = false) =>
  expanded
    ? `<section class="panel tile t-xp expanded">${tileHead("Experience", true, `<span class="muted">${d.character.klass} · Level ${d.character.level}</span>`)}<div class="xp-grid"><div>${ui.xpProgress()}<div class="hair" style="margin:1rem 0"></div><div class="label" style="margin-bottom:.5rem">Add XP</div>${ui.xpForm()}</div><div class="stack"><div class="hstack"><span class="label">Ledger</span><span class="right muted">4 entries</span></div><div class="scroll-y grow">${ui.xpLedger()}</div></div></div></section>`
    : `<section class="panel tile t-xp">${tileHead("Experience", false, `<span class="muted">Level ${d.character.level}</span>`)}<div class="scroll-y grow">${ui.xpProgress({ hero: false })}<div class="hstack" style="margin-top:.75rem;gap:.5rem"><span class="field placeholder" style="width:6rem">Amount</span><span class="field placeholder grow" style="display:flex">Reason</span><span class="btn">${ui.icon("plus")} Add</span></div></div></section>`;

const dock = (except) => {
  const chips = {
    act: ["Act", `${ui.esc(d.weapons.inHand.name)} · AC ${d.weapons.target.ac}`],
    result: ["Result", `${d.lastResult.d20} → Hits AC ${d.lastResult.hitsAc} · ${d.lastResult.verdict.toUpperCase()} · ${d.lastResult.damageTotal}`],
    char: ["Character", `THAC0 ${d.character.thac0} · 5 saves`],
    lookup: ["Spell lookup", `${d.lookup.query} · ${d.lookup.count} spells`],
    journal: ["Journal", `${ui.esc(d.journal[1].text)} ${d.journal[1].value}`],
    xp: ["Experience", `${d.xp.pct}% to level ${d.xp.nextLevel}`],
  };
  return `<footer class="dock">${Object.entries(chips)
    .filter(([k]) => !except.includes(k))
    .map(([, [t, s]]) => `<span class="dock-chip">${ui.icon("expand")}<span class="title">${t}</span><span class="muted">${s}</span></span>`)
    .join("")}</footer>`;
};

const shell = (canvas, dockExcept = null) => `<div class="app">
${strip()}
<main class="canvas ${dockExcept ? "expanded" : "overview"}">${canvas}</main>
${dockExcept ? dock(dockExcept) : ""}
</div>`;

export const pages = {
  "act-weapons": () => shell(actTile("weapons") + resultTile() + characterTile() + lookupTile() + journalTile() + xpTile()),
  "act-spells": () => shell(actTile("spells", true) + resultTile(true), ["act", "result"]),
  lookup: () => shell(lookupTile(true), ["lookup"]),
  xp: () => shell(xpTile(true), ["xp"]),
};

export const stylesheet = `/* Direction C: a status strip, six tiles, expand + dock. */
.app { height: 100vh; display: flex; flex-direction: column; }
.strip { display: flex; align-items: center; gap: 1.25rem; padding: .5rem 1rem; background: var(--surface); border-bottom: 1px solid var(--border); min-height: 3.5rem; }
.strip > div { min-width: 0; }
.s-brand { flex: 0 0 auto; }
.s-turn { flex: 0 0 auto; }
.s-effects { flex: 1 1 auto; gap: .5rem; flex-wrap: wrap; }
.s-hp { flex: 0 0 15rem; }
.s-hp .hpscale { display: none; }
.s-result { flex: 0 0 auto; display: flex; flex-direction: column; gap: .125rem; }
@media (max-width: 1500px) { .strip { gap: .875rem; } .s-brand .wordmark { display: none; } }
.canvas { flex: 1; min-height: 0; display: grid; gap: .75rem; padding: .75rem; }
.canvas.overview { grid-template-columns: minmax(0, 2fr) minmax(0, 1fr) minmax(0, 1.1fr); grid-template-rows: 1fr 1fr; }
.canvas.expanded { grid-template-columns: 1fr; }
.canvas.expanded:has(.pinned) { grid-template-columns: 1fr 27rem; }
.tile { display: flex; flex-direction: column; min-height: 0; padding: .75rem 1rem; }
.tile .result > .hstack:last-child { flex-wrap: wrap; row-gap: .375rem; }
.tile .stat-rows > div { padding: .25rem .5rem; }
.canvas.overview { gap: .625rem; }
.tile-head { display: flex; align-items: center; gap: .75rem; margin-bottom: .5rem; }
.tile-head .tabs { margin-left: .25rem; }
.tile-head .tab { padding: .25rem .5rem; }
.tile.expanded { border-color: var(--tint); }
.weapon-card { margin-top: .625rem; border: 1px solid var(--amb-700); border-radius: .625rem; padding: .875rem 1rem; background: linear-gradient(135deg, color-mix(in oklab, var(--tint) 10%, transparent), transparent 60%); }
.tiles { display: grid; grid-template-columns: repeat(4, 1fr); gap: .5rem; margin-top: .625rem; }
.tile .tiles .tile { display: flex; flex-direction: column; gap: .125rem; padding: .5rem .625rem; border-radius: .375rem; background: var(--surface); border: 1px solid var(--border); }
.tiles .tile.on { border-color: var(--tint); }
.tiles .tile.dim { opacity: .55; }
.two-col { display: grid; grid-template-columns: 1fr 1fr; column-gap: 1.5rem; }
.lookup-grid { display: grid; grid-template-columns: 16rem 1fr; gap: 1rem; min-height: 0; flex: 1; margin-top: .5rem; }
.t-lookup.expanded .lookup-grid { grid-template-columns: 24rem 1fr; }
.lookup-grid .detail { border-left: 1px solid var(--border); padding-left: 1rem; min-height: 0; display: flex; flex-direction: column; }
.detail-split { display: grid; grid-template-columns: 22rem 1fr; gap: 1.5rem; min-height: 0; flex: 1; }
.detail-split .prose { max-width: 70ch; }
.row28.sel { background: var(--amb-900); box-shadow: inset 2px 0 0 var(--tint); }
.xp-grid { display: grid; grid-template-columns: minmax(28rem, 1fr) minmax(0, 1.2fr); gap: 1.5rem; min-height: 0; flex: 1; }
.list-row.fresh { background: color-mix(in oklab, var(--tint) 8%, transparent); }
.t-char .saves .row28 { min-height: 1.5rem; }
.t-char .stack { gap: .5rem; }
.dock { display: flex; gap: .75rem; padding: 0 .75rem .75rem; }
.dock-chip { flex: 1; display: inline-flex; align-items: center; gap: .5rem; height: 2.5rem; padding: 0 .75rem; border: 1px dashed var(--neu-600); border-radius: .5rem; background: var(--surface); min-width: 0; white-space: nowrap; }
.dock-chip .ico { color: var(--tint); flex-shrink: 0; }
.inline { display: inline; vertical-align: -3px; }
`;
