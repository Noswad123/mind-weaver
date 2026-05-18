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
