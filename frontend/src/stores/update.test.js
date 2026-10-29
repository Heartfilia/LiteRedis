import assert from 'node:assert/strict'
import test, { after } from 'node:test'

import { createPinia, setActivePinia } from 'pinia'

const state = {
  check: 0,
  start: 0,
  release: { need_update: false, latest: '1.2.0' },
  startResult: { success: true, needs_restart: false, message: '更新完成' },
  checkPending: null,
  startPending: null,
}

function installMocks() {
  globalThis.window.go.main.App = {
    CheckLatestRelease: () => {
      state.check++
      return state.checkPending || Promise.resolve(state.release)
    },
    StartUpdate: () => {
      state.start++
      return state.startPending || Promise.resolve(state.startResult)
    },
  }
}

globalThis.window = {
  runtime: {
    EventsOnMultiple: () => () => {},
    EventsOn: () => () => {},
    EventsOff: () => {},
  },
  go: { main: { App: {} } },
}
installMocks()

const realSetTimeout = globalThis.setTimeout
const realClearTimeout = globalThis.clearTimeout
globalThis.setTimeout = () => 0
globalThis.clearTimeout = () => {}
after(() => {
  globalThis.setTimeout = realSetTimeout
  globalThis.clearTimeout = realClearTimeout
})

const { useUpdateStore } = await import('./update.js')

function resetMocks(overrides = {}) {
  state.check = 0
  state.start = 0
  state.release = { need_update: false, latest: '1.2.0' }
  state.startResult = { success: true, needs_restart: false, message: '更新完成' }
  state.checkPending = null
  state.startPending = null
  Object.assign(state, overrides)
  installMocks()
}

function freshStore() {
  setActivePinia(createPinia())
  return useUpdateStore()
}

test('没有可用更新时，按钮只触发检查', async () => {
  resetMocks()
  const store = freshStore()
  await store.runPrimaryAction()
  assert.equal(state.check, 1)
  assert.equal(state.start, 0)
  assert.equal(store.hasUpdate, false)
  assert.equal(store.checking, false)
})

test('检查到新版本后，再点一次按钮走安装', async () => {
  resetMocks({ release: { need_update: true, latest: '1.3.0' } })
  const store = freshStore()

  await store.runPrimaryAction()
  assert.equal(state.check, 1)
  assert.equal(state.start, 0)
  assert.equal(store.hasUpdate, true)

  await store.runPrimaryAction()
  assert.equal(state.check, 1, '第二次点击不应再走检查')
  assert.equal(state.start, 1, '第二次点击应触发安装')
  assert.equal(store.hasUpdate, false)
  assert.equal(store.updating, false)
})

test('检查进行中重复点击不会重复请求', async () => {
  resetMocks()
  let resolveCheck
  state.checkPending = new Promise(resolve => { resolveCheck = resolve })
  const store = freshStore()

  const first = store.runPrimaryAction()
  await store.runPrimaryAction()
  assert.equal(state.check, 1)
  resolveCheck({ need_update: false, latest: '1.2.0' })
  await first
  assert.equal(store.checking, false)
})

test('更新进行中重复点击不会重复安装', async () => {
  resetMocks({ release: { need_update: true, latest: '1.3.0' } })
  const store = freshStore()
  await store.runPrimaryAction()

  let resolveStart
  state.startPending = new Promise(resolve => { resolveStart = resolve })
  const first = store.runPrimaryAction()
  assert.equal(store.updating, true)
  await store.runPrimaryAction()
  assert.equal(state.start, 1, '更新中不应再次调用 StartUpdate')
  resolveStart({ success: true, needs_restart: true, message: '更新已就绪，应用正在重启' })
  await first
  assert.equal(store.stage, 'restart', '重启阶段应保持状态不复位')
  assert.equal(store.updating, true)
})

test('安装失败后保留“有更新”，再点按钮重试安装', async () => {
  resetMocks({
    release: { need_update: true, latest: '1.3.0' },
    startResult: { success: false, message: '下载失败' },
  })
  const store = freshStore()
  await store.runPrimaryAction()

  await store.runPrimaryAction()
  assert.equal(state.start, 1)
  assert.equal(store.hasUpdate, true, '失败后应保留更新标记以便重试')
  assert.equal(store.messageOk, false)

  await store.runPrimaryAction()
  assert.equal(state.start, 2, '再次点击应重试安装')
})

test('检查出错时清空更新标记并给出错误提示', async () => {
  resetMocks({ release: { need_update: false, latest: '', error: 'GitHub 接口触发了 IP 访问限制' } })
  const store = freshStore()
  store.hasUpdate = true
  await store.check()
  assert.equal(store.hasUpdate, false)
  assert.equal(store.messageOk, false)
  assert.match(store.message, /IP 访问限制/)
})

test('重启态：状态保持、再次点击不触发任何请求', async () => {
  resetMocks({
    release: { need_update: true, latest: '1.3.0' },
    startResult: { success: true, needs_restart: true, message: '更新已就绪，应用正在重启' },
  })
  const store = freshStore()
  await store.runPrimaryAction()
  await store.runPrimaryAction()

  assert.equal(state.start, 1)
  assert.equal(store.updating, true, '重启态必须保持 updating，按钮才不会闪回版本号')
  assert.equal(store.restarting, true)
  assert.equal(store.stage, 'restart')
  assert.equal(store.percent, 100)
  assert.equal(store.hasUpdate, false, '已安装成功，不该再显示“执行更新”')
  assert.equal(store.messageOk, true)
  assert.equal(store.message, '更新已就绪，应用正在重启')

  await store.runPrimaryAction()
  await store.install()
  await store.check()
  assert.equal(state.check, 1, '重启态不应重新检查')
  assert.equal(state.start, 1, '重启态不应重复安装')
  assert.equal(store.restarting, true)
})

