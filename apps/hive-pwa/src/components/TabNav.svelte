<script lang="ts">
  import type { AppTab } from '../appTypes'

  export let activeTab: AppTab
  export let onSelectTab: (tab: AppTab) => void
</script>

<nav class="tab-nav" aria-label="Primary sections">
  <button type="button" class:active-tab={activeTab === 'notes'} on:click={() => onSelectTab('notes')}>
    Notes
  </button>
  <button type="button" class:active-tab={activeTab === 'todos'} on:click={() => onSelectTab('todos')}>
    Todos
  </button>
  <button type="button" class:active-tab={activeTab === 'sync'} on:click={() => onSelectTab('sync')}>
    Sync
  </button>
  <button type="button" class:active-tab={activeTab === 'settings'} on:click={() => onSelectTab('settings')}>
    Settings
  </button>
</nav>
