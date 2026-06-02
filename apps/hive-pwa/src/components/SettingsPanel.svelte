<script lang="ts">
  export let endpoint = ''
  export let deviceID = ''
  export let token = ''
  export let remoteBusy = false
  export let onEndpointInput: (event: Event) => void
  export let onDeviceIDInput: (event: Event) => void
  export let onTokenInput: (event: Event) => void
  export let onValidateToken: () => Promise<void>
  export let onCheckSyncState: () => Promise<void>
  export let onManualSync: () => Promise<void>
  export let onLogout: () => Promise<void>
</script>

<section class="card">
  <label>
    Endpoint
    <input
      type="url"
      value={endpoint}
      on:input={onEndpointInput}
      placeholder="https://hive-sync-api.example.run.app"
    />
  </label>

  <label>
    Device ID
    <input
      type="text"
      value={deviceID}
      on:input={onDeviceIDInput}
      placeholder="personal"
    />
  </label>

  <label>
    Bearer Token
    <input
      type="password"
      value={token}
      on:input={onTokenInput}
      placeholder="token value only"
    />
  </label>

  <p class="hint">
    Settings, local notes, local todos, queue state, and last sync cursor are stored in browser
    IndexedDB on this device.
  </p>

  <div class="actions">
    <button type="button" on:click={onValidateToken} disabled={remoteBusy}>Validate Token</button>
    <button type="button" on:click={onCheckSyncState} disabled={remoteBusy}>Check Sync State</button>
    <button type="button" on:click={onManualSync} disabled={remoteBusy}>Manual Sync</button>
    <button type="button" class="danger" on:click={onLogout} disabled={remoteBusy}>Forget Token</button>
  </div>
</section>
