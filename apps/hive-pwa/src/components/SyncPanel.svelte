<script lang="ts">
  import type { StatusMessage } from '../appTypes'

  export let isOnline = true
  export let healthMessage: StatusMessage
  export let remoteStateMessage: StatusMessage
  export let storageMessage: StatusMessage
  export let workspaceMessage: StatusMessage
  export let remoteBusy = false
  export let notesCount = 0
  export let todosCount = 0
  export let pendingDraftCount = 0
  export let localOnlyCount = 0
  export let queuedCount = 0
  export let syncedCount = 0
  export let conflictCount = 0
  export let localCursor = '—'
  export let remoteCursor = '—'
  export let lastRemoteCheckAt = '—'
  export let lastObservedServerTime = '—'
  export let lastSuccessfulSyncAt = '—'
  export let onManualSync: () => Promise<void>
  export let onCheckSyncState: () => Promise<void>
</script>

<section class="card">
  <div class="section-header">
    <div>
      <h2>Sync health</h2>
      <p class="subtext">Connectivity, remote sync-state, local cursor, and queued work.</p>
    </div>
    <span class={`network-pill ${isOnline ? 'online' : 'offline'}`}>
      {isOnline ? 'Online' : 'Offline'}
    </span>
  </div>

  <p class={`status ${healthMessage.kind}`}>{healthMessage.text}</p>
  <p class={`status ${remoteStateMessage.kind}`}>{remoteStateMessage.text}</p>
  <p class={`status ${storageMessage.kind}`}>{storageMessage.text}</p>
  <p class={`status ${workspaceMessage.kind}`}>{workspaceMessage.text}</p>

  <div class="actions">
    <button type="button" on:click={onManualSync} disabled={remoteBusy}>Sync now</button>
    <button type="button" class="secondary" on:click={onCheckSyncState} disabled={remoteBusy}>Check state</button>
  </div>

  <div class="stats-grid">
    <div class="stat">
      <span class="stat-label">Connection</span>
      <strong>{isOnline ? 'Online' : 'Offline'}</strong>
    </div>
    <div class="stat">
      <span class="stat-label">Local notes</span>
      <strong>{notesCount}</strong>
    </div>
    <div class="stat">
      <span class="stat-label">Local todos</span>
      <strong>{todosCount}</strong>
    </div>
    <div class="stat">
      <span class="stat-label">Queued sync ops</span>
      <strong>{pendingDraftCount}</strong>
    </div>
    <div class="stat">
      <span class="stat-label">Local only</span>
      <strong>{localOnlyCount}</strong>
    </div>
    <div class="stat">
      <span class="stat-label">Queued items</span>
      <strong>{queuedCount}</strong>
    </div>
    <div class="stat">
      <span class="stat-label">Synced items</span>
      <strong>{syncedCount}</strong>
    </div>
    <div class="stat">
      <span class="stat-label">Conflicts</span>
      <strong>{conflictCount}</strong>
    </div>
    <div class="stat">
      <span class="stat-label">Local cursor</span>
      <strong>{localCursor}</strong>
    </div>
    <div class="stat">
      <span class="stat-label">Remote cursor</span>
      <strong>{remoteCursor}</strong>
    </div>
    <div class="stat">
      <span class="stat-label">Last remote check</span>
      <strong>{lastRemoteCheckAt}</strong>
    </div>
    <div class="stat">
      <span class="stat-label">Server time</span>
      <strong>{lastObservedServerTime}</strong>
    </div>
    <div class="stat">
      <span class="stat-label">Last successful sync</span>
      <strong>{lastSuccessfulSyncAt}</strong>
    </div>
  </div>
</section>
