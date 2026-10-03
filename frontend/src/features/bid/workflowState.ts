export function canGenerateOutline(parse: { status: string; confirmed_at?: string | null } | undefined, pending: boolean) {
  return !pending && parse?.status === 'confirmed' && Boolean(parse.confirmed_at)
}

export function shouldRefreshGenerationChapters(jobs: readonly { status: string }[] | undefined) {
  // Final/paused snapshots can contain newly persisted chapter results too.
  return Boolean(jobs?.some(job => ['queued', 'running', 'paused', 'done', 'failed', 'cancelled'].includes(job.status)))
}

export function parseConfirmationPayload(
  updatedAt: string,
  hasEdits: boolean,
  buildStructured: () => Record<string, unknown>,
) {
  // An unchanged confirmation needs no multi-kilobyte copy of model evidence.
  return hasEdits
    ? { expected_updated_at: updatedAt, structured_result: buildStructured() }
    : { expected_updated_at: updatedAt }
}
