HiveMind Cyberpunk Industrial UI Style Brief

Goal

Create a polished futuristic UI for the HiveMind Svelte app that blends:

* Cold worn steel: dark industrial panels, scratched edges, subtle bevels, and chassis-like framing.
* Radiating ember orange: active states should feel warm, powered, and alive, like orange light glowing from behind metal.
* Deep-freeze blue contrast: inactive or secondary elements should feel cold, dormant, and slightly frozen.
* Holographic markdown preview: the rendered note area should feel like a sci-fi holodisplay with faint grids, scanlines, coordinates, and subtle depth.

The existing app structure can stay mostly intact. The coding agent should primarily replace/extend CSS and add a few small SVG/icon components.

⸻

Visual Language

Core mood

Think “industrial cyberdeck meets Blade Runner holo terminal.” The app should not look like generic neon glassmorphism. It should look like a physical device UI: dark steel panels, glowing seams, hard angular corners, subtle grime, and small HUD details.

Color palette

Use CSS custom properties so the theme is easy to tune.

:root {
  --bg-void: #050812;
  --bg-panel: #0b111b;
  --bg-panel-2: #101925;
  --steel: #1b2633;
  --steel-light: #34465b;
  --steel-dark: #060a10;
  --ember: #ff8a3d;
  --ember-hot: #ffb067;
  --ember-deep: #c94d1c;
  --ember-glow: rgba(255, 122, 45, 0.52);
  --frost: #27a8ff;
  --frost-soft: #6fc7ff;
  --frost-deep: #0a3e67;
  --frost-glow: rgba(39, 168, 255, 0.36);
  --pink-signal: #ff5dbb;
  --green-sync: #38e6a4;
  --danger: #ff4d5e;
  --text: #f4f7fb;
  --text-muted: #9aa9ba;
  --text-dim: #617287;
  --border-cold: rgba(116, 174, 220, 0.28);
  --border-hot: rgba(255, 138, 61, 0.72);
  --shadow-black: rgba(0, 0, 0, 0.65);
}

Use orange/peach for active selection, primary actions, headings, and the currently selected note. Use blue for inactive nav items, dormant notes, secondary buttons, and cold metadata. Use pink only as a subtle accent on the logo/title gradient, not as the dominant theme.

⸻

Typography

Prefer a squared futuristic font for labels and buttons, paired with a readable body font.

Recommended stack:

font-family: Inter, ui-sans-serif, system-ui, sans-serif;

For nav buttons, metadata, badges, and HUD labels:

font-family: "Rajdhani", "Orbitron", Inter, sans-serif;
letter-spacing: 0.08em;
text-transform: uppercase;

If adding external fonts is acceptable, use:

<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&family=Rajdhani:wght@500;600;700&display=swap" rel="stylesheet">

⸻

Layout Changes

App shell

The whole app should sit in a large armored container. Add an outer frame effect to .app:

* Very dark background.
* Subtle diagonal/linear texture.
* 1px cold border.
* Inner orange glow near active areas.
* Clip-path or pseudo-elements for angled panel corners.

Suggested CSS:

html,
body {
  margin: 0;
  min-height: 100%;
  background:
    radial-gradient(circle at 18% 0%, rgba(255, 93, 187, 0.11), transparent 28%),
    radial-gradient(circle at 88% 12%, rgba(255, 138, 61, 0.14), transparent 30%),
    radial-gradient(circle at 70% 90%, rgba(39, 168, 255, 0.12), transparent 32%),
    var(--bg-void);
  color: var(--text);
}
.app {
  min-height: 100vh;
  padding: 1rem;
  background:
    linear-gradient(135deg, rgba(255,255,255,0.035), transparent 20%),
    repeating-linear-gradient(90deg, rgba(255,255,255,0.018) 0 1px, transparent 1px 80px),
    var(--bg-void);
}

⸻

Header

Current markup

<header class="app-header">
  <button type="button" class="title-toggle" on:click={() => (tabsOpen = !tabsOpen)} aria-expanded={tabsOpen}>
    <span class="brain-icon" aria-hidden="true">{tabsOpen ? '🧠' : '🫥'}</span>
    <span>HiveMind</span>
  </button>
