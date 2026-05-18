const DB_NAME = 'hive-pwa'
const DB_VERSION = 3

const CONFIG_STORE = 'config'
const SYNC_METADATA_STORE = 'syncMetadata'
const DRAFT_QUEUE_STORE = 'draftQueue'
const CACHED_NOTE_METADATA_STORE = 'cachedNoteMetadata'
const NOTES_STORE = 'notes'
const TODOS_STORE = 'todos'
const ENTITY_VERSIONS_STORE = 'entityVersions'
const SYNC_CONFLICTS_STORE = 'syncConflicts'

const CONFIG_KEY = 'settings'
const SYNC_METADATA_KEY = 'state'

export type EntityType = 'note' | 'todo'
export type DraftOperationType = 'upsert' | 'delete'
export type TodoSection = 'Inbox' | 'Next' | 'Waiting'
export type SyncOrigin = 'local' | 'remote'
export type SyncState = 'local-only' | 'queued' | 'synced' | 'conflict'
export type LocalSyncConflictStatus = 'open' | 'reviewed'

export type PersistedConfig = {
  key: typeof CONFIG_KEY
  endpoint: string
  deviceID: string
  token: string
  updatedAt: string
}

export type PersistedSyncMetadata = {
  key: typeof SYNC_METADATA_KEY
  lastAppliedCursor: string | null
  lastObservedRemoteCursor: string | null
  lastObservedServerTime: string | null
  lastRemoteCheckAt: string | null
  lastRemoteCheckKind: 'idle' | 'ok' | 'error'
  lastRemoteCheckMessage: string | null
  lastSuccessfulSyncAt: string | null
  updatedAt: string
}

export type DraftQueueItem = {
  id: string
  idempotencyKey: string
  entityType: EntityType
  entityID: string
  opType: DraftOperationType
  payload: string
  baseVersion: number
  status: 'pending'
  attemptCount: number
  lastError: string | null
  clientUpdatedAt: string
  createdAt: string
  updatedAt: string
}

export type CachedNoteMetadata = {
  noteID: string
  path: string
  updatedAt: string
}

export type LocalNote = {
  path: string
  title: string
  content: string
  tags: string[]
  domains: string[]
  syncOrigin: SyncOrigin
  syncState: SyncState
  lastSyncedAt: string | null
  updatedAt: string
}

export type LocalTodo = {
  id: string
  sourceID: string
  sourcePath: string
  taskScope: string
  taskArea: string
  todoSection: TodoSection
  text: string
  done: boolean
  meta: string
  order: number
  line: number | null
  syncOrigin: SyncOrigin
  syncState: SyncState
  lastSyncedAt: string | null
  updatedAt: string
}

export type LocalSyncConflict = {
  id: string
  entityType: EntityType
  entityID: string
  reason: string
  winnerDeviceID: string
  loserDeviceID: string
  status: LocalSyncConflictStatus
  createdAt: string
  reviewedAt: string | null
}

type EntityVersionRecord = {
  id: string
  entityType: EntityType
  entityID: string
  version: number
  updatedAt: string
}

let dbPromise: Promise<IDBDatabase> | null = null

function requestToPromise<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error ?? new Error('IndexedDB request failed'))
  })
}

function transactionToPromise(transaction: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    transaction.oncomplete = () => resolve()
    transaction.onerror = () => reject(transaction.error ?? new Error('IndexedDB transaction failed'))
    transaction.onabort = () => reject(transaction.error ?? new Error('IndexedDB transaction aborted'))
  })
}

function createStoreIfMissing(
  db: IDBDatabase,
  storeName: string,
  options?: IDBObjectStoreParameters,
): void {
  if (db.objectStoreNames.contains(storeName)) {
    return
  }

  db.createObjectStore(storeName, options)
}

function makeEntityVersionID(entityType: EntityType, entityID: string): string {
  return `${entityType}:${entityID}`
}

function normalizeTodoSection(value: string): TodoSection {
  switch (value.trim().toLowerCase()) {
    case 'next':
      return 'Next'
    case 'waiting':
      return 'Waiting'
    default:
      return 'Inbox'
  }
}

function sortByUpdatedAtDescending<T extends { updatedAt: string }>(items: T[]): T[] {
  return [...items].sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))
}

function sortByCreatedAtAscending<T extends { createdAt: string }>(items: T[]): T[] {
  return [...items].sort((left, right) => left.createdAt.localeCompare(right.createdAt))
}

function normalizeSyncOrigin(value: string | undefined): SyncOrigin {
  return value === 'remote' ? 'remote' : 'local'
}

