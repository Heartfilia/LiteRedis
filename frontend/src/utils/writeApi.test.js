import { test } from 'node:test'
import assert from 'node:assert/strict'
import { copyKeysToConnection, importKeysFromFile, publishMessage, setTTL, createKey, renameKey } from '../api/wails.js'
import { uncertainWrites, acknowledgeWrite } from './writeSafety.js'

test('copy uncertainty locks destination rather than source and prevents import resubmission', async () => {
  let copies = 0
  let imports = 0
  globalThis.window = { go: { main: { App: {
    CopyKeysToConnection: async () => { copies++; return { success: false, code: 'outcome_unknown' } },
    ImportKeysFromFile: async () => { imports++; return { success: true } },
    SetTTL: async () => ({ success: true }),
  } } } }
  await copyKeysToConnection('source', 'target', ['k'], true)
  await copyKeysToConnection('source', 'target', ['k'], true)
  await importKeysFromFile('target', true)
  assert.equal(copies, 1)
  assert.equal(imports, 0)
  assert.equal((await setTTL('source', 'k', 60)).success, true)
  assert.equal((await setTTL('target', 'k', 60)).code, 'outcome_unknown')
  acknowledgeWrite(uncertainWrites.value[0].scope)
})

test('publish returns receiver count, latches response loss, and does not latch explicit rejection', async () => {
  let calls = 0
  globalThis.window = { go: { main: { App: { PublishMessage: async () => { calls++; return 2 } } } } }
  assert.equal(await publishMessage('pub', 'channel', 'payload'), 2)
  window.go.main.App.PublishMessage = async () => { calls++; throw new Error('permission_denied: NOPERM') }
  await assert.rejects(publishMessage('pub', 'channel', 'payload'), /NOPERM/)
  assert.equal(uncertainWrites.value.length, 0)
  window.go.main.App.PublishMessage = async () => { calls++; throw new Error('EOF') }
  await assert.rejects(publishMessage('pub', 'channel', 'payload'))
  await assert.rejects(publishMessage('pub', 'channel', 'payload'))
  assert.equal(calls, 3)
  acknowledgeWrite(uncertainWrites.value[0].scope)
})

test('rename checks destination uncertainty and create uses the actual trimmed key scope', async () => {
  let calls = 0
  globalThis.window = { go: { main: { App: {
    CreateKey: async () => { calls++; return { success: false, code: 'outcome_unknown' } },
    RenameKey: async () => { calls++; return { success: true } },
    SetTTL: async () => { calls++; return { success: true } },
  } } } }
  await createKey('scope', { key: ' target ' })
  await setTTL('scope', 'target', 60)
  await renameKey('scope', 'other', 'target')
  assert.equal(calls, 1)
  acknowledgeWrite(uncertainWrites.value[0].scope)
})
