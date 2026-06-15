export function formatDateTime(value?: string | null, locale = 'zh-CN') {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString(locale)
}

export function formatDateOnly(value?: string | null, locale = 'zh-CN') {
  if (!value) return '-'
  if (/^\d{4}-\d{2}-\d{2}$/.test(value)) return value
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleDateString(locale)
}