</header>

Recommended markup update

Replace emoji with an SVG logo component or inline SVG. Keep the same click behavior.

<header class="app-header">
  <button
    type="button"
    class="title-toggle"
    on:click={() => (tabsOpen = !tabsOpen)}
    aria-expanded={tabsOpen}
  >
    <span class="hive-logo" aria-hidden="true">
      <HiveMindMark collapsed={!tabsOpen} />
    </span>
    <span class="title-stack">
      <span class="app-title">HiveMind</span>
      <span class="app-subtitle">Sync trust established</span>
    </span>
    <span class="collapse-indicator" aria-hidden="true"></span>
  </button>
</header>

Logo direction

The logo should be a geometric neural hive mark:

* Hexagon outer frame.
* Inner neural nodes/lines.
* Orange/pink gradient stroke when open.
* Frost-blue stroke when collapsed.
* Slight glow filter.

Example Svelte component:

<!-- HiveMindMark.svelte -->
<script lang="ts">
  export let collapsed = false
</script>
<svg class:collapsed viewBox="0 0 64 64" role="img" aria-label="HiveMind mark">
  <defs>
    <linearGradient id="hiveMarkGradient" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0%" stop-color="#ff5dbb" />
      <stop offset="55%" stop-color="#ff8a3d" />
      <stop offset="100%" stop-color="#27a8ff" />
    </linearGradient>
    <filter id="hiveGlow" x="-50%" y="-50%" width="200%" height="200%">
      <feGaussianBlur stdDeviation="2.4" result="blur" />
      <feMerge>
        <feMergeNode in="blur" />
        <feMergeNode in="SourceGraphic" />
      </feMerge>
    </filter>
  </defs>
  <path
    class="outer"
    d="M32 4 56 18v28L32 60 8 46V18L32 4Z"
    fill="none"
    stroke="url(#hiveMarkGradient)"
    stroke-width="2.5"
    filter="url(#hiveGlow)"
  />
  <path class="inner" d="M20 22h24M20 42h24M20 22l12 20 12-20M20 42l12-20 12 20" fill="none" />
  <circle class="node" cx="20" cy="22" r="2.4" />
  <circle class="node" cx="44" cy="22" r="2.4" />
  <circle class="node" cx="32" cy="32" r="2.8" />
  <circle class="node" cx="20" cy="42" r="2.4" />
  <circle class="node" cx="44" cy="42" r="2.4" />
</svg>
<style>
  svg {
    width: 2.8rem;
    height: 2.8rem;
  }
  .inner {
    stroke: rgba(255, 176, 103, 0.9);
    stroke-width: 1.5;
    filter: drop-shadow(0 0 7px rgba(255, 138, 61, 0.45));
  }
  .node {
    fill: #ffb067;
    filter: drop-shadow(0 0 7px rgba(255, 138, 61, 0.7));
  }
  svg.collapsed .outer,
  svg.collapsed .inner {
    stroke: #27a8ff;
  }
  svg.collapsed .node {
    fill: #6fc7ff;
  }
</style>

Header CSS:

