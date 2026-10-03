type Approval = {
  status: string
  current_step: number
  snapshot: { user_id?: string; role_code?: string }[]
}

// Only a presentation check. The server rechecks the assigned actor on every
// decision; hiding a button is not an authorization boundary.
export function canReviewApproval(approval: Approval, userId: string, roleCodes: string[]) {
  if (approval.status !== 'pending' || !userId) return false
  const step = approval.snapshot[approval.current_step - 1]
  if (!step) return false
  if (step.user_id) return step.user_id.toLowerCase() === userId.toLowerCase()
  return Boolean(step.role_code && roleCodes.includes(step.role_code))
}
