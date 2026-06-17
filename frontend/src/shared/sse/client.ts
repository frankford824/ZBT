import { isStaleSessionRefreshError, refreshSession } from '../api/client'
import { expireSessionAndRedirect, getStoredSession, shouldRefreshSession } from '../auth/session'

export type SseHandler<T> = {
  onMessage: (data: T, event: string) => void
  onOpen?: () => void
  onError?: (error: Error) => void
}

const sseApiBaseURL = import.meta.env.VITE_API_BASE_URL || '/api/v1'

export function openSse<T>(path: string, handler: SseHandler<T>) {
  const controller = new AbortController()
  void readSse(path, handler, controller.signal)
  return () => controller.abort()
}

async function readSse<T>(path: string, handler: SseHandler<T>, signal: AbortSignal) {
  let reader: ReadableStreamDefaultReader<Uint8Array> | null = null
  try {
    const response = await fetch(sseUrl(path), {
      headers: await sseHeaders(),
      signal,
    })
    if (response.status === 401) {
      expireSessionAndRedirect()
      throw new Error('登录状态已过期，请重新登录')
    }
    if (!response.ok || !response.body) {
      throw new Error('实时更新暂时不可用，请稍后重试')
    }
    handler.onOpen?.()
    reader = response.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    while (!signal.aborted) {
      const { value, done } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const parts = buffer.split(/\n\n|\r\n\r\n/)
      buffer = parts.pop() ?? ''
      for (const part of parts) {
        const parsed = parseSse(part)
        if (!parsed) continue
        if (parsed.event === 'error') {
          handler.onError?.(ssePayloadError(parsed.data))
          return
        }
        handler.onMessage(JSON.parse(parsed.data) as T, parsed.event)
      }
    }
  } catch (error) {
    if (!signal.aborted) {
      handler.onError?.(toUserFacingSseError(error))
    }
  } finally {
    reader?.releaseLock()
  }
}

function toUserFacingSseError(error: unknown) {
  if (error instanceof Error && /[\u4e00-\u9fff]/.test(error.message)) {
    return error
  }
  return new Error('实时更新暂时不可用，请稍后重试')
}

function ssePayloadError(data: string) {
  try {
    const parsed = JSON.parse(data) as { error?: unknown; message?: unknown }
    if (typeof parsed.message === 'string' && /[\u4e00-\u9fff]/.test(parsed.message)) {
      return new Error(parsed.message)
    }
    if (typeof parsed.error === 'string' && /[\u4e00-\u9fff]/.test(parsed.error)) {
      return new Error(parsed.error)
    }
  } catch {
    return new Error('实时更新暂时不可用，请稍后重试')
  }
  return new Error('实时更新暂时不可用，请稍后重试')
}

function parseSse(raw: string) {
  let event = 'message'
  const data: string[] = []
  for (const line of raw.split(/\r?\n/)) {
    if (line.startsWith(':')) continue
    if (line.startsWith('event:')) event = line.slice(6).trim()
    if (line.startsWith('data:')) data.push(line.slice(5).trimStart())
  }
  if (!data.length) return null
  return { event, data: data.join('\n') }
}

function sseUrl(path: string) {
  const trimmed = path.trim()
  if (/^https?:\/\//i.test(trimmed)) {
    return allowedAbsoluteSseURL(trimmed)
  }
  if (!isSafeRelativeSsePath(trimmed)) {
    throw new Error('实时更新暂时不可用，请稍后重试')
  }
  return joinBaseAndPath(sseApiBaseURL, trimmed)
}

async function sseHeaders() {
  const headers = new Headers({ Accept: 'text/event-stream' })
  let session = getStoredSession()
  if (session && shouldRefreshSession(session)) {
    try {
      session = await refreshSession()
    } catch (error) {
      if (!isStaleSessionRefreshError(error)) {
        expireSessionAndRedirect()
      }
      throw new Error('登录状态已过期，请重新登录', { cause: error })
    }
  }
  if (session) {
    headers.set('Authorization', `Bearer ${session.access_token}`)
    headers.set('X-Tenant-ID', session.session.tenant.id)
  }
  return headers
}

function allowedAbsoluteSseURL(raw: string) {
  try {
    const url = new URL(raw)
    const allowedOrigins = new Set([browserOrigin(), apiOrigin()].filter(Boolean))
    if (!allowedOrigins.has(url.origin)) {
      throw new Error('disallowed origin')
    }
    return url.toString()
  } catch {
    throw new Error('实时更新暂时不可用，请稍后重试')
  }
}

function isSafeRelativeSsePath(path: string) {
  if (!path || !path.startsWith('/') || path.startsWith('//')) return false
  if (containsUnsafePathCharacter(path)) return false
  try {
    const decoded = decodeURIComponent(path)
    return !decoded.includes('\\') && !decoded.startsWith('//')
  } catch {
    return false
  }
}

function joinBaseAndPath(baseURL: string, path: string) {
  return `${baseURL.replace(/\/+$/, '')}/${path.replace(/^\/+/, '')}`
}

function apiOrigin() {
  try {
    return new URL(sseApiBaseURL, browserOrigin() || undefined).origin
  } catch {
    return ''
  }
}

function browserOrigin() {
  return typeof window === 'undefined' ? '' : window.location.origin
}

function containsUnsafePathCharacter(path: string) {
  for (let index = 0; index < path.length; index += 1) {
    const code = path.charCodeAt(index)
    if (code <= 31 || code === 127 || path[index] === '\\') return true
  }
  return false
}