.app-header {
  margin-bottom: 0.75rem;
  border: 1px solid var(--border-cold);
  background:
    linear-gradient(180deg, rgba(20, 30, 44, 0.94), rgba(5, 8, 18, 0.96)),
    repeating-linear-gradient(135deg, rgba(255,255,255,0.035) 0 1px, transparent 1px 6px);
  box-shadow:
    inset 0 0 0 1px rgba(255,255,255,0.04),
    0 18px 40px var(--shadow-black);
  clip-path: polygon(10px 0, calc(100% - 10px) 0, 100% 10px, 100% calc(100% - 10px), calc(100% - 10px) 100%, 10px 100%, 0 calc(100% - 10px), 0 10px);
}
.title-toggle {
  width: 100%;
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: 1rem;
  align-items: center;
  border: 0;
  padding: 1rem 1.25rem;
  color: var(--text);
  background: transparent;
  cursor: pointer;
}
.app-title {
  display: block;
  font-size: clamp(2rem, 4vw, 3.2rem);
  line-height: 0.9;
  font-weight: 800;
  letter-spacing: -0.04em;
  background: linear-gradient(90deg, var(--pink-signal), var(--ember-hot) 54%, var(--frost-soft));
  -webkit-background-clip: text;
  color: transparent;
  text-shadow: 0 0 22px rgba(255, 138, 61, 0.25);
}
.app-subtitle {
  display: block;
  margin-top: 0.3rem;
  font-family: "Rajdhani", Inter, sans-serif;
  color: var(--frost-soft);
  letter-spacing: 0.18em;
  text-transform: uppercase;
  font-size: 0.78rem;
}
.collapse-indicator {
  width: 1rem;
  height: 1rem;
  border-right: 2px solid var(--frost-soft);
  border-bottom: 2px solid var(--frost-soft);
  transform: rotate(45deg);
  filter: drop-shadow(0 0 8px var(--frost-glow));
}

⸻

Navigation Tabs

The nav should look like heavy metal plates. Active tab should glow orange from behind. Inactive tabs should look cold/frozen blue.

.tab-nav {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 0.45rem;
  margin-bottom: 0.75rem;
  padding: 0.5rem;
  border: 1px solid var(--border-cold);
  background: linear-gradient(180deg, rgba(12, 18, 29, 0.95), rgba(5, 8, 18, 0.95));
  clip-path: polygon(8px 0, calc(100% - 8px) 0, 100% 8px, 100% calc(100% - 8px), calc(100% - 8px) 100%, 8px 100%, 0 calc(100% - 8px), 0 8px);
}
.tab-nav button,
.note-mode-toggle button,
button {
  font-family: "Rajdhani", Inter, sans-serif;
  font-weight: 700;
  letter-spacing: 0.08em;
}
.tab-nav button {
  position: relative;
  isolation: isolate;
  min-height: 3.4rem;
  border: 1px solid rgba(111, 199, 255, 0.22);
  color: var(--frost-soft);
  background:
    linear-gradient(180deg, rgba(20, 32, 46, 0.95), rgba(7, 12, 20, 0.96));
  text-transform: uppercase;
  clip-path: polygon(8px 0, 100% 0, 100% calc(100% - 8px), calc(100% - 8px) 100%, 0 100%, 0 8px);
  box-shadow:
    inset 0 0 16px rgba(39, 168, 255, 0.06),
    inset 0 -1px 0 rgba(255,255,255,0.05);
}
.tab-nav button::before {
  content: "";
  position: absolute;
  inset: auto 12% -1px 12%;
  height: 2px;
  background: var(--frost);
  opacity: 0.35;
  filter: blur(0.5px) drop-shadow(0 0 8px var(--frost-glow));
}
.tab-nav button.active-tab {
  color: var(--ember-hot);
  border-color: var(--border-hot);
  background:
    radial-gradient(circle at 50% 100%, rgba(255, 138, 61, 0.42), transparent 50%),
    linear-gradient(180deg, rgba(56, 31, 18, 0.96), rgba(14, 12, 11, 0.98));
  box-shadow:
    inset 0 0 18px rgba(255, 138, 61, 0.22),
    0 0 28px rgba(255, 111, 36, 0.16);
}
.tab-nav button.active-tab::before {
  background: var(--ember-hot);
  opacity: 1;
  filter: blur(0.3px) drop-shadow(0 0 12px var(--ember-glow));
}

⸻

Cards and Panels

All .card, .split-card, .list-panel, and .editor-panel should share an armored panel style.

