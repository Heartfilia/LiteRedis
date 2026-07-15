import assert from 'node:assert/strict'
import test from 'node:test'

import { normalizeLoadCount } from '../stores/settings.js'

test('load counts use the documented default and clamp large values', () => {
  assert.equal(normalizeLoadCount(undefined), 20)
  assert.equal(normalizeLoadCount(0), 20)
  assert.equal(normalizeLoadCount(-5), 20)
  assert.equal(normalizeLoadCount('42'), 42)
  assert.equal(normalizeLoadCount(20000), 10000)
})
