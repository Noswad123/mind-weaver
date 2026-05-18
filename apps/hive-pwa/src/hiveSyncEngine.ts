import {
  deleteLocalNote,
  deleteLocalTodo,
  getLocalNote,
  getLocalTodo,
  incrementEntityVersion,
  listDraftQueueItems,
  loadPersistedSyncMetadata,
  markDraftQueueAttemptFailed,
  removeDraftQueueItem,
  savePersistedSyncMetadata,
  upsertLocalSyncConflict,
  upsertLocalNote,
  upsertLocalTodo,
  type DraftQueueItem,
  type EntityType,
  type LocalNote,
  type LocalTodo,
  type TodoSection,
} from './hiveStorage'
import {
  fetchSyncState,
  pullSyncOperations,
  pushSyncOperations,
  type PullOperation,
  type PushConflict,
} from './syncApi'

type RunManualSyncInput = {
  endpoint: string
  token: string
  deviceID: string
}

export type ManualSyncResult = {
  pushedAccepted: number
  pushedRejected: number
  pulledApplied: number
  pulledPages: number
  conflictsLogged: number
  finalCursor: string
  serverLatestCursor: string
  serverTime: string
}

type NotePayload = {
  path?: string
  title?: string
  content?: string
  tags?: string[]
  domains?: string[]
}

type TodoPayload = {
  id?: string
  source_id?: string
  source_path?: string
  task_scope?: string
  task_area?: string
  todo_section?: string
  text?: string
  done?: boolean
  meta?: string
  order?: number
  line?: number
  updated_at?: string
  title?: string
  status?: string
}

function parsePayload<T>(payload: unknown): T {
  if (typeof payload === 'string') {
    return JSON.parse(payload) as T
  }

  return payload as T
}

function normalizeUniqueStrings(input: string[] | undefined): string[] {
  if (!input) {
    return []
  }

  return [...new Set(input.map((value) => value.trim()).filter(Boolean))].sort((left, right) =>
    left.localeCompare(right),
  )
}

function deriveNoteTitle(path: string): string {
  const lastSegment = path.split('/').pop() ?? path
  return lastSegment.replace(/\.[^.]+$/, '').trim() || 'Untitled note'
}

function sectionFromLegacyStatus(status: string | undefined): TodoSection {
  switch ((status ?? '').trim().toLowerCase()) {
    case 'doing':
      return 'Next'
    case 'done':
    case 'archived':
      return 'Waiting'
    default:
      return 'Inbox'
  }
}

function normalizeTodoSection(value: string | undefined): TodoSection {
  switch ((value ?? '').trim().toLowerCase()) {
    case 'next':
      return 'Next'
    case 'waiting':
      return 'Waiting'
    default:
      return 'Inbox'
  }
}

function buildPushOperations(items: DraftQueueItem[]) {
  return items.map((item) => ({
    op_id: item.id,
    idempotency_key: item.idempotencyKey,
    entity_type: item.entityType,
    entity_id: item.entityID,
    op_type: item.opType,
    payload: JSON.parse(item.payload),
    client_updated_at: item.clientUpdatedAt || item.updatedAt || item.createdAt,
    base_version: item.baseVersion,
  }))
}

async function applyNoteOperation(operation: PullOperation): Promise<string> {
  if (operation.op_type === 'delete') {
    const payload = operation.payload ? parsePayload<{ path?: string }>(operation.payload) : {}
    const path = (operation.entity_id || payload.path || '').trim()
    if (!path) {
      throw new Error('note delete path is required')
    }

    await deleteLocalNote(path)
    return path
  }

  const payload = parsePayload<NotePayload>(operation.payload)
  const path = (payload.path || operation.entity_id || '').trim()
  if (!path) {
    throw new Error('note path is required')
  }

  const note: LocalNote = {
    path,
    title: (payload.title || '').trim() || deriveNoteTitle(path),
    content: payload.content ?? '',
    tags: normalizeUniqueStrings(payload.tags),
    domains: normalizeUniqueStrings(payload.domains),
    syncOrigin: 'remote',
    syncState: 'synced',
    lastSyncedAt: new Date().toISOString(),
    updatedAt: operation.client_updated_at || new Date().toISOString(),
  }

  await upsertLocalNote(note)
  return path
}

