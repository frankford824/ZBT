export type SseHandler<T> = {
  onMessage: (data: T) => void
  onError?: (event: Event) => void
}

export function openSse<T>(path: string, handler: SseHandler<T>) {
  const source = new EventSource(path, { withCredentials: true })
  source.onmessage = (event) => {
    handler.onMessage(JSON.parse(event.data) as T)
  }
  source.onerror = (event) => {
    handler.onError?.(event)
  }
  return () => source.close()
}
