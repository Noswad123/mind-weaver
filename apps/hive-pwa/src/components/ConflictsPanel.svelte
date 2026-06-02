<script lang="ts">
  import type { LocalSyncConflict } from '../hiveStorage'
  import {
    describeConflictReason,
    formatConflictTarget,
    formatTimestamp,
  } from '../appUtils'

  export let recentConflicts: LocalSyncConflict[] = []
  export let openConflictReviewCount = 0
  export let conflictBusy = false
  export let reviewConflict: (conflictID: string) => Promise<void>
</script>

<section class="card">
  <div class="section-header">
    <div>
      <h2>Conflict visibility</h2>
      <p class="subtext">Minimal mobile conflict banner/log baseline for Phase 3B.</p>
    </div>
    <span class={`network-pill ${openConflictReviewCount > 0 ? 'offline' : 'online'}`}>
      {openConflictReviewCount > 0 ? `${openConflictReviewCount} open` : 'No open conflicts'}
    </span>
  </div>

  {#if openConflictReviewCount > 0}
    <p class="status error">
      Some mobile changes hit sync conflicts. Review the recent list below, then use desktop tooling
      like <code>mw sync conflicts review --export-dir ~/.local/share/mw/conflicts/reviews</code>
      for deeper triage/export workflows.
    </p>
  {:else}
    <p class="status ok">No open mobile conflict records right now.</p>
  {/if}

  {#if recentConflicts.length === 0}
    <p class="empty-state">No local conflict records yet.</p>
  {:else}
    <div class="todo-list">
      {#each recentConflicts as conflict}
        <div class="todo-item">
          <div class="badge-row">
            <span class="pill state-pill conflict">{conflict.status === 'open' ? 'Open' : 'Reviewed'}</span>
            <span class="pill source-pill {conflict.entityType === 'note' ? 'local' : 'remote'}">{conflict.entityType}</span>
          </div>
          <strong>{formatConflictTarget(conflict)}</strong>
          <span>{describeConflictReason(conflict.reason)}</span>
          <span class="muted">Created {formatTimestamp(conflict.createdAt)}</span>
          {#if conflict.winnerDeviceID || conflict.loserDeviceID}
            <span class="muted">
              winner {conflict.winnerDeviceID || 'unknown'} · loser {conflict.loserDeviceID || 'unknown'}
            </span>
          {/if}
          {#if conflict.reviewedAt}
            <span class="muted">Reviewed {formatTimestamp(conflict.reviewedAt)}</span>
          {/if}
          {#if conflict.status === 'open'}
            <div class="actions">
              <button type="button" class="secondary" on:click={() => reviewConflict(conflict.id)} disabled={conflictBusy}>
                Mark Reviewed
              </button>
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</section>
