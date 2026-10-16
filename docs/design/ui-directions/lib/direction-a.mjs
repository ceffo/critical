// Direction A, "Vitals bar": everything that must always be visible is one dense bar across the
// top (turn + initiative + Next turn, the effects timeline, HP with its bar, the last result);
// below it a rail switches explicitly between three stages that each own the whole width.
import * as d from "./data.mjs";
import * as ui from "./ui.mjs";

export const id = "a-vitals-bar";
export const name = "A · Vitals bar + stages";
export const logo = "facet-crit";
// The fourth mark option is shown on this direction's lookup and XP pages.
const altLogo = "facet-crit";
export const css = "direction.css";

const RAIL = [
  ["act", "Act", "fist"],
  ["spells", "Spells", "book"],
  ["xp", "Experience", "trend"],
  ["inventory", "Inventory", "bag"],
];

const vitals = (mark) => `<header class="vitals">
  <div class="v-brand">${ui.brand(mark)}</div>
  <div class="v-turn">${ui.turnBlock("hero")}</div>
  <div class="v-effects">${ui.effectsTrack(8)}</div>
  <div class="v-hp">${ui.hpBlock("headline")}</div>
  <div class="v-result"><div class="hstack"><span class="label">Last result</span><span class="muted label" style="text-transform:none;letter-spacing:0">${d.lastResult.when}</span></div>${ui.resultLine()}<div class="muted body" style="margin-top:.25rem">${ui.esc(d.lastResult.subject)} vs AC ${d.lastResult.targetAc} · ${d.lastResult.damageDie} → ${d.lastResult.damageRoll} + ${d.lastResult.damageBonus}</div></div>
</header>`;

const shell = (active, stage, mark = logo) => `<div class="app">
${vitals(mark)}
<div class="below">
  ${ui.rail(active, RAIL)}
  <main class="stage stage-${active}">${stage}</main>
</div>
</div>`;

const act = (tab) =>
  shell(
    "act",
    `<section class="panel act"><div class="panel-head"><span class="title">Act</span>${ui.actTabs(tab)}</div><div class="scroll-y grow">${ui.actBody(tab, { columns: 2 })}</div></section>
<section class="panel fil stack"><div class="panel-head"><span class="label">Result</span><span class="right tag outline">Latest</span></div>${ui.resultCard()}<div class="hair" style="margin:.25rem 0"></div>${ui.journalHead()}<div class="scroll-y grow">${ui.journalList()}</div><div class="hair"></div>${ui.diceComposer()}</section>
<aside class="panel moi stack"><div><div class="title">${ui.esc(d.character.name)}</div><div class="muted">${d.character.klass} · Level ${d.character.level}</div></div>${ui.abilities()}<div><div class="label" style="margin-bottom:.375rem">HP change</div>${ui.hpChange()}</div>${ui.chargeCard()}<div><div class="label" style="margin-bottom:.375rem">Saving throws</div>${ui.saves()}</div></aside>`,
  );

const lookup = () =>
  shell(
    "spells",
    `<section class="panel lookup-side stack"><div class="panel-head"><span class="title">Spell lookup</span></div>${ui.lookupFilter()}<div class="hair"></div><div class="scroll-y grow">${ui.lookupList()}</div></section>
<section class="panel lookup-detail">${ui.lookupDetail({ split: true })}</section>`,
    altLogo,
  );

const xp = () =>
  shell(
    "xp",
    `<section class="panel xp-progress"><div class="panel-head"><span class="title">Experience</span><span class="muted">${d.character.klass} · Level ${d.character.level}</span></div>${ui.xpProgress()}</section>
<section class="panel xp-ledger stack"><div class="panel-head"><span class="title">Ledger</span><span class="right muted">4 entries</span></div><div class="scroll-y grow">${ui.xpLedger()}</div></section>
<section class="panel xp-add"><div class="panel-head"><span class="title">Add XP</span></div>${ui.xpForm()}<p class="muted" style="margin-top:.75rem">+10 %: the Bard's prime-requisite bonus (DEX 17 and CHA 16, both 16 or more).</p></section>`,
    altLogo,
  );

export const pages = {
  "act-weapons": () => act("weapons"),
  "act-spells": () => act("spells"),
  lookup,
  xp,
};