function normalizeSyncState(value: string | undefined, lastSyncedAt: string | null): SyncState {
  switch (value) {
    case 'queued':
    case 'synced':
    case 'conflict':
    case 'local-only':
      return value
    default:
      return lastSyncedAt ? 'synced' : 'local-only'
  }
}

function normalizeLocalNote(note: LocalNote): LocalNote {
  const lastSyncedAt = note.lastSyncedAt ?? null
  return {
    path: note.path.trim(),
    title: note.title.trim(),
    content: note.content,
    tags: note.tags,
    domains: note.domains,
    syncOrigin: normalizeSyncOrigin(note.syncOrigin),
    syncState: normalizeSyncState(note.syncState, lastSyncedAt),
    lastSyncedAt,
    updatedAt: note.updatedAt,
  }
}

function normalizeLocalTodo(todo: LocalTodo): LocalTodo {
  const lastSyncedAt = todo.lastSyncedAt ?? null
  return {
    id: todo.id.trim(),
    sourceID: todo.sourceID.trim(),
    sourcePath: todo.sourcePath.trim(),
    taskScope: todo.taskScope.trim(),
    taskArea: todo.taskArea.trim(),
    todoSection: normalizeTodoSection(todo.todoSection),
    text: todo.text.trim(),
    done: todo.done,
    meta: todo.meta.trim(),
    order: todo.order,
    line: todo.line,
    syncOrigin: normalizeSyncOrigin(todo.syncOrigin),
    syncState: normalizeSyncState(todo.syncState, lastSyncedAt),
    lastSyncedAt,
    updatedAt: todo.updatedAt,
  }
}

function normalizeLocalSyncConflict(conflict: LocalSyncConflict): LocalSyncConflict {
  return {
    id: conflict.id.trim(),
    entityType: conflict.entityType,
    entityID: conflict.entityID.trim(),
    reason: conflict.reason.trim(),
    winnerDeviceID: conflict.winnerDeviceID.trim(),
    loserDeviceID: conflict.loserDeviceID.trim(),
    status: conflict.status === 'reviewed' ? 'reviewed' : 'open',
    createdAt: conflict.createdAt,
    reviewedAt: conflict.reviewedAt ?? null,
  }
}

function openHiveDB(): Promise<IDBDatabase> {
  if (dbPromise) {
    return dbPromise
  }

  dbPromise = new Promise((resolve, reject) => {
    if (typeof window === 'undefined' || !('indexedDB' in window)) {
      reject(new Error('IndexedDB is unavailable in this browser'))
      return
    }

    const request = window.indexedDB.open(DB_NAME, DB_VERSION)

    request.onupgradeneeded = () => {
      const db = request.result

      createStoreIfMissing(db, CONFIG_STORE, { keyPath: 'key' })
      createStoreIfMissing(db, SYNC_METADATA_STORE, { keyPath: 'key' })

      if (!db.objectStoreNames.contains(DRAFT_QUEUE_STORE)) {
        const store = db.createObjectStore(DRAFT_QUEUE_STORE, { keyPath: 'id' })
        store.createIndex('by_created_at', 'createdAt', { unique: false })
      }

      if (!db.objectStoreNames.contains(CACHED_NOTE_METADATA_STORE)) {
        const store = db.createObjectStore(CACHED_NOTE_METADATA_STORE, { keyPath: 'noteID' })
        store.createIndex('by_updated_at', 'updatedAt', { unique: false })
      }

      if (!db.objectStoreNames.contains(NOTES_STORE)) {
        const store = db.createObjectStore(NOTES_STORE, { keyPath: 'path' })
        store.createIndex('by_updated_at', 'updatedAt', { unique: false })
      }

      if (!db.objectStoreNames.contains(TODOS_STORE)) {
        const store = db.createObjectStore(TODOS_STORE, { keyPath: 'id' })
        store.createIndex('by_updated_at', 'updatedAt', { unique: false })
      }

      if (!db.objectStoreNames.contains(ENTITY_VERSIONS_STORE)) {
        const store = db.createObjectStore(ENTITY_VERSIONS_STORE, { keyPath: 'id' })
        store.createIndex('by_entity', ['entityType', 'entityID'], { unique: true })
      }

      if (!db.objectStoreNames.contains(SYNC_CONFLICTS_STORE)) {
        const store = db.createObjectStore(SYNC_CONFLICTS_STORE, { keyPath: 'id' })
        store.createIndex('by_created_at', 'createdAt', { unique: false })
        store.createIndex('by_status', 'status', { unique: false })
      }
    }

    request.onsuccess = () => resolve(request.result)
    request.onerror = () => {
      dbPromise = null
      reject(request.error ?? new Error('Failed to open Hive IndexedDB database'))
    }
  })

  return dbPromise
}

