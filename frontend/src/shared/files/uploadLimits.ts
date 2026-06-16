export const MAX_UPLOAD_SIZE_BYTES = 200 * 1024 * 1024

export function formatBytes(value: number): string {
  if (value < 1024) {
    return `${value} B`
  }
  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} KB`
  }
  return `${(value / 1024 / 1024).toFixed(1)} MB`
}

export function isUploadFileTooLarge(file: File): boolean {
  return file.size > MAX_UPLOAD_SIZE_BYTES
}

export function uploadSizeLimitMessage(): string {
  return `单个文件不能超过 ${formatBytes(MAX_UPLOAD_SIZE_BYTES)}`
}
