/**
 * JSON client for the KubeBench backend (docs/design-v0.5.md §6.2).
 *
 * Every request goes to `/api<path>`. Bodies are sent as JSON, query
 * parameters are always built with `qs()` (never by hand), and failures
 * become an `ApiError` carrying the backend's `{"error", "code", "reason",
 * "params"}` body (§3.3). Network failures surface as `ApiError` with
 * status 0.
 */
import { reasonText } from '@/core/reasons'
import type { ReasonParams } from './types'

/** Machine codes the backend uses (§3.3), plus `network` for transport failures. */
export type ApiErrorCode =
  | 'unauthenticated'
  | 'session_expired'
  | 'forbidden'
  | 'forbidden_role'
  | 'cluster_read_only'
  | 'admin_required'
  | 'no_cluster'
  | 'not_found'
  | 'bad_request'
  | 'conflict'
  | 'rate_limited'
  | 'plugin_disabled'
  | 'plugin_unavailable'
  | 'capability_unavailable'
  | 'redacted_secret'
  | 'password_change_required'
  | 'kubernetes'
  | 'network'
  | 'http_error'
  | (string & {})

/** The server's explanation of an error: a stable reason code with its parameters (§3.3). */
export interface ApiErrorReason {
  reason: string
  params?: ReasonParams
}

export class ApiError extends Error {
  /** HTTP status, or 0 when the server could not be reached. */
  readonly status: number
  readonly code: ApiErrorCode
  /** Reason code of an error KubeBench explains itself; absent for e.g. Kubernetes API errors. */
  readonly reason?: string
  readonly params?: ReasonParams

  /** `message` is the server's English text, the fallback for reasons the UI cannot translate. */
  constructor(status: number, code: ApiErrorCode, message: string, explanation?: ApiErrorReason) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.reason = explanation?.reason
    this.params = explanation?.params
  }
}

export function isApiError(err: unknown, code?: ApiErrorCode): err is ApiError {
  return err instanceof ApiError && (code === undefined || err.code === code)
}

/**
 * Human-readable message for any thrown value, in the UI language when the
 * server explained the error with a reason code it knows.
 */
export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) return reasonText(err.reason, err.params, err.message)
  if (err instanceof Error) return err.message
  return String(err)
}

export type QueryValue = string | number | boolean | null | undefined
export type QueryParams = Record<string, QueryValue>

/**
 * Builds a query string (with the leading `?`) from params. Every value is
 * URL-encoded; `undefined`, `null` and empty strings are skipped.
 */
export function qs(params?: QueryParams): string {
  if (!params) return ''
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === '') continue
    search.append(key, String(value))
  }
  const query = search.toString()
  return query ? `?${query}` : ''
}

/** Absolute app URL for an API path, e.g. for EventSource or WebSocket clients. */
export function apiUrl(path: string, params?: QueryParams): string {
  return `/api${path}${qs(params)}`
}

export interface ApiRequestInit extends Omit<RequestInit, 'body'> {
  /** Serialised as JSON unless it is a string, FormData, Blob or URLSearchParams. */
  body?: unknown
  params?: QueryParams
}

/**
 * Called when the server says the session is gone (401 outside /auth) or
 * that the password must be changed first. The shell subscribes to show a
 * "sign in again" prompt instead of failing every request silently.
 */
type SessionListener = (code: ApiErrorCode) => void
const sessionListeners = new Set<SessionListener>()

export function onSessionLost(listener: SessionListener): () => void {
  sessionListeners.add(listener)
  return () => sessionListeners.delete(listener)
}

const STATUS_CODES: Record<number, ApiErrorCode> = {
  400: 'bad_request',
  401: 'unauthenticated',
  403: 'forbidden',
  404: 'not_found',
  409: 'conflict',
  429: 'rate_limited',
}

function isRawBody(body: unknown): body is BodyInit {
  return (
    typeof body === 'string' ||
    body instanceof FormData ||
    body instanceof Blob ||
    body instanceof URLSearchParams
  )
}

async function readBody(res: Response): Promise<unknown> {
  if (res.status === 204) return undefined
  const type = res.headers.get('Content-Type') ?? ''
  if (type.includes('application/json')) return res.json().catch(() => null)
  return res.text()
}

