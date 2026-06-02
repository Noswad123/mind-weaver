<script lang="ts">
  import HiveMindMark from '../HiveMindMark.svelte'

  export let tabsOpen = true
  export let onToggleTabs: () => void
</script>

<header class="app-header">
  <button type="button" class="title-toggle" on:click={onToggleTabs} aria-expanded={tabsOpen}>
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
