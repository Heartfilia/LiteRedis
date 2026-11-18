import assert from 'node:assert/strict'
import test from 'node:test'

import {
  connectionIconCompact,
  connectionIconLines,
  connectionIconText,
  iconTextLines,
  normalizeIconText,
} from './iconText.js'

test('normalizeIconText trims whitespace and caps at four characters', () => {
  assert.equal(normalizeIconText(''), '')
  assert.equal(normalizeIconText('  AB  '), 'AB')
  assert.equal(normalizeIconText('A\nB\tC'), 'ABC')
  assert.equal(normalizeIconText('ABCDE'), 'ABCD')
  assert.equal(normalizeIconText('缓存一号库'), '缓存一号')
  assert.equal(normalizeIconText('😀😀😀😀😀'), '😀😀😀😀')
})

test('iconTextLines puts two characters on each line', () => {
  assert.deepEqual(iconTextLines(''), [])
  assert.deepEqual(iconTextLines('A'), ['A'])
  assert.deepEqual(iconTextLines('AB'), ['AB'])
  assert.deepEqual(iconTextLines('ABC'), ['AB', 'C'])
  assert.deepEqual(iconTextLines('ABCD'), ['AB', 'CD'])
})

test('connection icons fall back to the first name character', () => {
  assert.equal(connectionIconText({ name: 'alpha' }), 'A')
  assert.equal(connectionIconText({ name: '缓存' }), '缓')
  assert.equal(connectionIconText({ host: 'redis.example.com' }), 'R')
  assert.equal(connectionIconText({}), '?')
  assert.deepEqual(connectionIconLines({ name: 'alpha' }), ['A'])
  assert.equal(connectionIconCompact({ name: 'alpha' }), false)
})

test('custom icon text overrides the name and can use two lines', () => {
  assert.equal(connectionIconText({ name: 'alpha', iconText: 'xy' }), 'xy')
  assert.equal(connectionIconText({ name: 'alpha', icon_text: '缓存' }), '缓存')
  assert.deepEqual(connectionIconLines({ name: 'alpha', icon_text: 'ABCD' }), ['AB', 'CD'])
  assert.equal(connectionIconCompact({ name: 'alpha', icon_text: 'AB' }), true)
  assert.equal(connectionIconCompact({ name: 'alpha', icon_text: 'A' }), false)
})

test('blank custom icon text falls back to the name initial', () => {
  assert.equal(connectionIconText({ name: 'alpha', iconText: '   ' }), 'A')
  assert.deepEqual(connectionIconLines({ name: 'alpha', icon_text: '\n\t' }), ['A'])
})