/** Message for a failed response: the JSON `error`, a short plain-text body, or the status line. */
function failureMessage(res: Response, payload: unknown): string {
  if (payload && typeof payload === 'object') {
    const { error } = payload as { error?: unknown }
    if (typeof error === 'string' && error) return error
  }
  // Plain-text errors (Go's http.Error) are useful; HTML error pages from proxies are not.
  const isPlainText = (res.headers.get('Content-Type') ?? '').includes('text/plain')
  if (isPlainText && typeof payload === 'string' && payload.trim()) return payload.trim().slice(0, 300)
  return `${res.status} ${res.statusText}`.trim()
}

export async function apiFetch<T>(path: string, init: ApiRequestInit = {}): Promise<T> {
  const { body, params, headers, ...rest } = init
  const requestHeaders = new Headers(headers)
  requestHeaders.set('Accept', 'application/json')
  let requestBody: BodyInit | undefined
  if (body !== undefined) {
    if (isRawBody(body)) {
      requestBody = body
    } else {
      requestBody = JSON.stringify(body)
      requestHeaders.set('Content-Type', 'application/json')
    }
  }

  let res: Response
  try {
    res = await fetch(apiUrl(path, params), { ...rest, headers: requestHeaders, body: requestBody })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') throw err
    throw new ApiError(0, 'network', errorMessage(err))
  }

  const payload = await readBody(res)
  if (!res.ok) throw refusal(path, res, payload)
  return payload as T
}

/** The reason code and parameters of an error body, if it has them. */
function explanationOf(payload: unknown): ApiErrorReason | undefined {
  if (!payload || typeof payload !== 'object') return undefined
  const { reason, params } = payload as { reason?: unknown; params?: unknown }
  if (typeof reason !== 'string' || !reason) return undefined
  return { reason, params: params && typeof params === 'object' ? (params as ReasonParams) : undefined }
}

/** The ApiError of a failed response; tells the shell when the session is gone. */
function refusal(path: string, res: Response, payload: unknown): ApiError {
  const bodyCode =
    payload && typeof payload === 'object' ? (payload as { code?: unknown }).code : undefined
  const code: ApiErrorCode =
    typeof bodyCode === 'string' && bodyCode ? bodyCode : (STATUS_CODES[res.status] ?? 'http_error')
  if ((res.status === 401 && !path.startsWith('/auth/')) || code === 'password_change_required') {
    sessionListeners.forEach((listener) => listener(code))
  }
  return new ApiError(res.status, code, failureMessage(res, payload), explanationOf(payload))
}

/**
 * Why the server refuses `GET /api<path>`, for clients that cannot read a
 * refused response themselves (EventSource): repeats the request and
 * returns its ApiError, like apiFetch would throw it, or null when the
 * server accepts the request now (the response is then dropped unread).
 */
export async function apiRefusal(path: string, params?: QueryParams): Promise<ApiError | null> {
  const controller = new AbortController()
  let res: Response
  try {
    res = await fetch(apiUrl(path, params), { signal: controller.signal })
  } catch (err) {
    return new ApiError(0, 'network', errorMessage(err))
  }
  if (res.ok) {
    controller.abort()
    return null
  }
  return refusal(path, res, await readBody(res))
}

export function apiGet<T>(path: string, params?: QueryParams, init?: ApiRequestInit): Promise<T> {
  return apiFetch<T>(path, { ...init, method: 'GET', params })
}

export function apiPost<T>(path: string, body?: unknown, params?: QueryParams): Promise<T> {
  return apiFetch<T>(path, { method: 'POST', body, params })
}

export function apiPut<T>(path: string, body?: unknown, params?: QueryParams): Promise<T> {
  return apiFetch<T>(path, { method: 'PUT', body, params })
}

export function apiPatch<T>(path: string, body?: unknown, params?: QueryParams): Promise<T> {
  return apiFetch<T>(path, { method: 'PATCH', body, params })
}

export function apiDelete<T>(path: string, params?: QueryParams): Promise<T> {
  return apiFetch<T>(path, { method: 'DELETE', params })
}
