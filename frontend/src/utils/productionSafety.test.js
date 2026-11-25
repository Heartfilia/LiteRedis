import test from 'node:test'
import assert from 'node:assert/strict'
import { confirmProductionOperation } from './productionSafety.js'
import { confirmation, resolveConfirmation } from './confirm.js'

test('production requires exact typed confirmation; non-production remains unchanged', async () => {
  assert.equal(await confirmProductionOperation({ environment: 'development' }, 0, 'delete'), true)
  const pending = confirmProductionOperation({ id: 'p', name: 'Orders', environment: 'production' }, 4, 'delete', 38)
  assert.equal(confirmation.value.requiredText, 'DELETE')
  assert.match(confirmation.value.message, /Orders.*DB 4/)
  resolveConfirmation(true, 'delete')
  assert.ok(confirmation.value)
  resolveConfirmation(true, 'DELETE')
  assert.equal(await pending, true)
})

test('production cancellation and overlapping requests never grant permission', async () => {
  const pending = confirmProductionOperation({ environment: 'production' }, 0, 'ttl', 2)
  assert.equal(await confirmProductionOperation({ environment: 'production' }, 1, 'delete'), false)
  resolveConfirmation(false)
  assert.equal(await pending, false)
})
