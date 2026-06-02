<script lang="ts">
  export let queuedCount = 0
  export let lastSuccessfulSyncAt = '—'
  export let deviceID = ''
</script>

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
