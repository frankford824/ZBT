import type { LoginSessionPayload } from '../../app/store/session'

export type SseHandler<T> = {
  onMessage: (data: T, event: string) => void
  onOpen?: () => void
  onError?: (error: Error) => void
}

export function openSse<T>(path: string, handler: SseHandler<T>) {
  const controller = new AbortController()
  void readSse(path, handler, controller.signal)
  return () => controller.abort()
}

async function readSse<T>(path: string, handler: SseHandler<T>, signal: AbortSignal) {
  try {
    const response = await fetch(sseUrl(path), {
      headers: sseHeaders(),
      signal,
    })
    if (!response.ok || !response.body) {
      throw new Error(`SSE request failed: ${response.status}`)
    }
    handler.onOpen?.()
    const reader = response.body.getReader()
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
        handler.onMessage(JSON.parse(parsed.data) as T, parsed.event)
      }
    }
  } catch (error) {
    if (!signal.aborted) {
      handler.onError?.(error instanceof Error ? error : new Error(String(error)))
    }
  }
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
  if (/^https?:\/\//.test(path)) return path
  const baseURL = import.meta.env.VITE_API_BASE_URL || '/api/v1'
  return `${baseURL}${path}`
}

function sseHeaders() {
  const headers = new Headers({ Accept: 'text/event-stream' })
  const raw = localStorage.getItem('zbt.session')
  if (!raw) return headers
  try {
    const session = JSON.parse(raw) as LoginSessionPayload
    headers.set('Authorization', `Bearer ${session.access_token}`)
    headers.set('X-Tenant-ID', session.session.tenant.id)
  } catch {
    localStorage.removeItem('zbt.session')
  }
  return headers
}
