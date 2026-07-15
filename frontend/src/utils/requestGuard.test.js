import assert from 'node:assert/strict'
import test from 'node:test'

import { createRequestGuard } from './requestGuard.js'

test('rejects a response after its context changes', () => {
  let context = { connID: 'a', db: 0, key: 'one' }
  const guard = createRequestGuard(() => context)
  const token = guard.begin('load')

  context = { connID: 'b', db: 0, key: 'one' }

  assert.equal(guard.isCurrent(token), false)
})

test('only accepts the latest request in the same scope', () => {
  const context = { connID: 'a', db: 0, key: 'one' }
  const guard = createRequestGuard(() => context)
  const first = guard.begin('search')
  const second = guard.begin('search')

  assert.equal(guard.isCurrent(first), false)
  assert.equal(guard.isCurrent(second), true)
})

test('keeps independent scopes valid until explicitly invalidated', () => {
  const context = { connID: 'a', db: 0, key: 'one' }
  const guard = createRequestGuard(() => context)
  const load = guard.begin('load')
  const write = guard.begin('write')

  assert.equal(guard.isCurrent(load), true)
  assert.equal(guard.isCurrent(write), true)

  guard.invalidateAll()
  assert.equal(guard.isCurrent(load), false)
  assert.equal(guard.isCurrent(write), false)
})
