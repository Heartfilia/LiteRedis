import test from 'node:test'
import assert from 'node:assert/strict'
import { createPinia, setActivePinia } from 'pinia'
import { useWorkspaceStore } from './workspace.js'
import { useConnectionsStore } from './connections.js'
import { confirmation, resolveConfirmation } from '../utils/confirm.js'

for (const [method, args] of [['batchDeleteSelected', []], ['batchSetTTLSelected', [10]], ['batchRenameSelectedPrefix', ['a:', 'b:']]]) {
  test(`${method} rejects changed DB or selection during production confirmation`, async () => {
    setActivePinia(createPinia())
    const workspace = useWorkspaceStore()
    const connections = useConnectionsStore()
    connections.connections = [{ id: 'p', name: 'Production', environment: 'production' }]
    workspace.activeConnID = 'p'
    workspace.currentDB = 4
    workspace.selectedKeys = ['a:1']
    let calls = 0
    globalThis.window = { go: { main: { App: new Proxy({}, { get: () => async () => { calls++; return { success: true } } }) } } }
    const result = workspace[method](...args)
    assert.ok(confirmation.value)
    workspace.currentDB = 5
    resolveConfirmation(true, confirmation.value.requiredText)
    assert.equal((await result).cancelled, true)
    assert.equal(calls, 0)
    delete globalThis.window
  })
}
