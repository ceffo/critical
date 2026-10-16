// Direction A3, "Now and history": the vitals bar is one 44 px strip (brand, the activities as
// tabs instead of a rail, turn + Next turn, effect pills, HP, last result). The Act stage is two
// bands: "now" (the act panel, the result with its calculation, the character side by side) and
// "history" (the journal as a full-width table with the dice composer as one line under it). The
// lookup is a table with the stats as columns beside the detail; XP is the ledger as the hero
// with the total, the thresholds and the form in a side column.
import * as d from "./data.mjs";
import * as ui from "./ui.mjs";

export const id = "a3-now-history";
export const name = "A3 · Now and history";
export const logo = "facet-bare";
export const css = "direction.css";

const TABS = [
  ["act", "Act", "fist"],
  ["spells", "Spells", "book"],
  ["xp", "Experience", "trend"],
  ["inventory", "Inventory", "bag"],
];

const strip = (active) => `<header class="strip">
  <span class="v-brand">${ui.brand(logo)}</span>
  ${ui.activityTabs(active, TABS)}
  <span class="v-turn hstack"><span class="label">Turn</span><span class="key">${d.turn.number}</span><span class="muted">Init</span><span class="tag">${d.turn.initiative}</span><span class="btn ghost xs icon">${ui.icon("reroll")}</span><span class="btn primary" style="height:1.75rem">Next turn ${ui.icon("arrow")}</span><span class="btn icon">${ui.icon("dots")}</span></span>
  <span class="v-effects hstack"><span class="label">Effects</span>${ui.effectsInline({ compact: true })}<span class="btn ghost xs icon">${ui.icon("plus")}</span></span>
  <span class="v-hp hstack"><span class="label">HP</span><span class="key">${d.character.hp.current}</span><span class="muted">/ ${d.character.hp.max}</span><span class="mini-hp">${ui.hpBar()}</span><span class="danger">${d.character.hp.lastDelta}</span></span>
  <span class="v-result hstack">${ui.resultLine()}</span>
  <span class="btn ghost xs icon" style="margin-left:auto">${ui.icon("gear")}</span>
</header>`;

const shell = (active, stage) => `<div class="app">
${strip(active)}
<main class="stage stage-${active}">${stage}</main>
</div>`;

const act = (tab) =>
  shell(
    "act",
    `<div class="now">
  <section class="panel act"><div class="panel-head">${ui.actTabs(tab)}</div><div class="scroll-y grow">${ui.actBody(tab, { columns: 2 })}</div></section>
  <section class="panel result-panel scroll-y"><div class="panel-head"><span class="label">Result</span><span class="right tag outline">Latest</span></div>${ui.resultCard({ details: true })}</section>
  <aside class="panel moi stack scroll-y" style="gap:.625rem"><div class="hstack"><span class="title">${ui.esc(d.character.name)}</span><span class="right muted">${d.character.klass} · ${d.character.level}</span></div>${ui.abilities()}${ui.hpChange()}${ui.chargeCard()}<div><div class="label" style="margin-bottom:.25rem">Saving throws</div>${ui.saves()}</div></aside>
</div>
<section class="panel history stack" style="gap:.5rem">${ui.journalHead()}<div class="scroll-y grow">${ui.journalTable()}</div><div class="hair"></div>${ui.diceLine()}</section>`,
  );

const lookup = () =>
  shell(
    "spells",
    `<section class="panel lookup-table stack"><div class="panel-head"><span class="title">Spell lookup</span></div>${ui.lookupFilter()}<div class="scroll-y grow" style="margin-top:.5rem">${ui.lookupTable()}</div></section>
<section class="panel detail stack" style="gap:.75rem"><div>${ui.lookupHead()}${ui.lookupStats()}</div>${ui.castBox()}<div class="scroll-y grow">${ui.lookupProse()}</div></section>`,
  );

const xp = () =>
  shell(
    "xp",
    `<section class="panel xp-ledger stack"><div class="panel-head"><span class="title">Ledger</span><span class="right muted">4 entries</span></div><div class="scroll-y grow">${ui.xpLedger()}</div></section>
<div class="stack xp-side">
  <section class="panel"><div class="panel-head"><span class="title">Experience</span><span class="muted">${d.character.klass} · Level ${d.character.level}</span></div><div class="hstack" style="align-items:baseline;gap:.5rem"><span class="hero">${d.xp.total}</span><span class="muted">XP</span></div><div class="xpbar" style="margin-top:.5rem"><i class="fill" style="width:${d.xp.pct}%"></i></div><div class="hstack between muted label" style="text-transform:none;letter-spacing:0;margin-top:.25rem"><span>${d.xp.floor}</span><span>${d.xp.pct}%</span><span>${d.xp.ceiling}</span></div><div class="highlight" style="margin-top:.5rem">Before level ${d.xp.nextLevel} · ${d.xp.toGo} XP</div></section>
  <section class="panel"><div class="panel-head"><span class="title">Add XP</span></div>${ui.xpForm()}<p class="muted" style="margin-top:.75rem">+10 %: the Bard's prime-requisite bonus (DEX 17 and CHA 16, both 16 or more).</p></section>
  <section class="panel grow scroll-y"><div class="panel-head"><span class="title">Thresholds</span></div>${ui.xpLevelsList()}</section>
</div>`,
  );

