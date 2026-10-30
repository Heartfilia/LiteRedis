import assert from 'node:assert/strict'
import test from 'node:test'

import { createPinia, setActivePinia } from 'pinia'

import { useWorkspaceStore } from './workspace.js'

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
