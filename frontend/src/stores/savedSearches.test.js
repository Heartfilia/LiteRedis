import assert from 'node:assert/strict'
import test from 'node:test'
import { createPinia, setActivePinia } from 'pinia'
import { useSavedSearchesStore, SAVED_SEARCHES_STORAGE_KEY as KEY } from './savedSearches.js'

function setup(t, initial = null) {
  const previous = Object.getOwnPropertyDescriptor(globalThis, 'localStorage')
  const data = new Map(initial === null ? [] : [[KEY, initial]])
  const storage = {
    getItem: key => data.get(key) ?? null,
    setItem: (key, value) => data.set(key, value),
  }
  Object.defineProperty(globalThis, 'localStorage', { configurable: true, value: storage })
  t.after(() => {
    if (previous) Object.defineProperty(globalThis, 'localStorage', previous)
    else delete globalThis.localStorage
  })
  setActivePinia(createPinia())
  return { store: useSavedSearchesStore(), storage, data }
}

const input = (patch = {}) => ({ name: 'Users', connID: 'conn-A', db: 0, pattern: 'user:*', ...patch })
const record = (patch = {}) => ({ ...input(), id: 'saved-1', lastUsed: 0, ...patch })

test('CRUD persists only whitelisted fields, preserves exact glob and isolates connection/database', t => {
  const { store, data } = setup(t)
  const added = store.add(input({ name: ' Users ', pattern: ' user:* ', password: 'SECRET', ssh: { password: 'SECRET' } }))
  assert.equal(added.success, true)
  assert.equal(added.item.name, 'Users')
  assert.equal(added.item.pattern, ' user:* ')
  assert.equal(added.item.lastUsed, 0)
  store.add(input({ connID: 'conn-B' }))
  store.add(input({ db: 1 }))
  assert.equal(store.forConnection('conn-A').length, 2)
  assert.equal(store.forConnection('conn-A', 0).length, 1)
  assert.equal(store.forConnection('missing').length, 0)
  const updated = store.update(added.item.id, { name: 'Renamed', pattern: '*', id: 'spoofed', lastUsed: 10, password: 'SECRET' })
  assert.equal(updated.success, true)
  assert.equal(updated.item.id, added.item.id)
  assert.equal(updated.item.lastUsed, 0)
  assert.equal(store.touch(added.item.id).success, true)
  assert.ok(store.items[0].lastUsed > 0)
  assert.equal(data.get(KEY).includes('SECRET'), false)
  assert.deepEqual(Object.keys(JSON.parse(data.get(KEY))[0]).sort(), ['connID', 'db', 'id', 'lastUsed', 'name', 'pattern'])
  setActivePinia(createPinia())
  const reloaded = useSavedSearchesStore()
  assert.equal(reloaded.load().success, true)
  assert.deepEqual(reloaded.items, store.items)
  assert.equal(reloaded.remove(added.item.id).success, true)
  assert.equal(reloaded.items.length, 2)
  for (const action of [() => reloaded.remove('missing'), () => reloaded.touch('missing'), () => reloaded.update('missing', {})]) {
    assert.equal(action().code, 'not_found')
  }
})

test('validates required strings, Unicode limits and nonnegative integer database without coercion', t => {
  const { store } = setup(t)
  for (const patch of [
    { name: '' }, { name: ' ' }, { name: 'x'.repeat(81) }, { connID: '' },
    { db: -1 }, { db: 1.5 }, { db: '0' }, { db: NaN }, { db: Infinity }, { db: Number.MAX_SAFE_INTEGER + 1 },
    { pattern: '' }, { pattern: null }, { pattern: 'x'.repeat(1025) },
  ]) assert.equal(store.add(input(patch)).code, 'invalid_input', JSON.stringify(patch))
  assert.equal(store.items.length, 0)
  const valid = store.add(input({ name: '😀'.repeat(80), pattern: '😀'.repeat(1024), db: 15 }))
  assert.equal(valid.success, true)
  assert.equal(store.update(valid.item.id, { db: -1 }).code, 'invalid_input')
  assert.equal(store.items[0].db, 15)
})

test('load filters invalid values, duplicate ids, extra fields and excess records', t => {
  const values = [null, [], {}, record({ db: '0' }), record({ id: 'negative', lastUsed: -1 }),
    record({ id: 'bad-pattern', pattern: 'x'.repeat(1025) }),
    record({ password: 'SECRET' }), record({ name: 'duplicate' }),
    ...Array.from({ length: 110 }, (_, index) => record({ id: `id-${index}` })),
  ]
  const { store } = setup(t, JSON.stringify(values))
  assert.equal(store.load().success, true)
  assert.equal(store.items.length, 100)
  assert.equal(store.items[0].name, 'Users')
  assert.equal('password' in store.items[0], false)
  assert.equal(store.add(input()).code, 'limit_reached')
})

test('malformed JSON and invalid root are reported and never rewritten during load', t => {
  const { store, data } = setup(t)
  for (const raw of ['{broken', '{}', 'null', '42']) {
    data.set(KEY, raw)
    assert.equal(store.load().code, 'invalid_storage')
    assert.deepEqual(store.items, [])
    assert.equal(data.get(KEY), raw)
  }
})

test('read failures block writes and preserve existing in-memory state', t => {
  const { store, storage, data } = setup(t, JSON.stringify([record()]))
  storage.getItem = () => { throw new Error('Read denied') }
  assert.equal(store.add(input()).code, 'storage_read_failed')
  assert.equal(store.loaded, false)
  assert.match(store.error, /Read denied/)
  assert.equal(data.get(KEY), JSON.stringify([record()]))
})

test('persistence failures never report success or change memory/disk for any mutation', t => {
  const { store, storage, data } = setup(t, JSON.stringify([record()]))
  assert.equal(store.load().success, true)
  const before = JSON.stringify(store.items)
  const disk = data.get(KEY)
  storage.setItem = () => { throw new Error('Quota exceeded') }
  for (const action of [() => store.add(input()), () => store.update('saved-1', { name: 'Changed' }),
    () => store.touch('saved-1'), () => store.remove('saved-1')]) {
    const result = action()
    assert.equal(result.success, false)
    assert.equal(result.code, 'storage_write_failed')
    assert.match(result.message, /Quota exceeded/)
    assert.equal(JSON.stringify(store.items), before)
    assert.equal(data.get(KEY), disk)
  }
})

test('automatic initial load preserves existing records and does not access other storage keys', t => {
  const { store, storage, data } = setup(t, JSON.stringify([record()]))
  const getItem = storage.getItem
  const setItem = storage.setItem
  storage.getItem = key => { assert.equal(key, KEY); return getItem(key) }
  storage.setItem = (key, value) => { assert.equal(key, KEY); setItem(key, value) }
  data.set('other', 'untouched')
  assert.equal(store.add(input()).success, true)
  assert.equal(store.items.length, 2)
  assert.equal(data.get('other'), 'untouched')
})
