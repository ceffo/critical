// The Critical! mark options (ADR 0020 step 5 names the current one). Every mark is drawn in the
// role colours (`--tint` for the primary, `--highlight` for the "!" / the lit face) with static
// Arcane dark-mode fallbacks, so the same SVG works inline on a themed page and as a standalone
// file at favicon size.
const T = "var(--tint, #a99bf5)";
const H = "var(--highlight, #f0c437)";
const ON = "var(--on-accent, #1a1630)";

export const logos = {
  current: {
    name: "Current (reference)",
    concept: "A filled d20 face in the primary with the \"!\" in the highlight; ADR 0020 step 5.",
    body: `<polygon points="16,1.5 28.6,8.75 28.6,23.25 16,30.5 3.4,23.25 3.4,8.75" fill="${T}"/>
<polygon points="16,7 23.8,20.5 8.2,20.5" fill="none" stroke="${ON}" stroke-opacity=".6" stroke-width="1.6" stroke-linejoin="round"/>
<rect x="14.3" y="9.5" width="3.4" height="9" rx="1.7" fill="${H}"/>
<circle cx="16" cy="22.6" r="2" fill="${H}"/>`,
  },
  facet: {
    name: "Facet",
    concept:
      "A line-drawn icosahedron (the d20 seen from a vertex) in the primary; its front face lights up in gold: the crit face.",
    body: `<polygon points="16,2 28,9 28,23 16,30 4,23 4,9" fill="none" stroke="${T}" stroke-width="2" stroke-linejoin="round"/>
<polygon points="16,8.5 23,20.5 9,20.5" fill="${H}" stroke="${H}" stroke-width="1" stroke-linejoin="round"/>
<path d="M16 8.5L4 9M16 8.5L28 9M23 20.5L28 23M23 20.5L16 30M9 20.5L4 23M9 20.5L16 30" fill="none" stroke="${T}" stroke-width="1.5" stroke-linecap="round"/>`,
  },
  blade: {
    name: "Blade",
    concept:
      "The \"!\" itself is the mark: a sword point-down in the primary, its dot the gold spark where the blade lands.",
    body: `<rect x="14.4" y="1.5" width="3.2" height="4" rx="1" fill="${T}"/>
<rect x="8.5" y="5" width="15" height="2.8" rx="1.4" fill="${T}"/>
<path d="M12.6 8.2h6.8L17 21.5h-2z" fill="${T}"/>
<path d="M16 23.2l2.3 1.7-.9 2.7h-2.8l-.9-2.7z" fill="${H}"/>`,
  },
  impact: {
    name: "Impact",
    concept:
      "An eight-point burst in the primary (the hit landing) carrying a bold gold \"!\": loud, unmistakable at 16 px.",
    body: `<path d="M16 1.5l3 6.1 6.1-3-3 6.1 6.1 3-6.1 3 3 6.1-6.1-3-3 6.1-3-6.1-6.1 3 3-6.1-6.1-3 6.1-3-3-6.1 6.1 3z" fill="${T}" stroke="${T}" stroke-width="1" stroke-linejoin="round"/>
<rect x="14.4" y="8" width="3.2" height="10.5" rx="1.6" fill="${H}"/>
<circle cx="16" cy="22.4" r="2.1" fill="${H}"/>`,
  },
  monogram: {
    name: "Monogram",
    concept:
      "A heavy \"C\" in the primary opening onto a gold \"!\": the name reduced to C!, calm and typographic; the wordmark can drop its mark.",
    body: `<path d="M23.5 7.5A11 11 0 1 0 23.5 24.5" fill="none" stroke="${T}" stroke-width="5.5" stroke-linecap="round"/>
<rect x="21.8" y="8.5" width="3.6" height="11" rx="1.8" fill="${H}"/>
<circle cx="23.6" cy="23.6" r="2.2" fill="${H}"/>`,
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
