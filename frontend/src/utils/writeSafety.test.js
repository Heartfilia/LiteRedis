import { test } from 'node:test'
import assert from 'node:assert/strict'
import { guardedWrite, uncertainWrites, acknowledgeWrite } from './writeSafety.js'

test('unknown writes are not replayed, errors preserve original input and require explicit verification', async () => {
  let calls = 0
  const args = ['test-connection', 'list-key', 'draft']
  const invoke = async () => { calls++; throw new Error('response lost') }
  const first = await guardedWrite('RPush', args, invoke)
  assert.equal(first.code, 'outcome_unknown')
  assert.equal((await guardedWrite('RPush', args, invoke)).code, 'outcome_unknown')
  assert.equal(calls, 1)
  assert.equal(args[2], 'draft')
  acknowledgeWrite(uncertainWrites.value[0].scope)
  assert.equal((await guardedWrite('RPush', args, async () => { calls++; return { success: true } })).success, true)
  assert.equal(calls, 2)
})

test('same-key overlapping writes are blocked but independent keys proceed', async () => {
  let complete
  const first = guardedWrite('RPush', ['connection', 'one'], () => new Promise(resolve => { complete = resolve }))
  assert.equal((await guardedWrite('RPush', ['connection', 'one'], async () => { throw new Error('must not call') })).code, 'busy')
  assert.equal((await guardedWrite('SAdd', ['connection', 'two'], async () => ({ success: true }))).success, true)
  complete({ success: true })
  await first
})

test('Console and batch unknown outcomes protect the whole connection', async () => {
  await guardedWrite('ExecuteRedisCommand', ['console-connection', 'INCR x'], async () => ({ success: false, code: 'outcome_unknown' }), true)
  let calls = 0
  await guardedWrite('SetTTL', ['console-connection', 'other'], async () => { calls++; return { success: true } })
  assert.equal(calls, 0)
  acknowledgeWrite(uncertainWrites.value[0].scope)
})