.card,
.list-panel,
.editor-panel,
.stat,
.todo-item,
.list-item {
  border: 1px solid rgba(116, 174, 220, 0.2);
  background:
    linear-gradient(180deg, rgba(13, 20, 31, 0.94), rgba(5, 9, 16, 0.96)),
    repeating-linear-gradient(135deg, rgba(255,255,255,0.025) 0 1px, transparent 1px 8px);
  box-shadow:
    inset 0 0 0 1px rgba(255,255,255,0.035),
    0 18px 45px rgba(0,0,0,0.45);
}
.card {
  padding: 1.2rem;
  clip-path: polygon(14px 0, calc(100% - 14px) 0, 100% 14px, 100% calc(100% - 14px), calc(100% - 14px) 100%, 14px 100%, 0 calc(100% - 14px), 0 14px);
}
.section-header h2,
.card h2 {
  margin: 0;
  color: var(--ember-hot);
  font-size: 1.65rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  text-shadow: 0 0 16px rgba(255, 138, 61, 0.28);
}
.subtext,
.muted,
.hint {
  color: var(--text-muted);
}

⸻

Notes List Items

Selected note: orange ember glow. Unselected notes: cold frozen blue.

.list-item,
.todo-item {
  width: 100%;
  display: grid;
  gap: 0.35rem;
  padding: 1rem 1.1rem;
  text-align: left;
  color: var(--text);
  background:
    radial-gradient(circle at 0% 50%, rgba(39, 168, 255, 0.1), transparent 38%),
    linear-gradient(180deg, rgba(9, 18, 28, 0.95), rgba(5, 10, 18, 0.96));
  border: 1px solid rgba(111, 199, 255, 0.22);
  clip-path: polygon(10px 0, 100% 0, 100% calc(100% - 10px), calc(100% - 10px) 100%, 0 100%, 0 10px);
  box-shadow: inset 0 0 20px rgba(39, 168, 255, 0.05);
}
.list-item strong,
.todo-item strong {
  color: var(--frost-soft);
  font-size: 1.05rem;
}
.list-item span,
.todo-item span {
  color: var(--text-muted);
}
.list-item:hover,
.todo-item:hover {
  border-color: rgba(111, 199, 255, 0.55);
  box-shadow:
    inset 0 0 24px rgba(39, 168, 255, 0.12),
    0 0 18px rgba(39, 168, 255, 0.08);
}
.list-item.selected-item,
.todo-item.selected-item,
.list-item:first-of-type {
  border-color: var(--border-hot);
  background:
    radial-gradient(circle at 0% 50%, rgba(255, 138, 61, 0.28), transparent 42%),
    linear-gradient(180deg, rgba(28, 18, 13, 0.96), rgba(7, 8, 10, 0.98));
  box-shadow:
    inset 4px 0 0 rgba(255, 176, 103, 0.85),
    inset 0 0 24px rgba(255, 138, 61, 0.12),
    0 0 24px rgba(255, 102, 36, 0.12);
}
.list-item.selected-item strong,
.list-item:first-of-type strong {
  color: var(--ember-hot);
}

Note: the first-of-type selector is optional if the app does not have a selected item while browsing. Ideally, apply selected-item to the active/previewed item instead of relying on first item.

⸻

Pills and Status Badges

Make state labels feel like small illuminated capsules.

.badge-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  align-items: center;
}
.pill {
  display: inline-flex;
  align-items: center;
  min-height: 1.35rem;
  padding: 0.15rem 0.55rem;
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 700;
  line-height: 1;
  background: rgba(255,255,255,0.055);
}
.source-pill.remote,
.source-pill.local {
  color: var(--frost-soft);
  background: rgba(39, 168, 255, 0.12);
  border-color: rgba(39, 168, 255, 0.28);
}
.state-pill.synced {
  color: #b7ffe3;
  background: rgba(56, 230, 164, 0.13);
  border-color: rgba(56, 230, 164, 0.35);
  box-shadow: 0 0 10px rgba(56, 230, 164, 0.1);
}
.state-pill.queued,
.state-pill.local-only {
  color: var(--ember-hot);
  background: rgba(255, 138, 61, 0.13);
  border-color: rgba(255, 138, 61, 0.35);
}
.state-pill.conflict,
.status.error {
  color: #ffd1d6;
  background: rgba(255, 77, 94, 0.13);
  border-color: rgba(255, 77, 94, 0.35);
}

