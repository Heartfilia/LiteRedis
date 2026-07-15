import assert from 'node:assert/strict'
import test from 'node:test'

import { createPinia, setActivePinia } from 'pinia'

import { useConnectionsStore } from './connections.js'

test('heartbeat schedules serial cycles and does not restart after stop', async (t) => {
  const originalSetTimeout = globalThis.setTimeout
  const originalClearTimeout = globalThis.clearTimeout
  const scheduled = []
  const cleared = []
  globalThis.setTimeout = (callback, delay) => {
    const timer = { callback, delay, id: scheduled.length + 1 }
    scheduled.push(timer)
    return timer.id
  }
  globalThis.clearTimeout = id => cleared.push(id)
  t.after(() => {
    globalThis.setTimeout = originalSetTimeout
    globalThis.clearTimeout = originalClearTimeout
  })

  setActivePinia(createPinia())
  const store = useConnectionsStore()
  let resolveCycle
  store._runHeartbeatCycle = () => new Promise(resolve => {
    resolveCycle = resolve
  })

  store.startHeartbeat({})
  store.startHeartbeat({})
  assert.equal(scheduled.length, 1)
  assert.equal(scheduled[0].delay, 20000)

  const runningCycle = scheduled[0].callback()
  assert.equal(store.heartbeatInFlight, true)
  assert.equal(scheduled.length, 1)

  store.stopHeartbeat()
  resolveCycle()
  await runningCycle

  assert.equal(store.heartbeatRunning, false)
  assert.equal(store.heartbeatInFlight, false)
  assert.equal(scheduled.length, 1)
})

test('connection failure reporting honors thresholds and ignores business errors', async () => {
  setActivePinia(createPinia())
  const store = useConnectionsStore()
  store.connectedIds.add('conn-1')

  const ignored = await store.reportConnectionFailure('conn-1', 'ERR wrong type', 2)
  assert.equal(ignored, null)
  assert.equal(store.heartbeatFailures['conn-1'] || 0, 0)

  const first = await store.reportConnectionFailure('conn-1', 'i/o timeout', 2)
  assert.equal(first.disconnected, false)
  assert.equal(store.connectedIds.has('conn-1'), true)

  store.reportConnectionSuccess('conn-1')
  assert.equal(store.heartbeatFailures['conn-1'], 0)

  await store.reportConnectionFailure('conn-1', 'connection reset', 2)
  const second = await store.reportConnectionFailure('conn-1', 'connection reset', 2)
  assert.equal(second.disconnected, true)
  assert.equal(store.connectedIds.has('conn-1'), false)
})