export const pages = {
  "act-weapons": () => act("weapons"),
  "act-spells": () => act("spells"),
  lookup,
  xp,
};

export const stylesheet = `/* Direction A3: one strip on top with the activities as tabs; a "now" band and the journal as history. */
.app { height: 100vh; display: flex; flex-direction: column; }
.strip { height: 2.75rem; display: flex; align-items: center; gap: 1rem; padding: 0 .75rem; border-bottom: 1px solid var(--border); background: var(--surface); white-space: nowrap; }
.strip > * { min-width: 0; flex-shrink: 0; }
.strip .label { margin-right: -.25rem; }
.act-tabs { display: flex; gap: .125rem; align-self: stretch; }
.act-tabs .tab { padding: 0 .625rem; border-bottom-width: 2px; }
.act-tabs .tab.on { background: var(--amb-900); }
.v-effects .hstack { flex-wrap: nowrap; }
.mini-hp { display: inline-block; width: 7rem; }
.mini-hp .hpbar { height: .5rem; }
.mini-hp .hpscale { display: none; }
@media (max-width: 1500px) { .strip { gap: .625rem; } .v-brand .wordmark { display: none; } .act-tabs .tab { font-size: .75rem; padding: 0 .5rem; } .v-result .muted, .v-effects .label, .v-hp .label, .v-turn .label { display: none; } .mini-hp { width: 5rem; } }
.stage { flex: 1; min-height: 0; display: grid; gap: .75rem; padding: .75rem; }
.stage-act { grid-template-rows: minmax(0, 46fr) minmax(0, 54fr); }
@media (max-height: 1000px) { .stage-act { grid-template-rows: minmax(0, 54fr) minmax(0, 46fr); } }
.now { display: grid; grid-template-columns: minmax(28rem, 1.2fr) minmax(22rem, 1fr) 21rem; gap: .75rem; min-height: 0; }
.panel.act { display: flex; flex-direction: column; }
.result-panel { border-color: var(--amb-700); }
.weapon-card { margin-top: .75rem; border: 1px solid var(--amb-700); border-radius: .625rem; padding: 1rem; background: linear-gradient(135deg, color-mix(in oklab, var(--tint) 10%, transparent), transparent 60%); }
.tiles { display: grid; grid-template-columns: repeat(4, 1fr); gap: .5rem; margin-top: .75rem; }
.tile { display: flex; flex-direction: column; gap: .125rem; padding: .5rem .625rem; border-radius: .375rem; background: var(--surface); border: 1px solid var(--border); }
.tile.on { border-color: var(--tint); }
.tile.dim { opacity: .55; }
.two-col { display: grid; grid-template-columns: 1fr 1fr; column-gap: 1.5rem; }
.saves .row28 { background: none; border-top: 1px solid var(--border); border-radius: 0; }
.jt { display: grid; }
.jt-row { display: grid; grid-template-columns: 9rem 3rem 1.125rem 1fr 4rem; align-items: center; gap: .75rem; min-height: 1.75rem; padding: .125rem .5rem; border-left: 2px solid transparent; }
.jt-row.first { border-top: 1px solid var(--border); }
.jt-row.first:first-child { border-top: 0; }
.jt-row.current { background: var(--amb-900); border-left-color: var(--tint); }
.jt-row .ico { color: var(--ink-muted); }
.jt-row .time { font-size: .6875rem; text-align: right; }
.jt-row .val { font-weight: 600; text-align: right; }
.jt-row .reroll { opacity: 0; margin-left: .5rem; color: var(--tint); font-size: .6875rem; display: inline-flex; align-items: center; gap: .25rem; vertical-align: middle; }
.jt-row:hover .reroll, .jt-row.show-hover .reroll { opacity: 1; }
.jt-row.show-hover { background: var(--neu-900); }
.stage-spells { grid-template-columns: 1fr minmax(24rem, 30rem); }
.lk-head, .lk-row { display: grid; grid-template-columns: 2rem minmax(14rem, 1.4fr) 7rem 7rem minmax(8rem, 1fr) 5rem 4rem; gap: .75rem; align-items: center; padding: .25rem .5rem; }
.lk-head { border-bottom: 1px solid var(--border); }
.lk-row { min-height: 2.25rem; border-radius: .375rem; }
.lk-row + .lk-row { border-top: 1px solid var(--border); border-radius: 0; }
.lk-row.sel { background: var(--amb-900); box-shadow: inset 2px 0 0 var(--tint); }
.detail .prose { max-width: 72ch; }
.stage-xp { grid-template-columns: 1fr 24rem; }
.levels-list .row28.now { background: var(--amb-900); box-shadow: inset 2px 0 0 var(--tint); }
.list-row.fresh { background: color-mix(in oklab, var(--tint) 8%, transparent); }
`;
