import assert from 'node:assert/strict'
import test from 'node:test'

import { createPinia, setActivePinia } from 'pinia'

import { useConnectionsStore } from './connections.js'

test('duplicate deep-copies every config field, clears identity and selects by unique name', async () => {
  setActivePinia(createPinia())
  const store = useConnectionsStore()
  const source = {
    id: 'original', name: 'Redis', group: ' prod ', created_at: 'old', updated_at: 'old', sort_order: 42,
    host: 'redis.internal', port: 6380, username: 'acl', password: ' secret ', db: 4,
    tls_enabled: true, tls_skip_verify: true, tls_ca_file: '/ca.pem', read_only: true,
    is_cluster: true, allow_cluster_scan: true, cluster_addrs: ['one:6379', 'two:6380'],
    proxy_enabled: false, proxy_url: 'socks5://user:pass@proxy:1080', icon_color: '#10b981',
    ssh_enabled: true, ssh_profile_id: 'jump', ssh: { host: 'jump', port: 22, user: 'ubuntu', password: ' pass ', passphrase: ' phrase ', private_key_path: '/key' },
  }
  store.connections = [source, { id: 'collision', name: ' REDIS COPY ', group: 'prod' }, { id: 'other-group', name: 'Redis copy 2', group: 'other' }]
  const original = JSON.parse(JSON.stringify(store.connections[0]))
  let saved
  store.save = async cfg => {
    saved = cfg
    store.connections.unshift({ ...cfg, id: 'new-id' })
    return { success: true }
  }
  const result = await store.duplicate('original')
  assert.equal(result.connection.id, 'new-id')
  const expected = { ...original, id: '', name: 'Redis copy 2' }
  delete expected.created_at
  delete expected.updated_at
  delete expected.sort_order
  assert.deepEqual(saved, expected)
  saved.cluster_addrs.push('three:6379')
  saved.ssh.password = 'changed'
  assert.deepEqual(store.connections.find(c => c.id === 'original'), original)
  assert.equal(store.connectedIds.has('new-id'), false)
})

test('duplicate handles repeated clicks and failed saves without mutating the source', async () => {
  setActivePinia(createPinia())
  const store = useConnectionsStore()
  store.connections = [{ id: 'original', name: 'Redis', group: '' }]
  let finish
  store.save = () => new Promise(resolve => { finish = resolve })
  const toasts = []
  store.showGlobalToast = (...args) => toasts.push(args)
  const pending = store.duplicate('original')
  assert.equal(store.duplicatingIds.has('original'), true)
  assert.equal((await store.duplicate('original')).cancelled, true)
  finish({ success: false, message: 'disk full' })
  assert.equal((await pending).success, false)
  assert.deepEqual(toasts, [['disk full', false]])
  assert.equal(store.duplicatingIds.size, 0)
  assert.deepEqual(store.connections, [{ id: 'original', name: 'Redis', group: '' }])
  store.save = async () => { throw new Error('write denied') }
  assert.equal((await store.duplicate('original')).message, 'write denied')
  assert.equal(store.duplicatingIds.size, 0)
})

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