⸻

Inputs, Selects, Textareas

Inputs should feel embedded into the terminal chassis.

input,
select,
textarea {
  width: 100%;
  box-sizing: border-box;
  border: 1px solid rgba(111, 199, 255, 0.24);
  background:
    linear-gradient(180deg, rgba(4, 8, 15, 0.96), rgba(8, 15, 24, 0.96));
  color: var(--text);
  border-radius: 0.35rem;
  padding: 0.85rem 0.95rem;
  outline: none;
  box-shadow:
    inset 0 0 18px rgba(0,0,0,0.38),
    inset 0 0 0 1px rgba(255,255,255,0.02);
}
input:focus,
select:focus,
textarea:focus {
  border-color: var(--ember-hot);
  box-shadow:
    inset 0 0 18px rgba(0,0,0,0.45),
    0 0 0 3px rgba(255, 138, 61, 0.15),
    0 0 24px rgba(255, 138, 61, 0.1);
}
label {
  display: grid;
  gap: 0.4rem;
  color: var(--frost-soft);
  font-family: "Rajdhani", Inter, sans-serif;
  font-weight: 700;
  letter-spacing: 0.07em;
  text-transform: uppercase;
}

⸻

Buttons

Primary buttons should be orange. Secondary should be frost blue. Danger remains red but subdued.

button {
  border: 1px solid var(--border-hot);
  color: var(--ember-hot);
  background:
    radial-gradient(circle at 50% 100%, rgba(255, 138, 61, 0.32), transparent 55%),
    linear-gradient(180deg, rgba(30, 18, 12, 0.96), rgba(8, 9, 12, 0.98));
  padding: 0.75rem 1rem;
  border-radius: 0.35rem;
  cursor: pointer;
  text-transform: uppercase;
  box-shadow:
    inset 0 0 14px rgba(255, 138, 61, 0.1),
    0 0 16px rgba(255, 138, 61, 0.08);
}
button.secondary,
.note-mode-toggle button:not(.active-tab) {
  border-color: rgba(111, 199, 255, 0.3);
  color: var(--frost-soft);
  background:
    radial-gradient(circle at 50% 100%, rgba(39, 168, 255, 0.18), transparent 58%),
    linear-gradient(180deg, rgba(13, 24, 36, 0.96), rgba(6, 10, 17, 0.98));
}
button.danger {
  border-color: rgba(255, 77, 94, 0.45);
  color: #ff9aa4;
  background: linear-gradient(180deg, rgba(45, 12, 18, 0.96), rgba(10, 7, 10, 0.98));
}
button:disabled {
  opacity: 0.48;
  cursor: not-allowed;
  filter: grayscale(0.45);
}

⸻

Holographic Markdown Preview

The preview pane is the biggest opportunity. Make .note-preview.rendered-markdown look like a holographic display.

Effects:

* Orange frame with angular clipped corners.
* Blue grid receding into the pane.
* Subtle scanlines.
* Faint central neural/hive watermark.
* Text is readable, but code blocks/tables/headings get HUD styling.

