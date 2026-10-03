const recoveryKey = 'zbt.preload-recovery-at'
const retryWindowMs = 60_000

// One automatic refresh per minute, even across document reloads. If storage is
// unavailable, show the error boundary instead of risking a reload loop.
export function shouldReloadAfterPreloadError(
  storage: Pick<Storage, 'getItem' | 'setItem'>,
  now = Date.now(),
): boolean {
  try {
    const previous = Number(storage.getItem(recoveryKey))
    if (previous > 0 && now - previous < retryWindowMs) return false
    storage.setItem(recoveryKey, String(now))
    return true
  } catch {
    return false
  }
}
