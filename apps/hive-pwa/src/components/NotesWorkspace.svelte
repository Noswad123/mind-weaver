<script lang="ts">
  import type { NoteMode } from '../appTypes'
  import type { LocalNote } from '../hiveStorage'
  import { describeSyncState, formatNoteStats, formatTimestamp } from '../appUtils'

  export let editingNotePath: string | null = null
  export let noteBusy = false
  export let sortedNotes: LocalNote[] = []
  export let notes: LocalNote[] = []
  export let selectedNote: LocalNote | null = null
  export let noteDraftActive = false
  export let noteSearchQuery = ''
  export let noteSortMode = 'updated'
  export let noteMode: NoteMode = 'read'
  export let notePath = ''
  export let noteTitle = ''
  export let noteContent = ''
  export let renderedNoteContent = ''
  export let resetNoteEditor: () => void
  export let editNote: (note: LocalNote) => void
  export let saveNote: () => Promise<void>
  export let removeCurrentNote: () => Promise<void>

  let noteListOpen = true
</script>

<section class="card split-card primary-workspace">
  <div class="section-header">
    <div>
      <h2>Notes</h2>
      <p class="subtext">Local and pulled notes with explicit origin/state labels for sync trust.</p>
    </div>
    <div class="actions section-actions">
      <button type="button" class="secondary" on:click={() => (noteListOpen = !noteListOpen)}>
        {noteListOpen ? 'Hide List' : 'Show List'}
      </button>
      <button type="button" class="secondary" on:click={resetNoteEditor} disabled={noteBusy}>
        New Note
      </button>
    </div>
  </div>

  {#if noteListOpen}
    <div class="list-toolbar notes-toolbar">
      <input type="text" bind:value={noteSearchQuery} placeholder="Search notes by title, path, state, or content" />
      <select bind:value={noteSortMode} aria-label="Sort notes">
        <option value="updated">Recently updated</option>
        <option value="title">Title A-Z</option>
        <option value="path">Path A-Z</option>
      </select>
      <span class="muted list-count">Showing {sortedNotes.length} of {notes.length} notes</span>
    </div>
  {/if}

  <div class="note-mobile-layout" class:note-list-collapsed={!noteListOpen}>
    {#if noteListOpen}
      <div class="list-panel">
        {#if sortedNotes.length === 0}
          <p class="empty-state">
            {notes.length === 0 ? 'No local notes yet.' : 'No notes match the current search.'}
          </p>
        {:else}
          {#each sortedNotes as note}
            <button
              type="button"
              class:selected-item={!noteDraftActive && editingNotePath === note.path}
              class="list-item"
              on:click={() => editNote(note)}
              disabled={noteBusy}
            >
              <strong>{note.title}</strong>
              <span>{note.path}</span>
              <div class="badge-row">
                <span class="pill source-pill {note.syncOrigin}">{note.syncOrigin}</span>
                <span class="pill state-pill {note.syncState}">{describeSyncState(note.syncState)}</span>
              </div>
              <span class="muted">Updated {formatTimestamp(note.updatedAt)}</span>
              {#if note.lastSyncedAt}
                <span class="muted">Last synced {formatTimestamp(note.lastSyncedAt)}</span>
              {/if}
            </button>
          {/each}
        {/if}
      </div>
    {/if}

    <div class="editor-panel note-reader-panel" class:has-selection={editingNotePath !== null || noteDraftActive}>
      <div class="detail-summary">
        <div class="reader-title-row">
          <strong>{noteDraftActive ? 'New note draft' : noteTitle || 'Select a note'}</strong>
          <div class="note-mode-toggle" aria-label="Note mode">
            <button type="button" class:active-tab={noteMode === 'read'} on:click={() => (noteMode = 'read')}>
              Read
            </button>
            <button type="button" class:active-tab={noteMode === 'edit'} on:click={() => (noteMode = 'edit')}>
              Edit
            </button>
          </div>
        </div>
        <div class="reader-meta-row">
          <span class="muted">{formatNoteStats(noteContent)}</span>
          {#if selectedNote}
            <div class="badge-row">
              <span class="pill source-pill {selectedNote.syncOrigin}">{selectedNote.syncOrigin}</span>
              <span class="pill state-pill {selectedNote.syncState}">
                {describeSyncState(selectedNote.syncState)}
              </span>
            </div>
          {/if}
        </div>
      </div>

      {#if noteMode === 'read'}
        <article class="note-preview rendered-markdown" aria-label="Rendered note">
          <span class="scanline" aria-hidden="true"></span>
          <span class="holo-corner top-left">MD::RENDER</span>
          <span class="holo-corner bottom-right">X: 982 · Y: 17 · Z: 4</span>
          {@html renderedNoteContent}
        </article>
      {:else}
        <label>
          Path
          <input type="text" bind:value={notePath} placeholder="notes/mobile/hub.md" />
        </label>

        <label>
          Title
          <input type="text" bind:value={noteTitle} placeholder="Optional title override" />
        </label>

        <label>
          Content
          <textarea bind:value={noteContent} rows="12" placeholder="# Mobile note"></textarea>
        </label>
      {/if}

      <div class="actions">
        <button type="button" on:click={saveNote} disabled={noteBusy}>Save Note</button>
        <button
          type="button"
          class="danger"
          on:click={removeCurrentNote}
          disabled={noteBusy || !notePath.trim()}
        >
          Delete Note
        </button>
      </div>
    </div>
  </div>
</section>