export const stylesheet = `/* Direction A: a vitals bar on top, a rail, and one stage per activity. */
.app { height: 100vh; display: flex; flex-direction: column; }
.vitals {
  display: grid;
  grid-template-columns: auto auto minmax(20rem, 1fr) 15rem 17rem;
  align-items: stretch;
  gap: 0;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
}
.vitals > div { padding: .625rem 1rem; border-right: 1px solid var(--border); min-width: 0; }
.vitals > div:last-child { border-right: 0; }
.v-brand { display: flex; align-items: center; }
@media (max-width: 1500px) { .vitals > div { padding: .625rem .75rem; } .v-brand .wordmark { display: none; } }
.v-turn { display: flex; align-items: center; }
.turn-block { display: grid; grid-template-columns: auto auto; column-gap: .75rem; row-gap: .25rem; align-items: center; }
.turn-num { display: flex; flex-direction: column; grid-row: 1 / 3; }
.turn-num .label { margin-bottom: -.25rem; }
.v-effects { display: flex; align-items: center; }
.effects-track { width: 100%; display: grid; row-gap: .125rem; }
.trow { display: grid; grid-template-columns: 9rem 1fr; align-items: center; min-height: 1.5rem; }
.ttrack { display: grid; align-items: center; min-height: 1.5rem; }
.tcol { text-align: center; font-size: .6875rem; color: var(--ink-muted); }
.tcol.now { color: var(--tint); font-weight: 600; background: var(--amb-900); border-radius: .25rem; }
.tcol.past { opacity: .4; }
.tline { height: 2px; background: var(--neu-700); align-self: center; }
.pill.fade { mask-image: linear-gradient(90deg, #000 70%, transparent); }
.v-hp { display: flex; flex-direction: column; justify-content: center; }
.v-result { display: flex; flex-direction: column; justify-content: center; }
.below { flex: 1; display: flex; min-height: 0; }
.rail { width: 4.25rem; display: flex; flex-direction: column; align-items: stretch; padding: .5rem 0; border-right: 1px solid var(--border); background: var(--surface); }
.rail-item { display: flex; flex-direction: column; align-items: center; gap: .25rem; padding: .625rem 0; font-size: .6875rem; color: var(--ink-muted); border-left: 2px solid transparent; }
.rail-item.on { color: var(--tint); border-left-color: var(--tint); background: var(--amb-900); }
.stage { flex: 1; min-width: 0; min-height: 0; display: grid; grid-auto-rows: minmax(0, 1fr); gap: .75rem; padding: .75rem; }
.stage-act { grid-template-columns: minmax(28rem, 1.1fr) minmax(24rem, 1fr) 20rem; }
.panel.act { display: flex; flex-direction: column; }
.weapon-card { margin-top: .75rem; border: 1px solid var(--amb-700); border-radius: .625rem; padding: 1rem; background: linear-gradient(135deg, color-mix(in oklab, var(--tint) 10%, transparent), transparent 60%); }
.tiles { display: grid; grid-template-columns: repeat(4, 1fr); gap: .5rem; margin-top: .75rem; }
.tile { display: flex; flex-direction: column; gap: .125rem; padding: .5rem .625rem; border-radius: .375rem; background: var(--surface); border: 1px solid var(--border); }
.tile.on { border-color: var(--tint); }
.tile.dim { opacity: .55; }
.two-col { display: grid; grid-template-columns: 1fr 1fr; column-gap: 1.5rem; }
.saves .row28 { background: none; border-top: 1px solid var(--border); border-radius: 0; }
.stage-spells { grid-template-columns: minmax(24rem, 28rem) 1fr; }
.lookup-side { display: flex; flex-direction: column; }
.lookup-detail { display: flex; flex-direction: column; min-height: 0; }
.detail-split { display: grid; grid-template-columns: 22rem 1fr; gap: 1.5rem; min-height: 0; flex: 1; }
.detail-split .prose { max-width: 70ch; }
.row28.sel { background: var(--amb-900); box-shadow: inset 2px 0 0 var(--tint); }
.stage-xp { grid-template-columns: 1fr 24rem; grid-template-rows: auto 1fr; }
.xp-progress { grid-column: 1 / 3; }
.list-row.fresh { background: color-mix(in oklab, var(--tint) 8%, transparent); }
`;
