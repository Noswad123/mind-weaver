<script lang="ts">
  import { onMount } from 'svelte'
  import DOMPurify from 'dompurify'
  import { marked } from 'marked'
  import HiveMindMark from './HiveMindMark.svelte'
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

  type StatusMessage = {
    kind: 'idle' | 'ok' | 'error'
    text: string
  }

  type AppTab = 'notes' | 'todos' | 'sync' | 'conflicts' | 'settings'
  type NoteMode = 'read' | 'edit'

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

  const LOCAL_OP_TIMEOUT_MS = 8000

  const formatTimestamp = (value: string | null | undefined): string => {
    if (!value) {
      return '—'
    }

    const timestamp = new Date(value)
    return Number.isNaN(timestamp.getTime()) ? value : timestamp.toLocaleString()
  }

  const deriveNoteTitle = (path: string): string => {
    const lastSegment = path.split('/').pop() ?? path
    return lastSegment.replace(/\.[^.]+$/, '').trim() || 'Untitled note'
  }

  const createLocalID = (): string => {
    if (typeof globalThis.crypto !== 'undefined') {
      if (typeof globalThis.crypto.randomUUID === 'function') {
        return globalThis.crypto.randomUUID()
      }

      if (typeof globalThis.crypto.getRandomValues === 'function') {
        const bytes = new Uint8Array(16)
        globalThis.crypto.getRandomValues(bytes)
        bytes[6] = (bytes[6] & 0x0f) | 0x40
        bytes[8] = (bytes[8] & 0x3f) | 0x80
        const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
        return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
      }
    }

    return `local-${Date.now()}-${Math.random().toString(16).slice(2, 12)}`
  }

  const withTimeout = async <T>(promise: Promise<T>, label: string): Promise<T> => {
    return await Promise.race([
      promise,
      new Promise<T>((_, reject) => {
        window.setTimeout(() => reject(new Error(`${label} timed out after ${LOCAL_OP_TIMEOUT_MS / 1000}s`)), LOCAL_OP_TIMEOUT_MS)
      }),
    ])
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

  const countSyncState = (
    collection: Array<{ syncState: SyncState }>,
    target: SyncState,
  ): number => collection.filter((item) => item.syncState === target).length

  const describeSyncState = (state: SyncState): string => {
    switch (state) {
      case 'local-only':
        return 'Local only'
      case 'queued':
        return 'Queued'
      case 'synced':
        return 'Synced'
      case 'conflict':
        return 'Conflict'
    }
  }

  const describeConflictReason = (reason: string): string => {
    if (reason.trim() === 'base_version_mismatch') {
      return 'Remote version changed before your push finished.'
    }

    return reason.trim() || 'Conflict detected.'
  }

  const formatConflictTarget = (conflict: LocalSyncConflict): string =>
    `${conflict.entityType}:${conflict.entityID}`

  const formatNoteStats = (content: string): string => {
    const lineCount = content === '' ? 0 : content.split(/\r?\n/).length
    return `${content.length} chars · ${lineCount} lines`
  }

  const renderMarkdown = (markdown: string): string => {
    const rendered = marked.parse(markdown || '_Nothing to preview yet._', {
      async: false,
      breaks: true,
      gfm: true,
    }) as string

    return DOMPurify.sanitize(rendered)
  }

  const buildTodoSyncPayload = (todo: LocalTodo): string =>
    JSON.stringify({
      id: todo.id,
      source_id: todo.sourceID,
      source_path: todo.sourcePath,
      task_scope: todo.taskScope,
      task_area: todo.taskArea,
      domains: ['task-index'],
      task_active: true,
      todo_section: todo.todoSection,
      text: todo.text,
      done: todo.done,
      meta: todo.meta,
      order: todo.order,
      updated_at: todo.updatedAt,
    })

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

  $: renderedNoteContent = renderMarkdown(noteContent)

  $: selectedTodo = selectedTodoID ? todos.find((todo) => todo.id === selectedTodoID) ?? null : null

  $: if (selectedTodo) {
    todoEditorText = selectedTodo.text
    todoEditorSection = selectedTodo.todoSection
    todoEditorDone = selectedTodo.done
    todoEditorMeta = selectedTodo.meta
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
    editingNotePath = null
    notePath = ''
    noteTitle = ''
    noteContent = ''
    noteMode = 'edit'
  }

  const closeNoteReader = (): void => {
    resetNoteEditor()
    noteMode = 'read'
  }

  const editNote = (note: LocalNote): void => {
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
  <header class="app-header">
    <button type="button" class="title-toggle" on:click={() => (tabsOpen = !tabsOpen)} aria-expanded={tabsOpen}>
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

  {#if !hasCachedCredentials}
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
  {:else}

  {#if tabsOpen}
  <nav class="tab-nav" aria-label="Primary sections">
    <button
      type="button"
      class:active-tab={activeTab === 'notes'}
      on:click={() => {
        activeTab = 'notes'
        if (editingNotePath) {
          closeNoteReader()
        }
      }}
    >
      Notes
    </button>
    <button
      type="button"
      class:active-tab={activeTab === 'todos'}
      on:click={() => {
        activeTab = 'todos'
        if (selectedTodoID) {
          resetTodoEditor()
        }
      }}
    >
      Todos
    </button>
    <button type="button" class:active-tab={activeTab === 'sync'} on:click={() => (activeTab = 'sync')}>
      Sync
    </button>
    <button type="button" class:active-tab={activeTab === 'conflicts'} on:click={() => (activeTab = 'conflicts')}>
      Conflicts
    </button>
    <button type="button" class:active-tab={activeTab === 'settings'} on:click={() => (activeTab = 'settings')}>
      Settings
    </button>
  </nav>
  {/if}

  {#if activeTab === 'settings'}
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
      Settings, local notes, local todos, queue state, and last sync cursor are stored in
      browser IndexedDB on this device.
    </p>

    <div class="actions">
      <button type="button" on:click={onValidateToken} disabled={remoteBusy}>Validate Token</button>
      <button type="button" on:click={onCheckSyncState} disabled={remoteBusy}>Check Sync State</button>
      <button type="button" on:click={onManualSync} disabled={remoteBusy}>Manual Sync</button>
      <button type="button" class="danger" on:click={onLogout} disabled={remoteBusy}>Forget Token</button>
    </div>
  </section>
  {/if}

  {#if activeTab === 'sync'}
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
        <strong>{notes.length}</strong>
      </div>
      <div class="stat">
        <span class="stat-label">Local todos</span>
        <strong>{todos.length}</strong>
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
  {/if}

  {#if activeTab === 'conflicts'}
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
        Some mobile changes hit sync conflicts. Review the recent list below, then use desktop
        tooling like <code>mw sync conflicts review --export-dir ~/.local/share/mw/conflicts/reviews</code>
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
  {/if}

  {#if activeTab === 'notes'}
  <section class="card split-card primary-workspace">
    {#if !editingNotePath}
    <div class="section-header">
      <div>
        <h2>Notes</h2>
        <p class="subtext">Local and pulled notes with explicit origin/state labels for sync trust.</p>
      </div>
      <button type="button" class="secondary" on:click={resetNoteEditor} disabled={noteBusy}>
        New Note
      </button>
    </div>

    <div class="list-toolbar">
      <input type="text" bind:value={noteSearchQuery} placeholder="Search notes by title, path, state, or content" />
      <select bind:value={noteSortMode} aria-label="Sort notes">
        <option value="updated">Recently updated</option>
        <option value="title">Title A-Z</option>
        <option value="path">Path A-Z</option>
      </select>
      <span class="muted">Showing {sortedNotes.length} of {notes.length} notes</span>
    </div>
    {/if}

    <div class="note-mobile-layout" class:selected-note-layout={editingNotePath !== null}>
      {#if !editingNotePath}
      <div class="list-panel">
        {#if sortedNotes.length === 0}
          <p class="empty-state">
            {notes.length === 0 ? 'No local notes yet.' : 'No notes match the current search.'}
          </p>
        {:else}
          {#each sortedNotes as note}
            <button
              type="button"
              class:selected-item={editingNotePath === note.path}
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

      <div class="editor-panel note-reader-panel" class:has-selection={editingNotePath !== null}>
        <div class="detail-summary">
          <div class="reader-title-row">
            <strong>{editingNotePath ? noteTitle : 'New note draft'}</strong>
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
  {/if}

  {#if activeTab === 'todos'}
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

  {/if}
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
  {/if}
</main>
