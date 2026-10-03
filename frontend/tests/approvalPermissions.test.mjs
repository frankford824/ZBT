import assert from 'node:assert/strict'
import test from 'node:test'
import { canReviewApproval } from '../src/features/team/approvalPermissions.ts'

test('assigned project manager can review without team admin permission', () => {
  const approval = { status: 'pending', current_step: 1, snapshot: [{ user_id: 'reviewer', role_code: 'project_manager' }] }
  assert.equal(canReviewApproval(approval, 'reviewer', ['project_manager']), true)
  assert.equal(canReviewApproval(approval, 'submitter', ['company_admin']), false)
  assert.equal(canReviewApproval({ ...approval, status: 'approved' }, 'reviewer', ['project_manager']), false)
})

test('role-based decisions use the current step, not other roles or steps', () => {
  const approval = { status: 'pending', current_step: 2, snapshot: [{ role_code: 'department_admin' }, { role_code: 'project_manager' }] }
  assert.equal(canReviewApproval(approval, 'user', ['project_manager']), true)
  assert.equal(canReviewApproval(approval, 'user', ['department_admin']), false)
  assert.equal(canReviewApproval({ ...approval, current_step: 3 }, 'user', ['project_manager']), false)
})