async function getRecord<T>(storeName: string, key: IDBValidKey): Promise<T | undefined> {
  const db = await openHiveDB()
  const transaction = db.transaction(storeName, 'readonly')
  const done = transactionToPromise(transaction)
  const request = transaction.objectStore(storeName).get(key) as IDBRequest<T | undefined>
  const result = await requestToPromise(request)
  await done
  return result
}

async function getAllRecords<T>(storeName: string): Promise<T[]> {
  const db = await openHiveDB()
  const transaction = db.transaction(storeName, 'readonly')
  const done = transactionToPromise(transaction)
  const request = transaction.objectStore(storeName).getAll() as IDBRequest<T[]>
  const result = await requestToPromise(request)
  await done
  return result
}

async function putRecord<T>(storeName: string, value: T): Promise<void> {
  const db = await openHiveDB()
  const transaction = db.transaction(storeName, 'readwrite')
  const done = transactionToPromise(transaction)
  transaction.objectStore(storeName).put(value)
  await done
}

async function deleteRecord(storeName: string, key: IDBValidKey): Promise<void> {
  const db = await openHiveDB()
  const transaction = db.transaction(storeName, 'readwrite')
  const done = transactionToPromise(transaction)
  transaction.objectStore(storeName).delete(key)
  await done
}

export async function initializeHiveDB(): Promise<void> {
  await openHiveDB()
}

export async function loadPersistedConfig(): Promise<PersistedConfig | undefined> {
  return getRecord<PersistedConfig>(CONFIG_STORE, CONFIG_KEY)
}

export async function savePersistedConfig(
  input: Omit<PersistedConfig, 'key' | 'updatedAt'>,
): Promise<PersistedConfig> {
  const record: PersistedConfig = {
    key: CONFIG_KEY,
    endpoint: input.endpoint,
    deviceID: input.deviceID,
    token: input.token,
    updatedAt: new Date().toISOString(),
  }

  await putRecord(CONFIG_STORE, record)
  return record
}

export async function loadPersistedSyncMetadata(): Promise<PersistedSyncMetadata | undefined> {
  return getRecord<PersistedSyncMetadata>(SYNC_METADATA_STORE, SYNC_METADATA_KEY)
}

export async function savePersistedSyncMetadata(
  input: Partial<
    Pick<
      PersistedSyncMetadata,
      | 'lastAppliedCursor'
      | 'lastObservedRemoteCursor'
      | 'lastObservedServerTime'
      | 'lastRemoteCheckAt'
      | 'lastRemoteCheckKind'
      | 'lastRemoteCheckMessage'
      | 'lastSuccessfulSyncAt'
    >
  >,
): Promise<PersistedSyncMetadata> {
  const existing = await loadPersistedSyncMetadata()
  const record: PersistedSyncMetadata = {
    key: SYNC_METADATA_KEY,
    lastAppliedCursor: input.lastAppliedCursor ?? existing?.lastAppliedCursor ?? null,
    lastObservedRemoteCursor:
      input.lastObservedRemoteCursor ?? existing?.lastObservedRemoteCursor ?? null,
    lastObservedServerTime:
      input.lastObservedServerTime ?? existing?.lastObservedServerTime ?? null,
    lastRemoteCheckAt: input.lastRemoteCheckAt ?? existing?.lastRemoteCheckAt ?? null,
    lastRemoteCheckKind: input.lastRemoteCheckKind ?? existing?.lastRemoteCheckKind ?? 'idle',
    lastRemoteCheckMessage:
      input.lastRemoteCheckMessage ?? existing?.lastRemoteCheckMessage ?? null,
    lastSuccessfulSyncAt: input.lastSuccessfulSyncAt ?? existing?.lastSuccessfulSyncAt ?? null,
    updatedAt: new Date().toISOString(),
  }

  await putRecord(SYNC_METADATA_STORE, record)
  return record
}

export async function listLocalNotes(): Promise<LocalNote[]> {
  const notes = await getAllRecords<LocalNote>(NOTES_STORE)
  return sortByUpdatedAtDescending(notes.map(normalizeLocalNote))
}

export async function getLocalNote(path: string): Promise<LocalNote | undefined> {
  const note = await getRecord<LocalNote>(NOTES_STORE, path.trim())
  return note ? normalizeLocalNote(note) : undefined
}

export async function upsertLocalNote(note: LocalNote): Promise<LocalNote> {
  const record = normalizeLocalNote(note)

  await putRecord(NOTES_STORE, record)
  return record
}

