import assert from 'node:assert/strict'
import test from 'node:test'

import { createRequestGuard } from './requestGuard.js'
import { mergeStreamEntries } from './streamPagination.js'

test('stream pages keep order and de-duplicate the boundary entry', () => {
  const current = [{ id: '5-0' }, { id: '4-0' }]
  const next = [{ id: '4-0', fields: { updated: 'yes' } }, { id: '3-0' }]

  assert.deepEqual(mergeStreamEntries(current, next), [
    { id: '5-0' },
    { id: '4-0', fields: { updated: 'yes' } },
    { id: '3-0' },
  ])
})

test('a stream page response is ignored after key context changes', () => {
  let context = { connID: 'a', db: 0, key: 'stream:one' }
  const guard = createRequestGuard(() => context)
  const request = guard.begin('load')
  let entries = [{ id: '5-0' }]

  context = { connID: 'a', db: 0, key: 'stream:two' }
  if (guard.isCurrent(request)) {
    entries = mergeStreamEntries(entries, [{ id: '4-0' }])
  }

  assert.deepEqual(entries, [{ id: '5-0' }])
})
