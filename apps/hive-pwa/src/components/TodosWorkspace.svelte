<script lang="ts">
  import type { LocalTodo, TodoSection } from '../hiveStorage'
  import { describeSyncState, formatTimestamp } from '../appUtils'

  export let selectedTodo: LocalTodo | null = null
  export let selectedTodoID: string | null = null
  export let todoBusy = false
  export let todos: LocalTodo[] = []
  export let filteredTodos: LocalTodo[] = []
  export let todoText = ''
  export let todoSection: TodoSection = 'Inbox'
  export let todoSearchQuery = ''
  export let todoFilter = 'all'
  export let todoEditorText = ''
  export let todoEditorSection: TodoSection = 'Inbox'
  export let todoEditorDone = false
  export let todoEditorMeta = ''
  export let resetTodoEditor: () => void
  export let createTodo: () => Promise<void>
  export let selectTodo: (todo: LocalTodo) => void
  export let toggleTodo: (todo: LocalTodo) => Promise<void>
  export let saveSelectedTodo: () => Promise<void>
  export let removeTodo: (todo: LocalTodo) => Promise<void>
</script>

<section class="card split-card primary-workspace">
  {#if !selectedTodo}
    <div class="section-header">
      <div>
        <h2>Todos</h2>
        <p class="subtext">Local and pulled todo entities with visible sync state labels.</p>
      </div>
      <button type="button" class="secondary" on:click={resetTodoEditor} disabled={todoBusy}>
        Clear Selection
      </button>
    </div>

    <div class="todo-compose">
      <input type="text" bind:value={todoText} placeholder="Add a todo" />
      <select bind:value={todoSection}>
        <option value="Inbox">Inbox</option>
        <option value="Next">Next</option>
        <option value="Waiting">Waiting</option>
      </select>
      <button type="button" on:click={createTodo} disabled={todoBusy}>Add Todo</button>
    </div>

    <div class="list-toolbar responsive-toolbar">
      <input type="text" bind:value={todoSearchQuery} placeholder="Search todos by text, section, state, or metadata" />
      <select bind:value={todoFilter}>
        <option value="all">All todos</option>
        <option value="open">Open only</option>
        <option value="done">Done only</option>
        <option value="attention">Needs attention</option>
        <option value="Inbox">Inbox</option>
        <option value="Next">Next</option>
        <option value="Waiting">Waiting</option>
      </select>
      <span class="muted">Showing {filteredTodos.length} of {todos.length} todos</span>
    </div>
  {/if}

  <div class="todo-mobile-layout" class:selected-todo-layout={selectedTodo !== null}>
    {#if !selectedTodo}
      <div class="todo-list">
        {#if filteredTodos.length === 0}
          <p class="empty-state">
            {todos.length === 0 ? 'No local todos yet.' : 'No todos match the current filter.'}
          </p>
        {:else}
          {#each filteredTodos as todo}
            <button
              type="button"
              class:selected-item={selectedTodoID === todo.id}
              class="todo-item todo-select"
              on:click={() => selectTodo(todo)}
              disabled={todoBusy}
            >
              <label class="todo-check">
                <input
                  type="checkbox"
                  checked={todo.done}
                  on:change|stopPropagation={() => toggleTodo(todo)}
                  disabled={todoBusy}
                />
                <span class:done={todo.done}>{todo.text}</span>
              </label>

              <div class="todo-meta">
                <div class="badge-row">
                  <span class="pill">{todo.todoSection}</span>
                  <span class="pill source-pill {todo.syncOrigin}">{todo.syncOrigin}</span>
                  <span class="pill state-pill {todo.syncState}">{describeSyncState(todo.syncState)}</span>
                </div>
                <span class="muted">Updated {formatTimestamp(todo.updatedAt)}</span>
                {#if todo.lastSyncedAt}
                  <span class="muted">Last synced {formatTimestamp(todo.lastSyncedAt)}</span>
                {/if}
              </div>
            </button>
          {/each}
        {/if}
      </div>
    {/if}

    <div class="editor-panel todo-detail-panel" class:has-selection={selectedTodo !== null}>
      {#if selectedTodo}
        <div class="detail-summary">
          <div class="reader-title-row">
            <strong class:done={todoEditorDone}>{todoEditorText || selectedTodo.text}</strong>
            <label class="compact-check">
              <input type="checkbox" bind:checked={todoEditorDone} />
              <span>Done</span>
            </label>
          </div>
          <div class="reader-meta-row">
            <span class="muted">{todoEditorSection}</span>
            <div class="badge-row">
              <span class="pill source-pill {selectedTodo.syncOrigin}">{selectedTodo.syncOrigin}</span>
              <span class="pill state-pill {selectedTodo.syncState}">
                {describeSyncState(selectedTodo.syncState)}
              </span>
            </div>
          </div>
        </div>

        <label>
          Task text
          <input type="text" bind:value={todoEditorText} placeholder="Describe the task" />
        </label>

        <label>
          Section
          <select bind:value={todoEditorSection}>
            <option value="Inbox">Inbox</option>
            <option value="Next">Next</option>
            <option value="Waiting">Waiting</option>
          </select>
        </label>

        <label>
          Meta
          <input type="text" bind:value={todoEditorMeta} placeholder="Optional mobile context" />
        </label>

        <div class="actions">
          <button type="button" on:click={saveSelectedTodo} disabled={todoBusy}>Save Todo Changes</button>
          <button type="button" class="danger" on:click={() => removeTodo(selectedTodo)} disabled={todoBusy}>
            Delete Todo
          </button>
        </div>

        <p class="hint">
          Source path: {selectedTodo.sourcePath || 'mobile/pwa'} · Last synced {selectedTodo.lastSyncedAt
            ? formatTimestamp(selectedTodo.lastSyncedAt)
            : 'never'}
        </p>
      {:else}
        <p class="empty-state">Select a todo to edit details, change section, or delete it.</p>
      {/if}
    </div>
  </div>
</section>
