<script lang="ts">
  import type { StatusMessage } from '../appTypes'

  export let endpoint = ''
  export let deviceID = ''
  export let token = ''
  export let healthMessage: StatusMessage
  export let remoteBusy = false
  export let onEndpointInput: (event: Event) => void
  export let onDeviceIDInput: (event: Event) => void
  export let onTokenInput: (event: Event) => void
  export let onValidateToken: () => Promise<void>
</script>

<section class="card login-card">
  <h2>Connect this device</h2>
  <p class="subtext">
    Enter the device ID and bearer token from your password manager. They are cached in this
    browser's IndexedDB after validation, so you should not need to enter them every visit.
  </p>

  <label>
    API Endpoint
    <input type="url" value={endpoint} on:input={onEndpointInput} placeholder="https://hive-sync-api.example.run.app" />
  </label>

  <label>
    Device ID
    <input type="text" value={deviceID} on:input={onDeviceIDInput} placeholder="phone" autocomplete="username" />
  </label>

  <label>
    Bearer Token
    <input type="password" value={token} on:input={onTokenInput} placeholder="token value only" autocomplete="current-password" />
  </label>

  <div class="actions">
    <button type="button" on:click={onValidateToken} disabled={remoteBusy}>Save and Validate</button>
  </div>
  <p class={`status ${healthMessage.kind}`}>{healthMessage.text}</p>
</section>
