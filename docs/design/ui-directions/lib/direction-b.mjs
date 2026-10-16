// Direction B, "Character left, history below": the round-3 combination of the user's picks,
// reworked in round 4 with the user. The header is one band: the brand (which is the menu), the
// turn, initiative + Next turn, then the effects timeline beside them and the last result docked
// at the right. Hit points live in the character column, between the abilities and HP change.
// Below the header the character is a column on the left screen edge, the activity rail right of
// it, then the stage. In Act the stage is A3's two bands: "now" (the act panel and the result side by side) and
// the journal as a full-width table, with a drag handle between them that resizes or collapses the
// journal; the dice composer is a footer across the whole width. The Weapons tab is rebuilt: the
// weapon in hand is one row whose Attack button carries the weight. The lookup and XP stages are
// A2's.
import * as d from "./data.mjs";
import * as ui from "./ui.mjs";

export const id = "b-character-left";
export const name = "B · Character left, history below";
export const logo = "facet-bare";
export const css = "direction.css";

const RAIL = [
  ["act", "Act", "fist"],
  ["spells", "Spells", "book"],
  ["xp", "Experience", "trend"],
  ["inventory", "Inventory", "bag"],
];

// The brand is the menu: mark + wordmark with a caret, hover/focus ring, no cog anywhere.
const brandMenu = () =>
  `<span class="brand-menu" role="button" tabindex="0" title="Menu">${ui.brand(logo)}${ui.ph("CaretDown", "ico caret")}</span>`;

const header = () => `<header class="vitals">
  <div class="v-brand">${brandMenu()}</div>
  <div class="v-turn"><span class="label">Turn</span><span class="hero">${d.turn.number}</span></div>
  <div class="v-init"><span class="hstack"><span class="muted">Initiative</span><span class="tag">${d.turn.initiative}</span><span class="btn ghost xs icon">${ui.icon("reroll")}</span></span><span class="hstack"><span class="btn primary">Next turn ${ui.icon("arrow")}</span><span class="btn icon">${ui.icon("dots")}</span></span></div>
  <div class="v-timeline">${ui.effectsTrack(12)}</div>
  <div class="v-result"><span class="label">Last result · ${d.lastResult.when}</span>${ui.resultLine()}</div>
</header>`;

const character = () =>
  `<aside class="moi stack"><div><div class="title">${ui.esc(d.character.name)}</div><div class="muted">${d.character.klass} · Level ${d.character.level}</div></div>${ui.abilities()}${ui.hpBlock("headline")}<div><div class="label" style="margin-bottom:.375rem">HP change</div>${ui.hpChange()}</div>${ui.chargeCard()}<div><div class="label" style="margin-bottom:.375rem">Saving throws · click to roll</div>${ui.savesButtons()}</div></aside>`;

const shell = (active, stage, cls = "") => `<div class="app">
${header()}
<div class="below">
  ${character()}
  ${ui.rail(active, RAIL, { gear: false })}
  <main class="stage stage-${active} ${cls}">${stage}</main>
</div>
<footer class="dice-foot">${ui.diceLine()}</footer>
</div>`;

const journalHead = () =>
  `<div class="hstack"><span class="title">Journal</span><span class="seg" style="height:1.5rem"><span class="on">All</span><span>Rolls</span></span><span class="muted">${d.journal.length} entries</span><span class="right btn xs">Undo</span><span class="btn xs ghost">Redo</span></div>`;

const act = (tab, { collapsed = false } = {}) =>
  shell(
    "act",
    `<div class="now-band">
  <section class="panel act"><div class="panel-head">${ui.actTabs(tab)}</div><div class="scroll-y grow">${ui.actBodyB(tab, { columns: 2 })}</div></section>
  <section class="panel result-panel scroll-y"><div class="panel-head"><span class="label">Result</span><span class="right tag outline">Latest</span></div>${ui.resultCard({ details: true })}</section>
</div>
${ui.splitHandle({ collapsed })}
${collapsed ? ui.journalBar() : `<section class="panel history stack" style="gap:.5rem">${journalHead()}<div class="scroll-y grow">${ui.journalTable()}</div></section>`}`,
    collapsed ? "collapsed" : "",
  );

const lookup = () =>
  shell(
    "spells",
    `<section class="panel lookup-side stack"><div class="panel-head"><span class="title">Spell lookup</span></div>${ui.lookupFilter()}<div class="hair"></div><div class="scroll-y grow">${ui.lookupList()}</div></section>
<section class="panel lookup-detail stack" style="gap:.75rem"><div>${ui.lookupHead()}${ui.lookupTiles()}</div>${ui.castBox()}<div class="scroll-y grow prose-cols">${ui.lookupProse()}</div></section>`,
  );