export async function deleteLocalNote(path: string): Promise<void> {
  await deleteRecord(NOTES_STORE, path.trim())
}

export async function listLocalTodos(): Promise<LocalTodo[]> {
  const todos = await getAllRecords<LocalTodo>(TODOS_STORE)
  return sortByUpdatedAtDescending(todos.map(normalizeLocalTodo))
}

export async function getLocalTodo(id: string): Promise<LocalTodo | undefined> {
  const todo = await getRecord<LocalTodo>(TODOS_STORE, id.trim())
  return todo ? normalizeLocalTodo(todo) : undefined
}

export async function upsertLocalTodo(todo: LocalTodo): Promise<LocalTodo> {
  const record = normalizeLocalTodo(todo)

  await putRecord(TODOS_STORE, record)
  return record
}

export async function deleteLocalTodo(id: string): Promise<void> {
  await deleteRecord(TODOS_STORE, id.trim())
}

export async function listDraftQueueItems(): Promise<DraftQueueItem[]> {
  const items = await getAllRecords<DraftQueueItem>(DRAFT_QUEUE_STORE)
  return sortByCreatedAtAscending(items)
}

export async function enqueueDraftQueueOperation(
  input: Omit<DraftQueueItem, 'status' | 'attemptCount' | 'lastError' | 'createdAt' | 'updatedAt'>,
): Promise<DraftQueueItem> {
  const now = new Date().toISOString()
  const item: DraftQueueItem = {
    ...input,
    id: input.id.trim(),
    idempotencyKey: input.idempotencyKey.trim(),
    entityID: input.entityID.trim(),
    payload: input.payload,
    status: 'pending',
    attemptCount: 0,
    lastError: null,
    createdAt: now,
    updatedAt: now,
  }

  await putRecord(DRAFT_QUEUE_STORE, item)
  return item
}

export async function markDraftQueueAttemptFailed(id: string, failureReason: string): Promise<void> {
  const existing = await getRecord<DraftQueueItem>(DRAFT_QUEUE_STORE, id.trim())
  if (!existing) {
    return
  }

  await putRecord(DRAFT_QUEUE_STORE, {
    ...existing,
    attemptCount: existing.attemptCount + 1,
    lastError: failureReason.trim() || 'sync request failed',
    updatedAt: new Date().toISOString(),
  })
}

export async function removeDraftQueueItem(id: string): Promise<void> {
  await deleteRecord(DRAFT_QUEUE_STORE, id.trim())
}

export async function countDraftQueueItems(): Promise<number> {
  const items = await listDraftQueueItems()
  return items.length
}

export async function getEntityVersion(entityType: EntityType, entityID: string): Promise<number> {
  const record = await getRecord<EntityVersionRecord>(
    ENTITY_VERSIONS_STORE,
    makeEntityVersionID(entityType, entityID.trim()),
  )

  return record?.version ?? 0
}

export async function incrementEntityVersion(entityType: EntityType, entityID: string): Promise<number> {
  const id = makeEntityVersionID(entityType, entityID.trim())
  const current = await getRecord<EntityVersionRecord>(ENTITY_VERSIONS_STORE, id)
  const version = (current?.version ?? 0) + 1

  await putRecord<EntityVersionRecord>(ENTITY_VERSIONS_STORE, {
    id,
    entityType,
    entityID: entityID.trim(),
    version,
    updatedAt: new Date().toISOString(),
  })

  return version
}

export async function listLocalSyncConflicts(): Promise<LocalSyncConflict[]> {
  const conflicts = await getAllRecords<LocalSyncConflict>(SYNC_CONFLICTS_STORE)
  return [...conflicts.map(normalizeLocalSyncConflict)].sort((left, right) =>
    right.createdAt.localeCompare(left.createdAt),
  )
}

export async function upsertLocalSyncConflict(conflict: LocalSyncConflict): Promise<LocalSyncConflict> {
  const record = normalizeLocalSyncConflict(conflict)
  await putRecord(SYNC_CONFLICTS_STORE, record)
  return record
}

export async function markLocalSyncConflictReviewed(id: string): Promise<void> {
  const existing = await getRecord<LocalSyncConflict>(SYNC_CONFLICTS_STORE, id.trim())
  if (!existing) {
    return
  }

  await putRecord(SYNC_CONFLICTS_STORE, {
    ...normalizeLocalSyncConflict(existing),
    status: 'reviewed',
    reviewedAt: new Date().toISOString(),
  })
}

export async function countOpenLocalSyncConflicts(): Promise<number> {
  const conflicts = await listLocalSyncConflicts()
  return conflicts.filter((conflict) => conflict.status === 'open').length
}
