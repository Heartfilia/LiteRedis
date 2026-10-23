import { test } from 'node:test'
import assert from 'node:assert/strict'
import { formatBytes, formatCount } from './format.js'

test('formatBytes handles units and invalid input', () => {
  assert.equal(formatBytes(0), '0 B')
  assert.equal(formatBytes(-1), '0 B')
  assert.equal(formatBytes(512), '512 B')
  assert.equal(formatBytes(1024), '1.0 KB')
  assert.equal(formatBytes(1536), '1.5 KB')
  assert.equal(formatBytes(1048576), '1.0 MB')
  assert.equal(formatBytes(1024 * 1024 * 200), '200 MB')
})

test('formatCount compacts large numbers', () => {
  assert.equal(formatCount(0), '0')
  assert.equal(formatCount(999), '999')
  assert.equal(formatCount(1200), '1.2K')
  assert.equal(formatCount(2500000), '2.50M')
})
