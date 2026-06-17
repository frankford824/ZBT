export type OpenFileUrlResult = 'opened' | 'invalid' | 'blocked'

export function openFileUrl(rawUrl: string | null | undefined): OpenFileUrlResult {
  const url = normalizeFileUrl(rawUrl)
  if (!url) {
    return 'invalid'
  }
  const opened = window.open(url.toString(), '_blank', 'noopener,noreferrer')
  if (!opened) {
    return 'blocked'
  }
  opened.opener = null
  return 'opened'
}

export function fileOpenErrorMessage(result: OpenFileUrlResult): string | null {
  if (result === 'invalid') {
    return '文件链接不可用，请重新获取'
  }
  if (result === 'blocked') {
    return '新窗口被阻止，请允许后重试'
  }
  return null
}

function normalizeFileUrl(rawUrl: string | null | undefined): URL | null {
  const value = rawUrl?.trim()
  if (!value || value.startsWith('//') || containsUnsafeUrlCharacter(value)) {
    return null
  }
  try {
    const url = new URL(value, window.location.href)
    return url.protocol === 'http:' || url.protocol === 'https:' ? url : null
  } catch {
    return null
  }
}

function containsUnsafeUrlCharacter(value: string): boolean {
  for (const char of value) {
    const code = char.charCodeAt(0)
    if (char === '\\' || code <= 31 || code === 127) {
      return true
    }
  }
  return false
}