const xp = () =>
  shell(
    "xp",
    `<section class="panel xp-top"><div class="xp-grid"><div><div class="panel-head"><span class="title">Experience</span><span class="muted">${d.character.klass} · Level ${d.character.level}</span></div>${ui.xpProgress()}</div><div class="xp-form"><div class="panel-head"><span class="title">Add XP</span></div>${ui.xpForm()}<p class="muted" style="margin-top:.75rem">+10 %: the Bard's prime-requisite bonus (DEX 17 and CHA 16, both 16 or more).</p></div></div></section>
<section class="panel xp-ledger stack"><div class="panel-head"><span class="title">Ledger</span><span class="right muted">4 entries</span></div><div class="scroll-y grow">${ui.xpLedger()}</div></section>`,
  );

export const pages = {
  "act-weapons": () => act("weapons"),
  "act-spells": () => act("spells"),
  "act-spells-collapsed": () => act("spells", { collapsed: true }),
  lookup,
  xp,
};

export const stylesheet = `/* Direction B: one header band (brand = menu, turn, initiative, the effects timeline, the last result docked right), the character on the left edge with HP, the rail beside it, A3's journal below with a handle, the dice as a footer. */
.app { height: 100vh; display: flex; flex-direction: column; }
.vitals { display: grid; grid-template-columns: auto auto auto minmax(0, 1fr) auto; align-items: stretch; border-bottom: 1px solid var(--border); background: var(--surface); }
.vitals > div { padding: .5rem 1rem; border-right: 1px solid var(--border); min-width: 0; display: flex; align-items: center; gap: .75rem; }
.vitals > div:last-child { border-right: 0; }
@media (max-width: 1500px) { .vitals > div { padding: .5rem .75rem; } .v-brand .wordmark { display: none; } }
.brand-menu { display: inline-flex; align-items: center; gap: .375rem; padding: .25rem .5rem .25rem .375rem; border: 1px solid var(--border); border-radius: .5rem; cursor: pointer; }
.brand-menu:hover, .brand-menu:focus-visible { border-color: var(--tint); background: var(--amb-900); outline: none; }
.brand-menu .caret { width: .75rem; height: .75rem; color: var(--ink-muted); }
.v-turn { gap: .5rem !important; }
.v-turn .hero { font-size: 2rem; line-height: 2.25rem; }
.v-init { flex-direction: column; align-items: flex-start !important; justify-content: center; gap: .25rem !important; }
.v-timeline { padding: .375rem 1rem .5rem !important; }
.v-result { flex-direction: column; align-items: flex-start !important; justify-content: center; gap: .25rem !important; }
.effects-track { width: 100%; display: grid; row-gap: .125rem; }
.moi .hp-block .headline { font-size: 1.5rem; line-height: 1.75rem; }
.moi .hp-change .btn { padding: 0 .5rem; }
.trow { display: grid; grid-template-columns: 11rem 1fr; align-items: center; min-height: 1.5rem; }
.ttrack { display: grid; align-items: center; min-height: 1.5rem; }
.tcol { text-align: center; font-size: .6875rem; color: var(--ink-muted); }
.tcol.now { color: var(--tint); font-weight: 600; background: var(--amb-900); border-radius: .25rem; }
.tcol.past { opacity: .4; }
.tline { height: 2px; background: var(--neu-700); align-self: center; }
.pill.fade { mask-image: linear-gradient(90deg, #000 70%, transparent); }
.below { flex: 1; display: flex; min-height: 0; }
.moi { width: 17.5rem; flex-shrink: 0; padding: .875rem 1rem; border-right: 1px solid var(--border); background: var(--surface); overflow-y: auto; }
.rail { width: 4.25rem; flex-shrink: 0; display: flex; flex-direction: column; align-items: stretch; padding: .5rem 0; border-right: 1px solid var(--border); background: var(--surface); }
.rail-item { display: flex; flex-direction: column; align-items: center; gap: .25rem; padding: .625rem 0; font-size: .6875rem; color: var(--ink-muted); border-left: 2px solid transparent; }
.rail-item.on { color: var(--tint); border-left-color: var(--tint); background: var(--amb-900); }
.saves-btns { display: grid; gap: .25rem; }
.save-btn { display: flex; align-items: center; gap: .5rem; height: 2rem; padding: 0 .625rem; border: 1px solid var(--border); border-radius: .375rem; }
.stage { flex: 1; min-width: 0; min-height: 0; display: grid; gap: .75rem; padding: .75rem; }
.dice-foot { border-top: 1px solid var(--border); background: var(--surface); padding: .5rem .75rem; }
/* Act: a column; the "now" band takes its content's height up to 60 % of the stage, the journal at least 14rem and the rest. */
.stage-act { display: flex; flex-direction: column; gap: 0; }
.now-band { flex: 0 1 auto; max-height: 60%; display: grid; grid-template-columns: minmax(28rem, 1.35fr) minmax(24rem, 1fr); gap: .75rem; min-height: 0; }
.history { flex: 1 0 14rem; min-height: 0; }
.stage-act.collapsed .now-band { flex: 1 1 auto; max-height: none; }
.stage-act.collapsed .history { flex: 0 0 auto; }
.panel.act { display: flex; flex-direction: column; }
.result-panel { border-color: var(--amb-700); }
.split-handle { position: relative; display: flex; align-items: center; justify-content: center; height: 1.5rem; margin: .125rem 0; cursor: row-resize; }
.split-handle .grip { width: 4rem; height: .25rem; border-radius: 999px; background: var(--neu-600); }
.split-handle .collapse-btn { position: absolute; right: .25rem; top: 0; height: 1.5rem; color: var(--ink-muted); }
.history.collapsed { padding: .5rem 1rem; }
.weapon-row { margin-top: .75rem; display: flex; align-items: center; gap: 1rem; padding: .75rem 1rem; border: 1px solid var(--amb-700); border-radius: .625rem; background: linear-gradient(135deg, color-mix(in oklab, var(--tint) 10%, transparent), transparent 60%); }
.weapon-row .w-main { flex: 1; min-width: 0; }
.weapon-row .w-stats { margin-top: .25rem; }
.weapon-row .w-stats b { color: var(--ink); font-weight: 600; }
.weapon-row .w-stats .dim { opacity: .6; }
.btn.primary.attack { height: 2.75rem; padding: 0 1.25rem 0 0; gap: 0; border-radius: .5rem; font-size: .9375rem; }
.btn.primary.attack .attack-ico { width: 2.75rem; height: 2.75rem; display: grid; place-items: center; }
.btn.primary.attack .ico { width: 1.25rem; height: 1.25rem; }
.btn.primary.attack > span { display: flex; flex-direction: column; align-items: flex-start; line-height: 1rem; }
.btn.primary.attack .attack-verb { font-weight: 600; }
.btn.primary.attack .attack-needs { font-size: .6875rem; font-weight: 500; opacity: .8; }
.two-col { display: grid; grid-template-columns: 1fr 1fr; column-gap: 1.5rem; }
.jt { display: grid; }
.jt-row { display: grid; grid-template-columns: 9rem 3rem 1.125rem 1fr 4rem; align-items: center; gap: .75rem; min-height: 1.75rem; padding: .125rem .5rem; border-left: 2px solid transparent; }
.jt-row.first { border-top: 1px solid var(--border); }
.jt-row.first:first-child { border-top: 0; }
.jt-row.current { background: var(--amb-900); border-left-color: var(--tint); }
.jt-row .ico { color: var(--ink-muted); }
.jt-row .time { font-size: .6875rem; text-align: right; }
.jt-row.first .val { font-weight: 600; }
.jt-row .val { font-weight: 600; text-align: right; }
.jt-row .reroll { opacity: 0; margin-left: .5rem; color: var(--tint); font-size: .6875rem; display: inline-flex; align-items: center; gap: .25rem; vertical-align: middle; }
.jt-row:hover .reroll, .jt-row.show-hover .reroll { opacity: 1; }
.jt-row.show-hover { background: var(--neu-900); }
.history.collapsed .val { font-weight: 600; }
/* Lookup and XP: A2's stages. */
.stage-spells { grid-template-columns: minmax(18rem, 22rem) 1fr; }
.tiles { display: grid; gap: .5rem; margin-top: .75rem; }
.tile { display: flex; flex-direction: column; gap: .125rem; padding: .5rem .625rem; border-radius: .375rem; background: var(--surface); border: 1px solid var(--border); }
.stat-tiles { grid-template-columns: repeat(5, minmax(0, 1fr)); }
.prose-cols .prose { columns: 2; column-gap: 2rem; max-width: 140ch; }
.prose-cols .prose p { break-inside: avoid; }
.row28.sel { background: var(--amb-900); box-shadow: inset 2px 0 0 var(--tint); }
.stage-xp { grid-template-rows: auto minmax(0, 1fr); }
.xp-grid { display: grid; grid-template-columns: minmax(0, 1fr) 24rem; gap: 2rem; }
.xp-grid > div { min-width: 0; }
.list-row.fresh { background: color-mix(in oklab, var(--tint) 8%, transparent); }
`;
