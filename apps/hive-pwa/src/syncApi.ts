export type SyncStateResponse = {
  server_time: string
  latest_cursor: string
}

export type DeviceRegisterResponse = {
  device_id: string
  registered: boolean
}

export type FetchResult<T> = {
  ok: boolean
  status: number
  data?: T
  error?: string
}

export type PushOperation = {
  op_id: string
  idempotency_key: string
  entity_type: 'note' | 'todo'
  entity_id: string
  op_type: 'upsert' | 'delete'
  payload: unknown
  client_updated_at: string
  base_version: number
}

export type PushRequest = {
  device_id: string
  device_name: string
  platform: string
  app_version: string
  operations: PushOperation[]
}

export type PushReject = {
  op_id: string
  reason: string
}

export type PushConflict = {
  entity_type: string
  entity_id: string
  winner: string
  loser: string
  reason: string
  winner_device_id: string
  loser_device_id: string
}

export type PushResponse = {
  accepted: string[]
  rejected: PushReject[]
  server_cursor: string
  conflicts: PushConflict[]
}

export type PullOperation = {
  op_id: string
  idempotency_key: string
  entity_type: 'note' | 'todo'
  entity_id: string
  op_type: 'upsert' | 'delete'
  payload: unknown
  client_updated_at: string
  base_version: number
}

export type PullResponse = {
  operations: PullOperation[]
  next_cursor: string
  has_more: boolean
}

const DEFAULT_REQUEST_TIMEOUT_MS = 15000
const PULL_REQUEST_TIMEOUT_MS = 60000

function readErrorPayload(payload: unknown): string {
  if (payload && typeof payload === 'object' && 'error' in payload) {
    const candidate = (payload as { error?: unknown }).error
    if (typeof candidate === 'string' && candidate.trim() !== '') {
      return candidate.trim()
    }
  }

  return 'request failed'
}

function authHeaders(token: string, contentType?: string): HeadersInit | undefined {
  const trimmed = token.trim()
  const headers: Record<string, string> = {}

  if (contentType) {
    headers['Content-Type'] = contentType
  }

  if (trimmed) {
    headers.Authorization = `Bearer ${trimmed}`
  }

  return Object.keys(headers).length > 0 ? headers : undefined
}

async function fetchWithTimeout(
  input: RequestInfo | URL,
  init?: RequestInit,
  timeoutMs = DEFAULT_REQUEST_TIMEOUT_MS,
): Promise<Response> {
  const controller = new AbortController()
  const timeout = window.setTimeout(() => controller.abort(), timeoutMs)

  try {
    return await fetch(input, {
      ...init,
      signal: controller.signal,
    })
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') {
      throw new Error(`request timed out after ${timeoutMs / 1000}s`)
    }
    throw error
  } finally {
    window.clearTimeout(timeout)
  }
}

export async function fetchSyncState(
  endpoint: string,
  token: string,
): Promise<FetchResult<SyncStateResponse>> {
  const url = `${endpoint.replace(/\/$/, '')}/v1/sync/state`
  const response = await fetchWithTimeout(url, {
    method: 'GET',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  })

  const payload = await response.json().catch(() => null)
  if (!response.ok) {
    return {
      ok: false,
      status: response.status,
      error: readErrorPayload(payload),
    }
  }

  return {
    ok: true,
    status: response.status,
    data: payload as SyncStateResponse,
  }
}

export async function registerDevice(
  endpoint: string,
  deviceID: string,
  token: string,
): Promise<FetchResult<DeviceRegisterResponse>> {
  const url = `${endpoint.replace(/\/$/, '')}/v1/devices/register`
  const response = await fetchWithTimeout(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ device_id: deviceID }),
  })

  const payload = await response.json().catch(() => null)
  if (!response.ok) {
    return {
      ok: false,
      status: response.status,
      error: readErrorPayload(payload),
    }
  }

  return {
    ok: true,
    status: response.status,
    data: payload as DeviceRegisterResponse,
  }
}

export async function pushSyncOperations(
  endpoint: string,
  token: string,
  payload: PushRequest,
): Promise<PushResponse> {
  const url = `${endpoint.replace(/\/$/, '')}/v1/sync/push`
  const response = await fetchWithTimeout(url, {
    method: 'POST',
    headers: authHeaders(token, 'application/json'),
    body: JSON.stringify(payload),
  })

  const responsePayload = await response.json().catch(() => null)
  if (!response.ok) {
    throw new Error(`push failed (${response.status}): ${readErrorPayload(responsePayload)}`)
  }

  return responsePayload as PushResponse
}

export async function pullSyncOperations(
  endpoint: string,
  token: string,
  cursor: string,
  limit = 100,
): Promise<PullResponse> {
  const url = new URL(`${endpoint.replace(/\/$/, '')}/v1/sync/pull`)
  url.searchParams.set('cursor', cursor.trim() || '0')
  url.searchParams.set('limit', String(limit))

  const response = await fetchWithTimeout(
    url.toString(),
    {
      method: 'GET',
      headers: authHeaders(token),
    },
    PULL_REQUEST_TIMEOUT_MS,
  )

  const responsePayload = await response.json().catch(() => null)
  if (!response.ok) {
    throw new Error(`pull failed (${response.status}): ${readErrorPayload(responsePayload)}`)
  }

  return responsePayload as PullResponse
}
