import assert from 'node:assert/strict'
import { test } from 'node:test'
import { shouldReloadAfterPreloadError } from '../src/shared/preloadRecovery.ts'

test('stale chunk recovery refreshes once and prevents loops across reloads', () => {
  const data = new Map()
  const storage = { getItem: (key) => data.get(key) ?? null, setItem: (key, value) => data.set(key, value) }
  assert.equal(shouldReloadAfterPreloadError(storage, 100_000), true)
  assert.equal(shouldReloadAfterPreloadError(storage, 100_001), false)
  assert.equal(shouldReloadAfterPreloadError(storage, 160_000), true)
})

test('storage failures cannot trigger an automatic refresh loop', () => {
  const denied = { getItem: () => { throw new Error('denied') }, setItem: () => {} }
  const deniedWrite = { getItem: () => null, setItem: () => { throw new Error('denied') } }
  assert.equal(shouldReloadAfterPreloadError(denied), false)
  assert.equal(shouldReloadAfterPreloadError(deniedWrite), false)
})
