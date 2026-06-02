import DOMPurify from 'dompurify'
import { marked } from 'marked'
import type { LocalSyncConflict, LocalTodo, SyncState } from './hiveStorage'

const LOCAL_OP_TIMEOUT_MS = 8000

export const formatTimestamp = (value: string | null | undefined): string => {
  if (!value) {
    return '—'
  }

  const timestamp = new Date(value)
  return Number.isNaN(timestamp.getTime()) ? value : timestamp.toLocaleString()
}

export const deriveNoteTitle = (path: string): string => {
  const lastSegment = path.split('/').pop() ?? path
  return lastSegment.replace(/\.[^.]+$/, '').trim() || 'Untitled note'
}

export const createLocalID = (): string => {
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

export const withTimeout = async <T>(promise: Promise<T>, label: string): Promise<T> => {
  return await Promise.race([
    promise,
    new Promise<T>((_, reject) => {
      globalThis.setTimeout(
        () => reject(new Error(`${label} timed out after ${LOCAL_OP_TIMEOUT_MS / 1000}s`)),
        LOCAL_OP_TIMEOUT_MS,
      )
    }),
  ])
}

export const countSyncState = (
  collection: Array<{ syncState: SyncState }>,
  target: SyncState,
): number => collection.filter((item) => item.syncState === target).length

export const describeSyncState = (state: SyncState): string => {
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

export const describeConflictReason = (reason: string): string => {
  if (reason.trim() === 'base_version_mismatch') {
    return 'Remote version changed before your push finished.'
  }

  return reason.trim() || 'Conflict detected.'
}

export const formatConflictTarget = (conflict: LocalSyncConflict): string =>
  `${conflict.entityType}:${conflict.entityID}`

export const formatNoteStats = (content: string): string => {
  const lineCount = content === '' ? 0 : content.split(/\r?\n/).length
  return `${content.length} chars · ${lineCount} lines`
}

export const renderMarkdown = (markdown: string): string => {
  const rendered = marked.parse(markdown || '_Nothing to preview yet._', {
    async: false,
    breaks: true,
    gfm: true,
  }) as string

  return DOMPurify.sanitize(rendered)
}

export const buildTodoSyncPayload = (todo: LocalTodo): string =>
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