async function applyTodoOperation(operation: PullOperation): Promise<string> {
  if (operation.op_type === 'delete') {
    const payload = operation.payload ? parsePayload<{ id?: string }>(operation.payload) : {}
    const id = (operation.entity_id || payload.id || '').trim()
    if (!id) {
      throw new Error('todo delete id is required')
    }

    await deleteLocalTodo(id)
    return id
  }

  const payload = parsePayload<TodoPayload>(operation.payload)
  const id = (payload.id || operation.entity_id || '').trim()
  if (!id) {
    throw new Error('todo id is required')
  }

  const todo: LocalTodo = {
    id,
    sourceID: (payload.source_id || payload.source_path || id).trim(),
    sourcePath: (payload.source_path || '').trim(),
    taskScope: (payload.task_scope || '').trim(),
    taskArea: (payload.task_area || '').trim(),
    todoSection: payload.todo_section
      ? normalizeTodoSection(payload.todo_section)
      : sectionFromLegacyStatus(payload.status),
    text: (payload.text || payload.title || '').trim(),
    done:
      payload.done ||
      ['done', 'archived'].includes((payload.status || '').trim().toLowerCase()),
    meta: (payload.meta || '').trim(),
    order: payload.order ?? 0,
    line: payload.line ?? null,
    syncOrigin: 'remote',
    syncState: 'synced',
    lastSyncedAt: new Date().toISOString(),
    updatedAt:
      (payload.updated_at || operation.client_updated_at || new Date().toISOString()).trim(),
  }

  if (!todo.text) {
    throw new Error('todo text is required')
  }

  await upsertLocalTodo(todo)
  return id
}

async function applyPulledOperation(operation: PullOperation): Promise<void> {
  const entityType = operation.entity_type.trim() as EntityType

  let entityID = ''
  switch (entityType) {
    case 'note':
      entityID = await applyNoteOperation(operation)
      break
    case 'todo':
      entityID = await applyTodoOperation(operation)
      break
    default:
      throw new Error(`unsupported entity_type ${operation.entity_type}`)
  }

  await incrementEntityVersion(entityType, entityID)
}

async function markEntitySynced(
  entityType: EntityType,
  entityID: string,
  syncedAt: string,
): Promise<void> {
  if (entityType === 'note') {
    const note = await getLocalNote(entityID)
    if (!note) {
      return
    }

    await upsertLocalNote({
      ...note,
      syncState: 'synced',
      lastSyncedAt: syncedAt,
      updatedAt: syncedAt,
    })
    return
  }

  const todo = await getLocalTodo(entityID)
  if (!todo) {
    return
  }

  await upsertLocalTodo({
    ...todo,
    syncState: 'synced',
    lastSyncedAt: syncedAt,
    updatedAt: syncedAt,
  })
}

async function markEntityConflict(conflict: PushConflict): Promise<void> {
  const entityType = conflict.entity_type.trim() as EntityType
  const entityID = conflict.entity_id.trim()
  if (!entityID) {
    return
  }

  const createdAt = new Date().toISOString()

  await upsertLocalSyncConflict({
    id: `conflict-${entityType}-${entityID}-${createdAt}`,
    entityType,
    entityID,
    reason: conflict.reason.trim() || 'base_version_mismatch',
    winnerDeviceID: conflict.winner_device_id.trim(),
    loserDeviceID: conflict.loser_device_id.trim(),
    status: 'open',
    createdAt,
    reviewedAt: null,
  })

  if (entityType === 'note') {
    const note = await getLocalNote(entityID)
    if (!note) {
      return
    }

    await upsertLocalNote({
      ...note,
      syncState: 'conflict',
      updatedAt: new Date().toISOString(),
    })
    return
  }

  if (entityType === 'todo') {
    const todo = await getLocalTodo(entityID)
    if (!todo) {
      return
    }

    await upsertLocalTodo({
      ...todo,
      syncState: 'conflict',
      updatedAt: new Date().toISOString(),
    })
  }
}

