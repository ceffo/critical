// The Critical! mark options: the current mark (ADR 0020 step 5) for reference, Facet · Spine
// (round 2) and the user's pick from round 2, "the first one but drop the triangle": Spine without
// its front face, in two takes. Every mark shares one geometry: a hexagon of radius 13.5 (the d20
// seen from a vertex) with the "!" on the die's vertical axis. The die is `--tint`, the "!"
// `--highlight`, with static Arcane dark-mode fallbacks, so the same SVG works inline on a themed
// page and as a standalone favicon.
const T = "var(--tint, #a99bf5)";
const H = "var(--highlight, #f0c437)";
const ON = "var(--on-accent, #1a1630)";

// Hexagon of radius 13.5 centred on (16,16); the front face (Spine only) joins the two upper-side
// vertices and the bottom one.
const HEX = "16,2.5 27.7,9.25 27.7,22.75 16,29.5 4.3,22.75 4.3,9.25";
const FACE = "4.3,9.25 27.7,9.25 16,29.5";

// The "!" as a stem (a rounded bar) from `y` and a dot under it; `w` is the stem width.
const bang = (fill, { w = 3, y = 11.5, h = 9, dot = 24.3 } = {}) =>
  `<rect x="${(16 - w / 2).toFixed(2)}" y="${y}" width="${w}" height="${h}" rx="${(w / 2).toFixed(2)}" fill="${fill}"/>
<circle cx="16" cy="${dot}" r="${(w * 0.6).toFixed(2)}" fill="${fill}"/>`;

export const logos = {
  current: {
    name: "Current (reference)",
    concept: "A filled d20 face in the primary with the \"!\" in the highlight; ADR 0020 step 5.",
    body: `<polygon points="16,1.5 28.6,8.75 28.6,23.25 16,30.5 3.4,23.25 3.4,8.75" fill="${T}"/>
<polygon points="16,7 23.8,20.5 8.2,20.5" fill="none" stroke="${ON}" stroke-opacity=".6" stroke-width="1.6" stroke-linejoin="round"/>
<rect x="14.3" y="9.5" width="3.4" height="9" rx="1.7" fill="${H}"/>
<circle cx="16" cy="22.6" r="2" fill="${H}"/>`,
  },
  "facet-spine": {
    name: "Facet · Spine (round 2)",
    concept:
      "Outline d20: the hexagon and its front face at one stroke weight; the gold \"!\" runs down the face's axis and its dot sits on the apex. The user's pick, minus the triangle.",
    body: `<polygon points="${HEX}" fill="none" stroke="${T}" stroke-width="2" stroke-linejoin="round"/>
<polygon points="${FACE}" fill="none" stroke="${T}" stroke-width="2" stroke-linejoin="round"/>
${bang(H)}`,
  },
  "facet-bare": {
    name: "Facet · Bare",
    concept:
      "Spine without the front face: the hexagon outline alone, the gold \"!\" centred on its axis, the same stroke and stem weights as Spine. Two shapes, one axis.",
    body: `<polygon points="${HEX}" fill="none" stroke="${T}" stroke-width="2" stroke-linejoin="round"/>
${bang(H, { w: 3, y: 8.5, h: 9.5, dot: 21.9 })}`,
  },
  "facet-bare-heavy": {
    name: "Facet · Bare, heavy",
    concept:
      "The same mark with a heavier edge (2.6) and a wider stem (3.8), tuned for the favicon: at 16 px the hexagon stays a closed shape and the \"!\" keeps its dot.",
    body: `<polygon points="${HEX}" fill="none" stroke="${T}" stroke-width="2.6" stroke-linejoin="round"/>
${bang(H, { w: 3.8, y: 8, h: 10, dot: 22.3 })}`,
  },
};

export const logoSvg = (id, attrs = "") =>
  `<svg viewBox="0 0 32 32" aria-hidden="true" ${attrs}>${logos[id].body}</svg>`;

/** The standalone file: the same drawing with an xmlns and a comment. */
export const logoFile = (id) =>
  `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="32" height="32">
  <!-- Critical! mark option "${logos[id].name}": ${logos[id].concept} Colours: --tint / --highlight with Arcane dark fallbacks. -->
${logos[id].body}
</svg>
`;
