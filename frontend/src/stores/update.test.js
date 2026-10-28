import assert from 'node:assert/strict'
import test from 'node:test'
import { after } from 'node:test'

import { createPinia, setActivePinia } from 'pinia'

const calls = { check: 0, start: 0 }
let releaseResponse = { need_update: false, latest: '1.2.0' }
let startResponse = { success: true, success: true, needs_restart: false, message: '更新完成' }
let checkPending = null
let startPending = null

globalThis.window = {
  runtime: {
    EventsOnMultiple: () => () => {},
    EventsOn: () => () => {},
    EventsOff: () => {},
  },
  go: {
    main: {
      App: {
        CheckLatestRelease: () => {
          calls.check++
          if (checkPending) return checkPending
          return Promise.resolve(releaseResponse)
        },
        StartUpdate: () => {
          calls.start++
          if (startPending) return startPending
          return Promise.resolve(startResponse)
        },
      },
    },
  },
}

// 避免 store 里的自动清除定时器把测试进程挂住
const realSetTimeout = globalThis.setTimeout
const realClearTimeout = globalThis.clearTimeout
globalThis.setTimeout = () => 0
globalThis.clearTimeout = () => {}
after(() => {
  globalThis.setTimeout = realSetTimeout
  globalThis.clearTimeout = realClearTimeout
})

const { useUpdateStore } = await import('./update.js')

function freshStore() {
  setActivePinia(createPinia())
  return useUpdateStore()
}

function resetMocks() {
  calls.check = 0
  calls.start = 0
  releaseResponse = { need_update: false, latest: '1.2.0' }
  startResponse = { success: true, needs_restart: false, message: '更新完成' }
  checkPending = null
  startPending = null
}

test('没有可用更新时，按钮只触发检查', async () => {
  resetMocks()
  const store = freshStore()
  await store.runPrimaryAction()
  assert.equal(calls.check, 1)
  assert.equal(calls.start, 0)
  assert.equal(store.hasUpdate, false)
  assert.equal(store.checking, false)
})

test('检查到新版本后，再点一次按钮走安装', async () => {
  resetMocks()
  releaseResponse = { need_update: true, latest: '1.3.0' }
  const store = freshStore()

  await store.runPrimaryAction()
  assert.equal(calls.check, 1)
  assert.equal(calls.start, 0)
  assert.equal(store.hasUpdate, true)

  await store.runPrimaryAction()
  assert.equal(calls.check, 1, '第二次点击不应再走检查')
  assert.equal(calls.start, 1, '第二次点击应触发安装')
  assert.equal(store.hasUpdate, false)
  assert.equal(store.updating, false)
})

test('检查进行中重复点击不会重复请求', async () => {
  resetMocks()
  let resolveCheck
  checkPending = new Promise(resolve => { resolveCheck = resolve })
  const store = freshStore()

  const first = store.runPrimaryAction()
  await store.runPrimaryAction()
  assert.equal(calls.check, 1)
  resolveCheck({ need_update: false, latest: '1.2.0' })
  await first
  assert.equal(store.checking, false)
})

test('更新进行中重复点击不会重复安装', async () => {
  resetMocks()
  releaseResponse = { need_update: true, latest: '1.3.0' }
  const store = freshStore()
  await store.runPrimaryAction()

  let resolveStart
  startPending = new Promise(resolve => { resolveStart = resolve })
  const first = store.runPrimaryAction()
  assert.equal(store.updating, true)
  await store.runPrimaryAction()
  assert.equal(calls.start, 1, '更新中不应再次调用 StartUpdate')
  resolveStart({ success: true, needs_restart: true, message: '更新已就绪，应用正在重启' })
  await first
  assert.equal(store.stage, 'restart', '重启阶段应保持状态不复位')
  assert.equal(store.updating, true)
})

test('安装失败后保留“有更新”，再点按钮重试安装', async () => {
  resetMocks()
  releaseResponse = { need_update: true, latest: '1.3.0' }
  startResponse = { success: false, message: '下载失败' }
  const store = freshStore()
  await store.runPrimaryAction()

  await store.runPrimaryAction()
  assert.equal(calls.start, 1)
  assert.equal(store.hasUpdate, true, '失败后应保留更新标记以便重试')
  assert.equal(store.messageOk, false)

  await store.runPrimaryAction()
  assert.equal(calls.start, 2, '再次点击应重试安装')
})

test('检查出错时清空更新标记并给出错误提示', async () => {
  resetMocks()
  const store = freshStore()
  store.hasUpdate = true
  globalThis.window.go.main.App.CheckLatestRelease = () => Promise.resolve({ need_update: false, latest: '', error: 'GitHub 接口触发了 IP 访问限制' })
  await store.check()
  assert.equal(store.hasUpdate, false)
  assert.equal(store.messageOk, false)
  assert.match(store.message, /IP 访问限制/)
})