.note-preview {
  position: relative;
  isolation: isolate;
  min-height: 28rem;
  overflow: hidden;
  padding: 2rem;
  border: 1px solid var(--border-hot);
  color: #fbe7d3;
  background:
    radial-gradient(circle at 50% 45%, rgba(39, 168, 255, 0.16), transparent 26%),
    radial-gradient(circle at 50% 100%, rgba(255, 138, 61, 0.18), transparent 45%),
    linear-gradient(180deg, rgba(4, 10, 17, 0.9), rgba(5, 8, 12, 0.96));
  clip-path: polygon(24px 0, calc(100% - 24px) 0, 100% 24px, 100% calc(100% - 24px), calc(100% - 24px) 100%, 24px 100%, 0 calc(100% - 24px), 0 24px);
  box-shadow:
    inset 0 0 28px rgba(255, 138, 61, 0.1),
    inset 0 0 55px rgba(39, 168, 255, 0.08),
    0 0 38px rgba(255, 102, 36, 0.1);
}
.note-preview::before {
  content: "";
  position: absolute;
  inset: 0;
  z-index: -2;
  background:
    linear-gradient(rgba(39, 168, 255, 0.11) 1px, transparent 1px),
    linear-gradient(90deg, rgba(39, 168, 255, 0.09) 1px, transparent 1px),
    repeating-linear-gradient(0deg, rgba(255,255,255,0.035) 0 1px, transparent 1px 5px);
  background-size: 44px 44px, 44px 44px, 100% 5px;
  transform: perspective(520px) rotateX(58deg) translateY(14rem) scale(1.5);
  transform-origin: center bottom;
  opacity: 0.65;
}
.note-preview::after {
  content: "";
  position: absolute;
  inset: 12%;
  z-index: -1;
  opacity: 0.23;
  background:
    radial-gradient(circle, rgba(111, 199, 255, 0.35), transparent 58%),
    url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 160 100'%3E%3Cg fill='none' stroke='%2327a8ff' stroke-width='1'%3E%3Cpath d='M25 70 Q80 15 135 70'/%3E%3Cpath d='M40 70 Q80 32 120 70'/%3E%3Cpath d='M55 70 Q80 48 105 70'/%3E%3Cpath d='M25 70 H135'/%3E%3C/g%3E%3C/svg%3E") center / contain no-repeat;
  filter: drop-shadow(0 0 20px rgba(39, 168, 255, 0.25));
}
.rendered-markdown h1,
.rendered-markdown h2,
.rendered-markdown h3 {
  color: var(--ember-hot);
  text-shadow: 0 0 14px rgba(255, 138, 61, 0.25);
  border-bottom: 1px solid rgba(255, 138, 61, 0.22);
  padding-bottom: 0.35rem;
}
.rendered-markdown a {
  color: var(--frost-soft);
  text-shadow: 0 0 10px rgba(39, 168, 255, 0.2);
}
.rendered-markdown code {
  color: var(--frost-soft);
  background: rgba(39, 168, 255, 0.1);
  border: 1px solid rgba(39, 168, 255, 0.2);
  padding: 0.1rem 0.3rem;
  border-radius: 0.25rem;
}
.rendered-markdown pre {
  border: 1px solid rgba(39, 168, 255, 0.25);
  background: rgba(3, 8, 14, 0.78);
  padding: 1rem;
  overflow: auto;
  box-shadow: inset 0 0 24px rgba(39, 168, 255, 0.06);
}

Optional: add tiny HUD coordinates in the preview with markup:

<article class="note-preview rendered-markdown" aria-label="Rendered note">
  <span class="holo-corner top-left">MD::RENDER</span>
  <span class="holo-corner bottom-right">X: 982 · Y: 17 · Z: 4</span>
  {@html renderedNoteContent}
</article>
.holo-corner {
  position: absolute;
  z-index: 2;
  font-family: "Rajdhani", Inter, sans-serif;
  color: rgba(255, 176, 103, 0.65);
  letter-spacing: 0.12em;
  font-size: 0.72rem;
  pointer-events: none;
}
.holo-corner.top-left {
  top: 0.85rem;
  left: 1rem;
}
.holo-corner.bottom-right {
  right: 1rem;
  bottom: 0.85rem;
}

⸻

Footer Status Rail

Add an optional footer rail under the main workspace for sync status, last sync, and encryption/device trust. This reinforces the sci-fi terminal feel.

Suggested Svelte placement near the bottom of <main> inside the authenticated branch:

<footer class="status-rail" aria-label="Sync status summary">
  <div>
    <span>Sync status</span>
    <strong>{queuedCount > 0 ? `${queuedCount} queued` : 'All systems nominal'}</strong>
  </div>
  <div>
    <span>Last sync</span>
    <strong>{lastSuccessfulSyncAt}</strong>
  </div>
  <div>
    <span>Device</span>
    <strong>{deviceID}</strong>
  </div>
