import assert from 'node:assert/strict'
import test from 'node:test'

import { moveRovingFocus } from './keyboard.js'

function makeRow() {
  const row = {
    offsetParent: {},
    focused: false,
    scrolled: false,
    focus() { this.focused = true },
    scrollIntoView() { this.scrolled = true },
  }
  return row
}

function makeEnv(count) {
  const rows = Array.from({ length: count }, makeRow)
  const container = { querySelectorAll: () => rows }
  const doc = { activeElement: null }
  globalThis.document = doc
  return { rows, container, doc }
}

function key(k) {
  const state = { prevented: false }
  return [k, state]
}

test('方向键从容器进入第一行，并在行之间移动', () => {
  const { rows, container, doc } = makeEnv(3)
  const [k, s] = key('ArrowDown')
  const event = { key: k, preventDefault: () => { s.prevented = true } }

  assert.equal(moveRovingFocus(event, container, '.row'), true)
  assert.equal(rows[0].focused, true)
  assert.equal(s.prevented, true)

  rows.forEach(r => { r.focused = false })
  doc.activeElement = rows[0]
  moveRovingFocus(event, container, '.row')
  assert.equal(rows[1].focused, true)

  rows.forEach(r => { r.focused = false })
  doc.activeElement = rows[2]
  moveRovingFocus(event, container, '.row')
  assert.equal(rows[2].focused, true, '最后一行继续按 ↓ 应该停在原地')
})

test('↑ 在首行停住，Home/End 跳到两端', () => {
  const { rows, container, doc } = makeEnv(4)
  const noop = () => {}

  moveRovingFocus({ key: 'ArrowUp', preventDefault: noop }, container, '.row')
  assert.equal(rows[0].focused, true)

  rows.forEach(r => { r.focused = false })
  doc.activeElement = rows[2]
  moveRovingFocus({ key: 'ArrowUp', preventDefault: noop }, container, '.row')
  assert.equal(rows[1].focused, true)

  rows.forEach(r => { r.focused = false })
  moveRovingFocus({ key: 'End', preventDefault: noop }, container, '.row')
  assert.equal(rows[3].focused, true)

  rows.forEach(r => { r.focused = false })
  moveRovingFocus({ key: 'Home', preventDefault: noop }, container, '.row')
  assert.equal(rows[0].focused, true)
})

test('其他按键不拦截（Tab / Enter / 字母）', () => {
  const { rows, container } = makeEnv(2)
  let prevented = false
  for (const k of ['Tab', 'Enter', 'a', 'Escape']) {
    prevented = false
    assert.equal(
      moveRovingFocus({ key: k, preventDefault: () => { prevented = true } }, container, '.row'),
      false
    )
    assert.equal(prevented, false)
  }
  assert.equal(rows[0].focused, false)
})

test('没有可见行时安全返回', () => {
  const container = { querySelectorAll: () => [] }
  globalThis.document = { activeElement: null }
  let prevented = false
  assert.equal(
    moveRovingFocus({ key: 'ArrowDown', preventDefault: () => { prevented = true } }, container, '.row'),
    false
  )
  assert.equal(prevented, false)

  // 隐藏的行（offsetParent 为空）也要被过滤掉
  const hidden = { offsetParent: null, focus() { this.focusCalled = true } }
  const hiddenContainer = { querySelectorAll: () => [hidden] }
  assert.equal(
    moveRovingFocus({ key: 'ArrowDown', preventDefault: () => {} }, hiddenContainer, '.row'),
    false
  )
})

function makeScroller(box, metrics) {
  const scroller = {
    __style: { overflowY: 'auto' },
    scrollTop: 0,
    scrollHeight: metrics.scrollHeight,
    clientHeight: metrics.clientHeight,
    getBoundingClientRect: () => box,
    parentElement: null,
  }
  return scroller
}

test('revealRow 把行滚进最近的滚动容器（下方越界）', async () => {
  const { revealRow } = await import('./keyboard.js')
  globalThis.window = { getComputedStyle: (node) => node.__style }
  globalThis.document = { body: {}, documentElement: {} }

  const scroller = makeScroller({ top: 100, bottom: 300 }, { scrollHeight: 1000, clientHeight: 200 })
  const row = {
    getBoundingClientRect: () => ({ top: 260, bottom: 340 }),
    parentElement: scroller,
  }
  revealRow(row)
  // 下边界 300 - 8 = 292，行底 340 越界 48
  assert.equal(scroller.scrollTop, 48)
})

test('revealRow 把行滚进最近的滚动容器（上方越界）', async () => {
  const { revealRow } = await import('./keyboard.js')
  globalThis.window = { getComputedStyle: (node) => node.__style }
  globalThis.document = { body: {}, documentElement: {} }

  const scroller = makeScroller({ top: 100, bottom: 300 }, { scrollHeight: 1000, clientHeight: 200 })
  const row = {
    getBoundingClientRect: () => ({ top: 90, bottom: 120 }),
    parentElement: scroller,
  }
  revealRow(row)
  // 上边界 100 + 8 = 108，行顶 90 越界 18
  assert.equal(scroller.scrollTop, -18)
})

test('行完全可见时不滚动', async () => {
  const { revealRow } = await import('./keyboard.js')
  globalThis.window = { getComputedStyle: (node) => node.__style }
  globalThis.document = { body: {}, documentElement: {} }

  const scroller = makeScroller({ top: 100, bottom: 300 }, { scrollHeight: 1000, clientHeight: 200 })
  const row = { getBoundingClientRect: () => ({ top: 150, bottom: 180 }), parentElement: scroller }
  revealRow(row)
  assert.equal(scroller.scrollTop, 0)
})

test('找不到可滚动容器时安全返回', async () => {
  const { revealRow } = await import('./keyboard.js')
  const row = { getBoundingClientRect: () => ({ top: 0, bottom: 10 }), parentElement: null }
  revealRow(row)
  revealRow(null)
})
