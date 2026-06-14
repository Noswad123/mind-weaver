<script lang="ts">
  import { onMount } from 'svelte'
  import AppHeader from './components/AppHeader.svelte'
  import ConflictsPanel from './components/ConflictsPanel.svelte'
  import LoginCard from './components/LoginCard.svelte'
  import NotesWorkspace from './components/NotesWorkspace.svelte'
  import SettingsPanel from './components/SettingsPanel.svelte'
  import StatusRail from './components/StatusRail.svelte'
  import SyncPanel from './components/SyncPanel.svelte'
  import TabNav from './components/TabNav.svelte'
  import TodosWorkspace from './components/TodosWorkspace.svelte'
  import type { AppTab, NoteMode, StatusMessage } from './appTypes'
  import {
    buildTodoSyncPayload,
    countSyncState,
    createLocalID,
    deriveNoteTitle,
    formatTimestamp,
    renderMarkdown,
    withTimeout,
  } from './appUtils'
  import { runManualSync } from './hiveSyncEngine'
  import {
    countOpenLocalSyncConflicts,
    countDraftQueueItems,
    deleteLocalNote,
    deleteLocalTodo,
    enqueueDraftQueueOperation,
    getEntityVersion,
    getLocalNote,
    getLocalTodo,
    initializeHiveDB,
    listLocalSyncConflicts,
    listLocalNotes,
    listLocalTodos,
    loadPersistedConfig,
    markLocalSyncConflictReviewed,
    loadPersistedSyncMetadata,
    savePersistedConfig,
    savePersistedSyncMetadata,
    upsertLocalNote,
    upsertLocalTodo,
    type EntityType,
    type LocalSyncConflict,
    type LocalNote,
    type LocalTodo,
    type SyncState,
    type TodoSection,
  } from './hiveStorage'
  import { fetchSyncState, registerDevice } from './syncApi'

  const DEFAULT_ENDPOINT =
    import.meta.env.VITE_HIVE_SYNC_API_URL ||
    'https://hive-sync-api-wr23e5lyna-ue.a.run.app'
  const DEFAULT_DEVICE_ID = 'personal'

  let endpoint = DEFAULT_ENDPOINT
  let deviceID = DEFAULT_DEVICE_ID
  let token = ''
  let hasCachedCredentials = false
  let remoteBusy = false
  let noteBusy = false
  let todoBusy = false
  let conflictBusy = false
  let activeTab: AppTab = 'notes'
  let tabsOpen = true
  let isOnline = typeof navigator === 'undefined' ? true : navigator.onLine

  let healthMessage: StatusMessage = { kind: 'idle', text: 'Ready' }
  let remoteStateMessage: StatusMessage = {
    kind: 'idle',
    text: 'Remote sync-state has not been checked yet.',
  }
  let storageMessage: StatusMessage = {
    kind: 'idle',
    text: 'Preparing local IndexedDB storage…',
  }
  let workspaceMessage: StatusMessage = {
    kind: 'idle',
    text: 'Local workspace is empty.',
  }

  let localCursor = '—'
  let remoteCursor = '—'
  let lastObservedServerTime = '—'
  let lastRemoteCheckAt = '—'
  let lastSuccessfulSyncAt = '—'
  let pendingDraftCount = 0
  let localOnlyCount = 0
  let queuedCount = 0
  let syncedCount = 0
  let conflictCount = 0

  let notes: LocalNote[] = []
  let todos: LocalTodo[] = []
  let recentConflicts: LocalSyncConflict[] = []
  let openConflictReviewCount = 0
  let noteSearchQuery = ''
  let noteSortMode = 'updated'
  let noteMode: NoteMode = 'read'
  let noteDraftActive = false
  let todoSearchQuery = ''
  let todoFilter = 'all'

  let editingNotePath: string | null = null
  let notePath = ''
  let noteTitle = ''
  let noteContent = ''

  let selectedTodoID: string | null = null
  let todoText = ''
  let todoSection: TodoSection = 'Inbox'
  let todoEditorText = ''
  let todoEditorSection: TodoSection = 'Inbox'
  let todoEditorDone = false
  let todoEditorMeta = ''

  $: filteredNotes = notes.filter((note) => {
    const query = noteSearchQuery.trim().toLowerCase()
    if (query === '') {
      return true
    }

    return [note.title, note.path, note.content, note.syncOrigin, note.syncState].some((value) =>
      value.toLowerCase().includes(query),
    )
  })

  $: sortedNotes = [...filteredNotes].sort((left, right) => {
    switch (noteSortMode) {
      case 'title':
        return left.title.localeCompare(right.title)
      case 'path':
        return left.path.localeCompare(right.path)
      default:
        return right.updatedAt.localeCompare(left.updatedAt)
    }
  })

  $: filteredTodos = todos.filter((todo) => {
    const query = todoSearchQuery.trim().toLowerCase()
    const matchesSearch =
      query === '' ||
      [todo.text, todo.todoSection, todo.syncOrigin, todo.syncState, todo.meta].some((value) =>
        value.toLowerCase().includes(query),
      )

    if (!matchesSearch) {
      return false
    }

    switch (todoFilter) {
      case 'open':
        return !todo.done
      case 'done':
        return todo.done
      case 'attention':
        return todo.syncState === 'conflict' || todo.syncState === 'queued'
      case 'Inbox':
      case 'Next':
      case 'Waiting':
        return todo.todoSection === todoFilter
      default:
        return true
    }
  })

  $: selectedNote = editingNotePath
    ? notes.find((note) => note.path === editingNotePath) ?? null
    : null

  $: if (
    !noteDraftActive &&
    sortedNotes.length > 0 &&
    (!editingNotePath || !sortedNotes.some((note) => note.path === editingNotePath))
  ) {
    editNote(sortedNotes[0])
  }

  $: renderedNoteContent = renderMarkdown(noteContent)

  $: selectedTodo = selectedTodoID ? todos.find((todo) => todo.id === selectedTodoID) ?? null : null

  $: if (selectedTodo) {
    todoEditorText = selectedTodo.text
    todoEditorSection = selectedTodo.todoSection
    todoEditorDone = selectedTodo.done
    todoEditorMeta = selectedTodo.meta
  }

  const reportStorageError = (error: unknown): void => {
    const message = error instanceof Error ? error.message : String(error)
    storageMessage = {
      kind: 'error',
      text: `IndexedDB unavailable: ${message}`,
    }
  }

  const updateConnectivityState = (): void => {
    isOnline = typeof navigator === 'undefined' ? true : navigator.onLine
  }

  const refreshLocalWorkspace = async (): Promise<void> => {
    const [nextNotes, nextTodos, nextConflicts, nextPendingCount, nextOpenConflictCount] =
      await withTimeout(
        Promise.all([
          listLocalNotes(),
          listLocalTodos(),
          listLocalSyncConflicts(),
          countDraftQueueItems(),
          countOpenLocalSyncConflicts(),
        ]),
        'local workspace refresh',
      )

    notes = nextNotes
    todos = nextTodos
    recentConflicts = nextConflicts
    pendingDraftCount = nextPendingCount
    openConflictReviewCount = nextOpenConflictCount
    localOnlyCount = countSyncState(notes, 'local-only') + countSyncState(todos, 'local-only')
    queuedCount = countSyncState(notes, 'queued') + countSyncState(todos, 'queued')
    syncedCount = countSyncState(notes, 'synced') + countSyncState(todos, 'synced')
    conflictCount = countSyncState(notes, 'conflict') + countSyncState(todos, 'conflict')

    workspaceMessage = {
      kind: 'ok',
      text: `Loaded ${notes.length} note(s), ${todos.length} todo(s), ${syncedCount} synced item(s), ${pendingDraftCount} queued sync op(s).`,
    }
  }

  const reviewConflict = async (conflictID: string): Promise<void> => {
    conflictBusy = true
    try {
      await markLocalSyncConflictReviewed(conflictID)
      await refreshLocalWorkspace()
      workspaceMessage = {
        kind: 'ok',
        text: 'Marked conflict as reviewed in the local mobile log.',
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
      workspaceMessage = { kind: 'error', text: `Review conflict failed: ${message}` }
    } finally {
      conflictBusy = false
    }
  }

  const resetTodoEditor = (): void => {
    selectedTodoID = null
    todoEditorText = ''
    todoEditorSection = 'Inbox'
    todoEditorDone = false
    todoEditorMeta = ''
  }

  const selectTodo = (todo: LocalTodo): void => {
    selectedTodoID = todo.id
  }

  const persistConfig = async (): Promise<void> => {
    try {
      await withTimeout(
        savePersistedConfig({
          endpoint: endpoint.trim(),
          deviceID: deviceID.trim(),
          token: token.trim(),
        }),
        'settings persistence',
      )
      storageMessage = {
        kind: 'ok',
        text: 'Credentials cached locally in this browser.',
      }
      hasCachedCredentials = endpoint.trim() !== '' && deviceID.trim() !== '' && token.trim() !== ''
    } catch (error) {
      reportStorageError(error)
    }
  }

  const queueDraftOperation = async (
    entityType: EntityType,
    entityID: string,
    opType: 'upsert' | 'delete',
    payload: string,
  ): Promise<void> => {
    const baseVersion = await withTimeout(
      getEntityVersion(entityType, entityID),
      `entity version lookup for ${entityType}:${entityID}`,
    )
    const opID = createLocalID()

    await withTimeout(
      enqueueDraftQueueOperation({
        id: opID,
        idempotencyKey: opID,
        entityType,
        entityID,
        opType,
        payload,
        baseVersion,
        clientUpdatedAt: new Date().toISOString(),
      }),
      `queue draft operation for ${entityType}:${entityID}`,
    )
  }

  const resetNoteEditor = (): void => {
    noteDraftActive = true
    editingNotePath = null
    notePath = ''
    noteTitle = ''
    noteContent = ''
    noteMode = 'edit'
  }

  const editNote = (note: LocalNote): void => {
    noteDraftActive = false
    editingNotePath = note.path
    notePath = note.path
    noteTitle = note.title
    noteContent = note.content
    noteMode = 'read'
  }

  const saveNote = async (): Promise<void> => {
    noteBusy = true
    try {
      const path = notePath.trim()
      if (!path) {
        workspaceMessage = { kind: 'error', text: 'Note path is required.' }
        return
      }

      const existingNote = await withTimeout(getLocalNote(path), `load note ${path}`)

      const note: LocalNote = {
        path,
        title: noteTitle.trim() || deriveNoteTitle(path),
        content: noteContent,
        tags: ['mobile', 'phase-2'],
        domains: ['hive-mind'],
        syncOrigin: existingNote?.syncOrigin ?? 'local',
        syncState: existingNote?.syncState === 'conflict' ? 'conflict' : 'queued',
        lastSyncedAt: existingNote?.lastSyncedAt ?? null,
        updatedAt: new Date().toISOString(),
      }

      await withTimeout(upsertLocalNote(note), `save note ${note.path}`)
      await queueDraftOperation(
        'note',
        note.path,
        'upsert',
        JSON.stringify({
          path: note.path,
          title: note.title,
          content: note.content,
          tags: note.tags,
          domains: note.domains,
        }),
      )
      await refreshLocalWorkspace()

      editingNotePath = note.path
      noteDraftActive = false
      noteSearchQuery = ''
      noteTitle = note.title
      workspaceMessage = {
        kind: 'ok',
        text: `Saved local note ${note.path} and queued it for sync.`,
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
      workspaceMessage = { kind: 'error', text: `Save note failed: ${message}` }
    } finally {
      noteBusy = false
    }
  }

  const removeCurrentNote = async (): Promise<void> => {
    noteBusy = true
    try {
      const path = notePath.trim()
      if (!path) {
        workspaceMessage = { kind: 'error', text: 'Choose a note to delete.' }
        return
      }

      await withTimeout(deleteLocalNote(path), `delete note ${path}`)
      await queueDraftOperation('note', path, 'delete', JSON.stringify({ path }))
      await refreshLocalWorkspace()
      resetNoteEditor()

      workspaceMessage = {
        kind: 'ok',
        text: `Deleted local note ${path} and queued the delete for sync.`,
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
      workspaceMessage = { kind: 'error', text: `Delete note failed: ${message}` }
    } finally {
      noteBusy = false
    }
  }

  const createTodo = async (): Promise<void> => {
    todoBusy = true
    try {
      const text = todoText.trim()
      if (!text) {
        workspaceMessage = { kind: 'error', text: 'Todo text is required.' }
        return
      }

      const id = createLocalID()
      const todo: LocalTodo = {
        id,
        sourceID: id,
        sourcePath: 'mobile/pwa',
        taskScope: 'project',
        taskArea: 'PWA',
        todoSection,
        text,
        done: false,
        meta: 'phase-2 pwa',
        order: todos.length + 1,
        line: null,
        syncOrigin: 'local',
        syncState: 'queued',
        lastSyncedAt: null,
        updatedAt: new Date().toISOString(),
      }

      await withTimeout(upsertLocalTodo(todo), `save todo ${todo.id}`)
      await queueDraftOperation(
        'todo',
        todo.id,
        'upsert',
        buildTodoSyncPayload(todo),
      )

      todoText = ''
      selectedTodoID = todo.id
      todoSearchQuery = ''
      todoFilter = 'all'
      await refreshLocalWorkspace()
      workspaceMessage = {
        kind: 'ok',
        text: `Added todo and queued it for sync (${todo.todoSection}).`,
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
      workspaceMessage = { kind: 'error', text: `Create todo failed: ${message}` }
    } finally {
      todoBusy = false
    }
  }

  const toggleTodo = async (todo: LocalTodo): Promise<void> => {
    todoBusy = true
    try {
      const existingTodo = await withTimeout(getLocalTodo(todo.id), `load todo ${todo.id}`)
      const updated: LocalTodo = {
        ...todo,
        done: !todo.done,
        syncState: existingTodo?.syncState === 'conflict' ? 'conflict' : 'queued',
        updatedAt: new Date().toISOString(),
      }

      await withTimeout(upsertLocalTodo(updated), `save todo ${updated.id}`)
      await queueDraftOperation(
        'todo',
        updated.id,
        'upsert',
        buildTodoSyncPayload(updated),
      )

      await refreshLocalWorkspace()
      workspaceMessage = {
        kind: 'ok',
        text: `Updated todo ${updated.text} and queued the change for sync.`,
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
      workspaceMessage = { kind: 'error', text: `Update todo failed: ${message}` }
    } finally {
      todoBusy = false
    }
  }

  const removeTodo = async (todo: LocalTodo): Promise<void> => {
    todoBusy = true
    try {
      await withTimeout(deleteLocalTodo(todo.id), `delete todo ${todo.id}`)
      await queueDraftOperation('todo', todo.id, 'delete', JSON.stringify({ id: todo.id }))
      if (selectedTodoID === todo.id) {
        resetTodoEditor()
      }
      await refreshLocalWorkspace()

      workspaceMessage = {
        kind: 'ok',
        text: `Deleted todo ${todo.text} and queued the delete for sync.`,
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
      workspaceMessage = { kind: 'error', text: `Delete todo failed: ${message}` }
    } finally {
      todoBusy = false
    }
  }

  const saveSelectedTodo = async (): Promise<void> => {
    if (!selectedTodo) {
      workspaceMessage = { kind: 'error', text: 'Select a todo to edit.' }
      return
    }

    todoBusy = true
    try {
      const text = todoEditorText.trim()
      if (!text) {
        workspaceMessage = { kind: 'error', text: 'Todo text is required.' }
        return
      }

      const updated: LocalTodo = {
        ...selectedTodo,
        text,
        todoSection: todoEditorSection,
        done: todoEditorDone,
        meta: todoEditorMeta.trim(),
        syncState: selectedTodo.syncState === 'conflict' ? 'conflict' : 'queued',
        updatedAt: new Date().toISOString(),
      }

      await withTimeout(upsertLocalTodo(updated), `save todo ${updated.id}`)
      await queueDraftOperation('todo', updated.id, 'upsert', buildTodoSyncPayload(updated))
      await refreshLocalWorkspace()
      workspaceMessage = {
        kind: 'ok',
        text: `Saved todo changes for ${updated.text} and queued them for sync.`,
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
      workspaceMessage = { kind: 'error', text: `Save todo changes failed: ${message}` }
    } finally {
      todoBusy = false
    }
  }

  onMount(() => {
    const syncConnectivity = (): void => {
      updateConnectivityState()
    }

    syncConnectivity()

    void (async () => {
      try {
        await initializeHiveDB()

        const legacyEndpoint = window.localStorage.getItem('hive.endpoint') ?? DEFAULT_ENDPOINT
        const legacyDeviceID = window.localStorage.getItem('hive.deviceId') ?? DEFAULT_DEVICE_ID
        const persistedConfig = await loadPersistedConfig()
        const persistedSyncMetadata = await loadPersistedSyncMetadata()

        endpoint = persistedConfig?.endpoint ?? legacyEndpoint
        deviceID = persistedConfig?.deviceID ?? legacyDeviceID
        token = persistedConfig?.token ?? ''
        hasCachedCredentials = endpoint.trim() !== '' && deviceID.trim() !== '' && token.trim() !== ''
        localCursor = persistedSyncMetadata?.lastAppliedCursor ?? '—'
        remoteCursor = persistedSyncMetadata?.lastObservedRemoteCursor ?? '—'
        lastObservedServerTime = formatTimestamp(persistedSyncMetadata?.lastObservedServerTime)
        lastRemoteCheckAt = formatTimestamp(persistedSyncMetadata?.lastRemoteCheckAt)
        lastSuccessfulSyncAt = formatTimestamp(persistedSyncMetadata?.lastSuccessfulSyncAt)
        remoteStateMessage = {
          kind: persistedSyncMetadata?.lastRemoteCheckKind ?? 'idle',
          text:
            persistedSyncMetadata?.lastRemoteCheckMessage ??
            'Remote sync-state has not been checked yet.',
        }

        if (!persistedConfig) {
          await withTimeout(
            savePersistedConfig({
              endpoint: legacyEndpoint,
              deviceID: legacyDeviceID,
              token: '',
            }),
            'initial settings persistence',
          )
          hasCachedCredentials = false
        }

        await refreshLocalWorkspace()
        storageMessage = {
          kind: 'ok',
          text: 'IndexedDB schema ready (settings, sync metadata, local notes, todos, queue).',
        }
      } catch (error) {
        reportStorageError(error)
      }
    })()

    window.addEventListener('online', syncConnectivity)
    window.addEventListener('offline', syncConnectivity)

    return () => {
      window.removeEventListener('online', syncConnectivity)
      window.removeEventListener('offline', syncConnectivity)
    }
  })

  const selectTab = (tab: AppTab): void => {
    activeTab = tab
    if (tab === 'todos' && selectedTodoID) {
      resetTodoEditor()
    }
  }

  const toggleTabs = (): void => {
    tabsOpen = !tabsOpen
  }

  const updateEndpoint = (value: string): void => {
    endpoint = value
    void persistConfig()
  }

  const updateDeviceID = (value: string): void => {
    deviceID = value
    void persistConfig()
  }

  const onEndpointInput = (event: Event): void => {
    updateEndpoint((event.currentTarget as HTMLInputElement).value)
  }

  const onDeviceIDInput = (event: Event): void => {
    updateDeviceID((event.currentTarget as HTMLInputElement).value)
  }

  const onTokenInput = (event: Event): void => {
    token = (event.currentTarget as HTMLInputElement).value
    void persistConfig()
  }

  const onValidateToken = async (): Promise<void> => {
    remoteBusy = true
    try {
      const result = await registerDevice(endpoint.trim(), deviceID.trim(), token.trim())
      if (!result.ok) {
        healthMessage = {
          kind: 'error',
          text: `Token check failed (${result.status}): ${result.error || 'unknown error'}`,
        }
        return
      }

      healthMessage = {
        kind: 'ok',
        text: `Token valid for device_id=${deviceID.trim()}`,
      }
      await persistConfig()
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
      healthMessage = { kind: 'error', text: `Token check failed: ${message}` }
    } finally {
      remoteBusy = false
    }
  }

  const onLogout = async (): Promise<void> => {
    token = ''
    hasCachedCredentials = false
    await persistConfig()
    healthMessage = { kind: 'idle', text: 'Signed out locally. Enter your bearer token to sync again.' }
    activeTab = 'settings'
  }

  const onCheckSyncState = async (): Promise<void> => {
    remoteBusy = true
    try {
      const result = await fetchSyncState(endpoint.trim(), token.trim())
      const checkedAt = new Date().toISOString()

      if (!result.ok || !result.data) {
        const remoteCheckText =
          `Remote sync-state failed (${result.status}): ${result.error || 'unknown error'}`

        lastRemoteCheckAt = formatTimestamp(checkedAt)
        remoteStateMessage = {
          kind: 'error',
          text: remoteCheckText,
        }

        await withTimeout(
          savePersistedSyncMetadata({
            lastRemoteCheckAt: checkedAt,
            lastRemoteCheckKind: 'error',
            lastRemoteCheckMessage: remoteCheckText,
          }),
          'sync-state error persistence',
        )

        healthMessage = {
          kind: 'error',
          text: `Sync state failed (${result.status}): ${result.error || 'unknown error'}`,
        }
        return
      }

      const syncedAt = checkedAt
      const remoteCheckText = `Remote sync-state ok (server cursor ${result.data.latest_cursor})`

      remoteCursor = result.data.latest_cursor
      lastObservedServerTime = formatTimestamp(result.data.server_time)
      lastRemoteCheckAt = formatTimestamp(checkedAt)
      lastSuccessfulSyncAt = formatTimestamp(syncedAt)
      remoteStateMessage = {
        kind: 'ok',
        text: remoteCheckText,
      }
      healthMessage = {
        kind: 'ok',
        text: `Sync state ok (server cursor ${result.data.latest_cursor})`,
      }

      try {
        await withTimeout(
          savePersistedSyncMetadata({
            lastObservedRemoteCursor: result.data.latest_cursor,
            lastObservedServerTime: result.data.server_time,
            lastRemoteCheckAt: checkedAt,
            lastRemoteCheckKind: 'ok',
            lastRemoteCheckMessage: remoteCheckText,
            lastSuccessfulSyncAt: syncedAt,
          }),
          'sync-state persistence',
        )
        storageMessage = {
          kind: 'ok',
          text: 'Sync metadata persisted locally in IndexedDB.',
        }
      } catch (error) {
        reportStorageError(error)
      }
    } catch (error) {
      const checkedAt = new Date().toISOString()
      const message = error instanceof Error ? error.message : String(error)
      const remoteCheckText = `Remote sync-state failed: ${message}`

      lastRemoteCheckAt = formatTimestamp(checkedAt)
      remoteStateMessage = {
        kind: 'error',
        text: remoteCheckText,
      }
      await withTimeout(
        savePersistedSyncMetadata({
          lastRemoteCheckAt: checkedAt,
          lastRemoteCheckKind: 'error',
          lastRemoteCheckMessage: remoteCheckText,
        }),
        'sync-state failure persistence',
      )

      healthMessage = { kind: 'error', text: `Sync state failed: ${message}` }
    } finally {
      remoteBusy = false
    }
  }

  const onManualSync = async (): Promise<void> => {
    remoteBusy = true
    try {
      const result = await runManualSync({
        endpoint: endpoint.trim(),
        token: token.trim(),
        deviceID: deviceID.trim(),
      })

      const persistedSyncMetadata = await withTimeout(
        loadPersistedSyncMetadata(),
        'load sync metadata after manual sync',
      )
      localCursor = persistedSyncMetadata?.lastAppliedCursor ?? result.finalCursor
      remoteCursor = persistedSyncMetadata?.lastObservedRemoteCursor ?? result.serverLatestCursor
      lastObservedServerTime = formatTimestamp(result.serverTime)
      lastRemoteCheckAt = formatTimestamp(new Date().toISOString())
      lastSuccessfulSyncAt = formatTimestamp(persistedSyncMetadata?.lastSuccessfulSyncAt)
      remoteStateMessage = {
        kind: 'ok',
        text: `Remote sync-state ok after manual sync (server cursor ${result.serverLatestCursor})`,
      }

      await withTimeout(
        savePersistedSyncMetadata({
          lastObservedRemoteCursor: result.serverLatestCursor,
          lastObservedServerTime: result.serverTime,
          lastRemoteCheckAt: new Date().toISOString(),
          lastRemoteCheckKind: 'ok',
          lastRemoteCheckMessage: `Remote sync-state ok after manual sync (server cursor ${result.serverLatestCursor})`,
        }),
        'manual sync metadata persistence',
      )

      await refreshLocalWorkspace()

      healthMessage = {
        kind: 'ok',
        text: `Manual sync complete: pushed ${result.pushedAccepted}, pulled ${result.pulledApplied} across ${result.pulledPages} page(s), conflicts ${result.conflictsLogged}.`,
      }
      storageMessage = {
        kind: 'ok',
        text: `Local cursor persisted at ${result.finalCursor} (server latest ${result.serverLatestCursor}).`,
      }
    } catch (error) {
      const checkedAt = new Date().toISOString()
      const message = error instanceof Error ? error.message : String(error)
      const remoteCheckText = `Manual sync failed before remote state stabilized: ${message}`

      lastRemoteCheckAt = formatTimestamp(checkedAt)
      remoteStateMessage = {
        kind: 'error',
        text: remoteCheckText,
      }
      await withTimeout(
        savePersistedSyncMetadata({
          lastRemoteCheckAt: checkedAt,
          lastRemoteCheckKind: 'error',
          lastRemoteCheckMessage: remoteCheckText,
        }),
        'manual sync failure persistence',
      )

      healthMessage = { kind: 'error', text: `Manual sync failed: ${message}` }
    } finally {
      remoteBusy = false
    }
  }
</script>

<main class="app">
  <AppHeader {tabsOpen} onToggleTabs={toggleTabs} />

  {#if !hasCachedCredentials}
    <LoginCard
      {endpoint}
      {deviceID}
      {token}
      {healthMessage}
      {remoteBusy}
      {onEndpointInput}
      {onDeviceIDInput}
      {onTokenInput}
      {onValidateToken}
    />
  {:else}
    {#if tabsOpen}
      <TabNav {activeTab} onSelectTab={selectTab} />
    {/if}

    {#if activeTab === 'settings'}
      <SettingsPanel
        {endpoint}
        {deviceID}
        {token}
        {remoteBusy}
        {onEndpointInput}
        {onDeviceIDInput}
        {onTokenInput}
        {onValidateToken}
        {onCheckSyncState}
        {onManualSync}
        {onLogout}
      />
    {/if}

    {#if activeTab === 'sync'}
      <SyncPanel
        {isOnline}
        {healthMessage}
        {remoteStateMessage}
        {storageMessage}
        {workspaceMessage}
        {remoteBusy}
        notesCount={notes.length}
        todosCount={todos.length}
        {pendingDraftCount}
        {localOnlyCount}
        {queuedCount}
        {syncedCount}
        {conflictCount}
        {localCursor}
        {remoteCursor}
        {lastRemoteCheckAt}
        {lastObservedServerTime}
        {lastSuccessfulSyncAt}
        {onManualSync}
        {onCheckSyncState}
      />
      <StatusRail {queuedCount} {lastSuccessfulSyncAt} {deviceID} />
      <ConflictsPanel
        {recentConflicts}
        {openConflictReviewCount}
        {conflictBusy}
        {reviewConflict}
      />
    {/if}

    {#if activeTab === 'notes'}
      <NotesWorkspace
        {editingNotePath}
        {noteBusy}
        {sortedNotes}
        {notes}
        {selectedNote}
        {noteDraftActive}
        bind:noteSearchQuery
        bind:noteSortMode
        bind:noteMode
        bind:notePath
        bind:noteTitle
        bind:noteContent
        {renderedNoteContent}
        {resetNoteEditor}
        {editNote}
        {saveNote}
        {removeCurrentNote}
      />
    {/if}

    {#if activeTab === 'todos'}
      <TodosWorkspace
        {selectedTodo}
        {selectedTodoID}
        {todoBusy}
        {todos}
        {filteredTodos}
        bind:todoText
        bind:todoSection
        bind:todoSearchQuery
        bind:todoFilter
        bind:todoEditorText
        bind:todoEditorSection
        bind:todoEditorDone
        bind:todoEditorMeta
        {resetTodoEditor}
        {createTodo}
        {selectTodo}
        {toggleTodo}
        {saveSelectedTodo}
        {removeTodo}
      />
    {/if}
  {/if}
</main>