</footer>
.status-rail {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 1rem;
  margin-top: 0.75rem;
  padding: 0.8rem 1rem;
  border: 1px solid rgba(111, 199, 255, 0.22);
  background:
    linear-gradient(180deg, rgba(12, 18, 26, 0.95), rgba(5, 8, 14, 0.96));
  clip-path: polygon(10px 0, calc(100% - 10px) 0, 100% 10px, 100% 100%, 0 100%, 0 10px);
}
.status-rail span {
  display: block;
  color: var(--text-dim);
  font-family: "Rajdhani", Inter, sans-serif;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  font-size: 0.75rem;
}
.status-rail strong {
  color: var(--frost-soft);
  font-weight: 700;
}

⸻

Responsive Behavior

Keep the current mobile-friendly structure, but ensure the sci-fi treatments do not make mobile cramped.

.note-mobile-layout,
.todo-mobile-layout {
  display: grid;
  grid-template-columns: minmax(18rem, 0.9fr) minmax(24rem, 1.3fr);
  gap: 1rem;
}
.list-toolbar,
.todo-compose,
.actions,
.reader-title-row,
.reader-meta-row,
.section-header {
  display: flex;
  gap: 0.75rem;
  align-items: center;
  justify-content: space-between;
}
.list-toolbar input,
.todo-compose input {
  flex: 1;
}
@media (max-width: 860px) {
  .app {
    padding: 0.6rem;
  }
  .tab-nav {
    grid-template-columns: 1fr;
  }
  .note-mobile-layout,
  .todo-mobile-layout,
  .status-rail {
    grid-template-columns: 1fr;
  }
  .list-toolbar,
  .todo-compose,
  .actions,
  .reader-title-row,
  .reader-meta-row,
  .section-header {
    align-items: stretch;
    flex-direction: column;
  }
  .note-preview {
    min-height: 18rem;
    padding: 1.25rem;
  }
}

⸻

Motion and Interaction

Use subtle transitions only. Avoid excessive animations.

* {
  transition:
    border-color 160ms ease,
    box-shadow 160ms ease,
    background-color 160ms ease,
    color 160ms ease,
    transform 160ms ease;
}
button:hover:not(:disabled),
.list-item:hover,
.todo-item:hover {
  transform: translateY(-1px);
}
@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    transition: none !important;
    animation: none !important;
  }
}

Optional scan animation for the preview:

.note-preview .scanline {
  position: absolute;
  inset-inline: 0;
  top: -20%;
  height: 18%;
  background: linear-gradient(180deg, transparent, rgba(39, 168, 255, 0.12), transparent);
  animation: holo-scan 7s linear infinite;
  pointer-events: none;
}
@keyframes holo-scan {
  to {
    transform: translateY(700%);
  }
}

Markup:

<article class="note-preview rendered-markdown" aria-label="Rendered note">
  <span class="scanline" aria-hidden="true"></span>
  {@html renderedNoteContent}
</article>

⸻

Implementation Plan for Coding Agent

1. Add HiveMindMark.svelte in the same component folder or in src/lib/components.
2. Update the header markup to use the SVG component instead of emoji.
3. Add optional HUD labels and scanline spans inside the note preview article.
4. Add optional status-rail footer beneath authenticated content.
5. Replace or extend the existing CSS with the custom properties and component styles above.
6. Tune spacing after the CSS is applied. The most important areas to check are:
    * Header height.
    * Tab nav button height.
    * Notes list item density.
    * Markdown preview readability.
    * Mobile layout below 860px.
7. Confirm accessibility:
    * Keep visible focus states.
    * Maintain aria-expanded on the header toggle.
    * Do not rely on color alone for conflict/sync state; keep text labels.
    * Respect prefers-reduced-motion.

⸻

Acceptance Criteria

The finished UI should:

* Feel like a dark industrial sci-fi console, not a generic dark Bootstrap app.
* Use orange/peach as the active powered state.
* Use blue as the cold dormant/inactive state.
* Make the preview area feel like a holographic markdown renderer.
* Preserve all current app functionality.
* Stay readable and usable on mobile.
* Keep the visual system centralized in CSS custom properties so colors can be tuned quickly.
