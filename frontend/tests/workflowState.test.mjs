import test from 'node:test'
import assert from 'node:assert/strict'
import { canGenerateOutline, parseConfirmationPayload } from '../src/features/bid/workflowState.ts'

test('outline requires persisted confirmation and cannot race a pending save', () => {
  assert.equal(canGenerateOutline(undefined, false), false)
  assert.equal(canGenerateOutline({ status: 'ready' }, false), false)
  assert.equal(canGenerateOutline({ status: 'confirmed' }, false), false)
  const saved = { status: 'confirmed', confirmed_at: '2026-10-03T14:00:00Z' }
  assert.equal(canGenerateOutline(saved, true), false)
  assert.equal(canGenerateOutline(saved, false), true)
})

test('unchanged confirmation is small; edits and optimistic version are retained', () => {
  const timestamp = '2026-10-03T14:00:00Z'
  assert.deepEqual(parseConfirmationPayload(timestamp, false, () => { throw Error('must not rebuild') }), { expected_updated_at: timestamp })
  assert.deepEqual(parseConfirmationPayload(timestamp, true, () => ({ project_name: '人工校正' })), {
    expected_updated_at: timestamp, structured_result: { project_name: '人工校正' },
  })
})
