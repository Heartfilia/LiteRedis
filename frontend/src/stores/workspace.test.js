import assert from 'node:assert/strict'
import test from 'node:test'

import { createPinia, setActivePinia } from 'pinia'

import { useWorkspaceStore } from './workspace.js'
import { registerEditorGuard } from '../utils/editSafety.js'
import { resolveConfirmation } from '../utils/confirm.js'

function setup() {
  setActivePinia(createPinia())
  const store = useWorkspaceStore()
  store.activeConnID = 'conn-1'
  store.currentDB = 0
  return store
}

test('recordRecentKey keeps the newest key first and drops duplicates', () => {
  const store = setup()
  store.recordRecentKey('a')
  store.recordRecentKey('b')
  store.recordRecentKey('a')
  assert.deepEqual(store.recentKeyList, ['a', 'b'])
})

test('recordRecentKey ignores keys that are already favorited', () => {
  const store = setup()
  store.toggleFavorite('a')
  store.recordRecentKey('a')
  store.recordRecentKey('b')
  assert.deepEqual(store.favoriteKeys, ['a'])
  assert.deepEqual(store.recentKeyList, ['b'])
})

test('favoriting a key removes it from the recent list', () => {
  const store = setup()
  store.recordRecentKey('a')
  store.recordRecentKey('b')
  store.toggleFavorite('a')
  assert.deepEqual(store.favoriteKeys, ['a'])
  assert.deepEqual(store.recentKeyList, ['b'])
})

test('unfavoriting lets the key show up in recent again', () => {
  const store = setup()
  store.toggleFavorite('a')
  store.recordRecentKey('a')
  assert.deepEqual(store.recentKeyList, [])
  store.toggleFavorite('a')
  store.recordRecentKey('a')
  assert.deepEqual(store.favoriteKeys, [])
  assert.deepEqual(store.recentKeyList, ['a'])
})

test('favorites and recents are scoped per connection and db', () => {
  const store = setup()
  store.toggleFavorite('a')
  store.recordRecentKey('b')
  store.currentDB = 1
  assert.deepEqual(store.favoriteKeys, [])
  assert.deepEqual(store.recentKeyList, [])
})

test('closing the current key preserves the connection and invalidates pending reads', async () => {
  const store = setup()
  store.selectedKey = 'draft'
  store.keyValue = { key: 'draft', type: 'string', string_val: 'original' }
  store.keyValueLoading = true
  const token = store._beginRequest('keyValue', { key: 'draft', requireSelectedKey: true })
  assert.equal(await store.closeCurrentKey(), true)
  assert.equal(store.selectedKey, null)
  assert.equal(store.keyValue, null)
  assert.equal(store.keyValueLoading, false)
  assert.equal(store.activeConnID, 'conn-1')
  assert.equal(store._isRequestCurrent(token), false)
  assert.equal(await store.closeCurrentKey(), false)
})

test('closing the current key respects draft cancellation and saving guards', async () => {
  const store = setup()
  store.selectedKey = 'draft'
  let saving = false
  const unregister = registerEditorGuard(Symbol(), () => ({ dirty: true, saving }))
  try {
    let pending = store.closeCurrentKey()
    resolveConfirmation(false)
    assert.equal(await pending, false)
    assert.equal(store.selectedKey, 'draft')
    saving = true
    pending = store.closeCurrentKey()
    resolveConfirmation(true)
    assert.equal(await pending, false)
    assert.equal(store.selectedKey, 'draft')
  } finally { unregister(); resolveConfirmation(false) }
})