export async function runManualSync(input: RunManualSyncInput): Promise<ManualSyncResult> {
  const endpoint = input.endpoint.trim()
  const token = input.token.trim()
  const deviceID = input.deviceID.trim()
  const persistedSyncMetadata = await loadPersistedSyncMetadata()

  let pushedAccepted = 0
  let pushedRejected = 0
  let pulledApplied = 0
  let pulledPages = 0
  let conflictsLogged = 0
  let currentCursor = persistedSyncMetadata?.lastAppliedCursor?.trim() || '0'

  const pending = await listDraftQueueItems()
  if (pending.length > 0) {
    const syncedAt = new Date().toISOString()
    const pushResponse = await pushSyncOperations(endpoint, token, {
      device_id: deviceID,
      device_name: deviceID,
      platform: 'web',
      app_version: 'phase-2-pwa',
      operations: buildPushOperations(pending),
    }).catch(async (error: unknown) => {
      const message = error instanceof Error ? error.message : String(error)
      await Promise.all(pending.map((item) => markDraftQueueAttemptFailed(item.id, message)))
      throw error
    })

    const pendingByID = new Map(pending.map((item) => [item.id, item]))
    const rejectedIDs = new Set<string>()

    for (const rejected of pushResponse.rejected) {
      if (!pendingByID.has(rejected.op_id)) {
        continue
      }

      rejectedIDs.add(rejected.op_id)
      pushedRejected += 1
      await markDraftQueueAttemptFailed(rejected.op_id, rejected.reason)
    }

    for (const acceptedID of pushResponse.accepted) {
      if (!pendingByID.has(acceptedID) || rejectedIDs.has(acceptedID)) {
        continue
      }

      pushedAccepted += 1
      const acceptedItem = pendingByID.get(acceptedID)
      await removeDraftQueueItem(acceptedID)
      if (acceptedItem && acceptedItem.opType === 'upsert') {
        await markEntitySynced(acceptedItem.entityType, acceptedItem.entityID, syncedAt)
      }
    }

    conflictsLogged = pushResponse.conflicts.length
    for (const conflict of pushResponse.conflicts) {
      await markEntityConflict(conflict)
    }
  }

  while (true) {
    const pullResponse = await pullSyncOperations(endpoint, token, currentCursor, 25)
    pulledPages += 1
    const previousCursor = currentCursor

    for (const operation of pullResponse.operations) {
      await applyPulledOperation(operation)
    }

    pulledApplied += pullResponse.operations.length

    const nextCursor = pullResponse.next_cursor.trim() || currentCursor
    currentCursor = nextCursor

    if (!pullResponse.has_more) {
      break
    }

    if (pullResponse.has_more && nextCursor === previousCursor && pullResponse.operations.length === 0) {
      throw new Error('pull loop cannot progress')
    }
  }

  const syncedAt = new Date().toISOString()
  const syncState = await fetchSyncState(endpoint, token)
  const serverLatestCursor = syncState.data?.latest_cursor ?? currentCursor
  const serverTime = syncState.data?.server_time ?? syncedAt

  await savePersistedSyncMetadata({
    lastAppliedCursor: currentCursor,
    lastObservedRemoteCursor: serverLatestCursor,
    lastObservedServerTime: serverTime,
    lastSuccessfulSyncAt: syncedAt,
  })

  return {
    pushedAccepted,
    pushedRejected,
    pulledApplied,
    pulledPages,
    conflictsLogged,
    finalCursor: currentCursor,
    serverLatestCursor,
    serverTime,
  }
}
