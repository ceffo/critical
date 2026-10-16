// The Critical! mark options: the current mark (ADR 0020 step 5) for reference and four
// developments of "Facet" (round 1), where the die and the "!" are one mark. Every variant shares
// one geometry: a hexagon of radius 13.5 (the d20 seen from a vertex) and its front face, the
// triangle on the alternate vertices with the apex down. The "!" sits on the die's vertical axis,
// inside the face, and its dot lands on the apex vertex, so the die's edges, the stem and the dot
// line up. The die is `--tint`, the "!" `--highlight`, with static Arcane dark-mode fallbacks, so
// the same SVG works inline on a themed page and as a standalone favicon.
const T = "var(--tint, #a99bf5)";
const H = "var(--highlight, #f0c437)";
const ON = "var(--on-accent, #1a1630)";
const OH = "var(--on-highlight, #1a1630)";

// Hexagon of radius 13.5 centred on (16,16); the front face joins the two upper-side vertices
// and the bottom one; the two remaining faces are the sides.
const HEX = "16,2.5 27.7,9.25 27.7,22.75 16,29.5 4.3,22.75 4.3,9.25";
const FACE = "4.3,9.25 27.7,9.25 16,29.5";
const SIDES = "M4.3 9.25L16 2.5L27.7 9.25";

// The "!" as a stem (a rounded bar) and a dot on the apex side; `w` is the stem width.
const bang = (fill, w = 3) =>
  `<rect x="${(16 - w / 2).toFixed(2)}" y="11.5" width="${w}" height="9" rx="${(w / 2).toFixed(2)}" fill="${fill}"/>
<circle cx="16" cy="24.3" r="${(w * 0.58).toFixed(2)}" fill="${fill}"/>`;

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
    name: "Facet · Spine",
    concept:
      "Outline d20: the hexagon and its front face at one stroke weight; the gold \"!\" runs down the face's axis and its dot sits on the apex. Two shapes and a mark, nothing to decipher at 16 px.",
    body: `<polygon points="${HEX}" fill="none" stroke="${T}" stroke-width="2" stroke-linejoin="round"/>
<polygon points="${FACE}" fill="none" stroke="${T}" stroke-width="2" stroke-linejoin="round"/>
${bang(H, 3)}`,
  },
  "facet-v": {
    name: "Facet · V",
    concept:
      "The front face is open at the top: only its two slanted edges are drawn, a V that frames the \"!\". The stem is a tapered wedge, parallel to the V, and the dot is a small facet: the \"!\" is cut from the die's own geometry.",
    body: `<polygon points="${HEX}" fill="none" stroke="${T}" stroke-width="2" stroke-linejoin="round"/>
<path d="M4.3 9.25L16 29.5L27.7 9.25" fill="none" stroke="${T}" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/>
<path d="M13.4 9.5h5.2L16.9 20.2h-1.8z" fill="${H}" stroke="${H}" stroke-width="1" stroke-linejoin="round"/>
<path d="M14.1 22.6h3.8L16 25.9z" fill="${H}" stroke="${H}" stroke-width="1" stroke-linejoin="round"/>`,
  },
  "facet-solid": {
    name: "Facet · Solid",
    concept:
      "The die filled in the primary, its front face cut out as two dark edges, and the gold \"!\" inside that face, stem on the axis, dot on the apex. The solid shape carries the mark at 16 px; the face and the \"!\" share one centre line.",
    body: `<polygon points="${HEX}" fill="${T}"/>
<path d="M4.3 9.25L16 29.5L27.7 9.25M4.3 9.25h23.4" fill="none" stroke="${ON}" stroke-opacity=".55" stroke-width="1.5" stroke-linejoin="round" stroke-linecap="round"/>
${bang(H, 3)}`,
  },
  "facet-crit": {
    name: "Facet · Crit",
    concept:
      "The front face lit in gold (the crit face, from round 1) with the \"!\" cut out of it in the dark: the mark is the lit face itself, the die outline around it. One gold triangle with a notch reads from across the table.",
    body: `<polygon points="${HEX}" fill="none" stroke="${T}" stroke-width="2" stroke-linejoin="round"/>
<polygon points="${FACE}" fill="${H}" stroke="${H}" stroke-width="1.5" stroke-linejoin="round"/>
${bang(OH, 3)}`,
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
