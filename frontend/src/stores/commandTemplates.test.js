import assert from 'node:assert/strict'
import test from 'node:test'
import { createPinia, setActivePinia } from 'pinia'
import { COMMAND_TEMPLATES_STORAGE_KEY, useCommandTemplatesStore } from './commandTemplates.js'

function setup(raw = null) {
  const data = new Map(raw === null ? [] : [[COMMAND_TEMPLATES_STORAGE_KEY, raw]])
  globalThis.localStorage = { getItem: key => data.get(key) ?? null, setItem: (key, value) => data.set(key, value) }
  setActivePinia(createPinia())
  return { store: useCommandTemplatesStore(), data }
}

test('templates save/load/update/remove and persist only metadata, never parameters', () => {
  const { store, data } = setup()
  const result = store.save({ name: '  Read key  ', template: 'GET {{key}}', values: { key: 'secret' }, password: 'secret' })
  assert.equal(result.success, true)
  assert.equal(store.items[0].name, 'Read key')
  const persisted = JSON.parse(data.get(COMMAND_TEMPLATES_STORAGE_KEY))
  assert.deepEqual(Object.keys(persisted[0]).sort(), ['id', 'name', 'template'])
  assert.equal(data.get(COMMAND_TEMPLATES_STORAGE_KEY).includes('secret'), false)
  const id = result.item.id
  assert.equal(store.save({ id, name: 'Updated', template: 'TYPE {{key}}' }).success, true)
  setActivePinia(createPinia())
  const loaded = useCommandTemplatesStore()
  assert.equal(loaded.load().success, true)
  assert.equal(loaded.items[0].name, 'Updated')
  assert.equal(loaded.remove(id).success, true)
  assert.deepEqual(JSON.parse(data.get(COMMAND_TEMPLATES_STORAGE_KEY)), [])
})

test('templates validate name/body and enforce maximum count', () => {
  const { store } = setup()
  for (const input of [{ name: '', template: 'PING' }, { name: 'x'.repeat(81), template: 'PING' }, { name: 'x', template: 'GET prefix{{key}}' }, { name: 'x', template: 'x'.repeat(4097) }]) {
    assert.equal(store.save(input).success, false)
  }
  for (let index = 0; index < 50; index++) assert.equal(store.save({ name: `T${index}`, template: 'PING' }).success, true)
  assert.equal(store.save({ name: 'overflow', template: 'PING' }).code, 'limit_reached')
  assert.equal(store.items.length, 50)
})

test('storage exceptions never report success or mutate saved list', () => {
  const { store } = setup()
  assert.equal(store.save({ name: 'Read', template: 'GET {{key}}' }).success, true)
  const before = JSON.stringify(store.items)
  globalThis.localStorage.setItem = () => { throw new Error('Quota exceeded') }
  assert.equal(store.save({ name: 'Write', template: 'SET {{key}} {{value}}' }).code, 'storage_write_failed')
  assert.equal(JSON.stringify(store.items), before)
  assert.equal(store.remove(store.items[0].id).success, false)
  assert.equal(JSON.stringify(store.items), before)
  const read = setup().store
  globalThis.localStorage.getItem = () => { throw new Error('Blocked') }
  assert.equal(read.save({ name: 'Read', template: 'PING' }).success, false)
  assert.equal(read.loaded, false)
})

test('malformed JSON cannot be silently overwritten and legacy unsafe entries are excluded', () => {
  const { store, data } = setup('{broken')
  assert.equal(store.load().success, false)
  assert.equal(store.save({ name: 'New', template: 'PING' }).success, false)
  assert.equal(data.get(COMMAND_TEMPLATES_STORAGE_KEY), '{broken')
  const valid = { id: 'good', name: 'Read', template: 'GET {{key}}', values: { key: 'secret' } }
  const loaded = setup(JSON.stringify([valid, valid, { id: 'bad', name: 'bad', template: 'GET partial{{key}}' }])).store
  assert.equal(loaded.load().success, true)
  assert.deepEqual(loaded.items, [{ id: 'good', name: 'Read', template: 'GET {{key}}' }])
})