test('重启态：迟到的进度事件被忽略，状态保持重启', async () => {
  resetMocks({
    release: { need_update: true, latest: '1.3.0' },
    startResult: { success: true, needs_restart: true, message: '更新已就绪，应用正在重启' },
  })
  const store = freshStore()
  await store.runPrimaryAction()
  await store.runPrimaryAction()

  store.onProgress({ stage: 'download', percent: 30 })
  assert.equal(store.stage, 'restart', '重启阶段的迟到事件不应覆盖阶段')
  assert.equal(store.restarting, true)
  assert.equal(store.updating, true)
})

test('安装失败（needs_restart=false）不进入重启态', async () => {
  resetMocks({
    release: { need_update: true, latest: '1.3.0' },
    startResult: { success: false, needs_restart: false, message: '校验失败' },
  })
  const store = freshStore()
  await store.runPrimaryAction()
  await store.runPrimaryAction()

  assert.equal(store.updating, false)
  assert.equal(store.restarting, false)
  assert.equal(store.hasUpdate, true)
  assert.equal(store.messageOk, false)
  assert.equal(store.message, '校验失败')
})

test('安装过程中抛异常：状态复位，可再次重试', async () => {
  resetMocks({ release: { need_update: true, latest: '1.3.0' } })
  state.startPending = Promise.reject(new Error('连接被重置'))
  state.startPending.catch(() => {})
  const store = freshStore()
  await store.runPrimaryAction()
  await store.runPrimaryAction()

  assert.equal(store.updating, false, '异常后必须复位，否则按钮永久 disabled')
  assert.equal(store.restarting, false)
  assert.equal(store.hasUpdate, true)
  assert.equal(store.messageOk, false)
  assert.match(store.message, /连接被重置/)

  resetMocks({ release: { need_update: true, latest: '1.3.0' } })
  await store.runPrimaryAction()
  assert.equal(state.start, 1, '异常后允许重试')
})
test('restarting getter 边界：必须同时满足 updating 与 stage=restart', async () => {
  resetMocks()
  const store = freshStore()

  // 默认态
  assert.equal(store.restarting, false, '初始状态不应是重启态')

  // 各阶段但未在更新
  for (const stage of ['download', 'verify', 'install', 'restart']) {
    store.stage = stage
    store.updating = false
    assert.equal(store.restarting, false, `未在更新时 stage=${stage} 不应判定为重启态`)
  }

  // 更新中，非重启阶段
  store.updating = true
  for (const stage of ['download', 'verify', 'install', '']) {
    store.stage = stage
    assert.equal(store.restarting, false, `stage=${stage || '(空)'} 不应判定为重启态`)
  }

  // 大小写敏感：只认小写 restart
  store.stage = 'Restart'
  assert.equal(store.restarting, false, 'stage 大小写不匹配时不应判定为重启态')

  // 唯一致真组合
  store.stage = 'restart'
  store.updating = true
  assert.equal(store.restarting, true)

  // 检查中不干扰
  store.checking = true
  assert.equal(store.restarting, true, 'checking 不应影响重启态判定')
  store.checking = false

  // 残留 restart 阶段但更新已结束（例如退出失败后手动复位）→ 按钮必须能恢复
  store.updating = false
  assert.equal(store.restarting, false, 'updating 复位后重启态必须结束')
})
test('静默检查（打开设置时自动触发）：失败不弹提示', async () => {
  resetMocks({ release: { need_update: false, latest: '', error: 'GitHub 接口触发了 IP 访问限制' } })
  const store = freshStore()
  await store.check({ silent: true })
  assert.equal(store.hasUpdate, false)
  assert.equal(store.message, '', '自动检查失败必须静默')
  assert.equal(store.messageOk, true, '提示状态不应被自动检查改写')
  assert.equal(store.checking, false)
})

test('静默检查：发现新版本只更新状态，不弹提示', async () => {
  resetMocks({ release: { need_update: true, latest: '1.4.0' } })
  const store = freshStore()
  await store.check({ silent: true })
  assert.equal(store.hasUpdate, true)
  assert.equal(store.latest, '1.4.0')
  assert.equal(store.message, '', '自动检查不应弹出“发现新版本”提示')
})

test('静默检查：已是最新版本同样不弹提示', async () => {
  resetMocks({ release: { need_update: false, latest: '1.4.0' } })
  const store = freshStore()
  await store.check({ silent: true })
  assert.equal(store.hasUpdate, false)
  assert.equal(store.message, '')
})

test('静默检查失败不会影响已有提示以外的行为：随后手动检查仍会提示异常', async () => {
  resetMocks({ release: { need_update: false, latest: '', error: '代理连接失败' } })
  const store = freshStore()
  await store.check({ silent: true })
  assert.equal(store.message, '')

  await store.check()
  assert.equal(store.messageOk, false)
  assert.match(store.message, /代理连接失败/, '手动检查失败必须提示')
})

test('静默检查遇到进行中的更新不会打断：直接跳过', async () => {
  resetMocks({ release: { need_update: true, latest: '1.4.0' } })
  const store = freshStore()
  store.updating = true
  await store.check({ silent: true })
  assert.equal(state.check, 0, '更新中不应发起检查')
  assert.equal(store.checking, false)
})
