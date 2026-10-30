import assert from 'node:assert/strict'
import test from 'node:test'

import { computeWindow } from './virtualList.js'

test('computeWindow: 顶部时从 0 开始，上下留白正确', () => {
  const w = computeWindow(10000, 0, 600, 32, 8)
  assert.equal(w.start, 0)
  assert.equal(w.end, Math.ceil(600 / 32) + 8)
  assert.equal(w.topPad, 0)
  assert.equal(w.bottomPad, (10000 - w.end) * 32)
})

test('computeWindow: 滚动到中间时窗口跟随，留白合计补齐总高度', () => {
  const rowHeight = 32
  const total = 5000
  const scrollTop = 32000 // 第 1000 行
  const viewport = 640
  const w = computeWindow(total, scrollTop, viewport, rowHeight, 8)
  assert.equal(w.start, 1000 - 8)
  assert.ok(w.end > 1000)
  assert.equal(w.topPad, (1000 - 8) * rowHeight)
  const rendered = (w.end - w.start) * rowHeight
  assert.equal(w.topPad + rendered + w.bottomPad, total * rowHeight, '留白 + 渲染高度应等于内容总高')
})

test('computeWindow: 滚到底部时不越界', () => {
  const rowHeight = 32
  const total = 500
  const viewport = 600
  const w = computeWindow(total, (total - 1) * rowHeight, viewport, rowHeight, 8)
  assert.equal(w.end, total)
  assert.equal(w.bottomPad, 0)
  assert.ok(w.start <= total)
})

test('computeWindow: 数据不足一屏或行高未知时全量渲染', () => {
  assert.deepEqual(computeWindow(5, 0, 600, 32, 8), { start: 0, end: 5, topPad: 0, bottomPad: 0 })
  assert.deepEqual(computeWindow(0, 0, 600, 32, 8), { start: 0, end: 0, topPad: 0, bottomPad: 0 })
  assert.deepEqual(computeWindow(1000, 100, 600, 0, 8), { start: 0, end: 1000, topPad: 0, bottomPad: 0 })
})

test('computeWindow: 异常入参不会产生负索引', () => {
  const w = computeWindow(-5, -100, -50, 32, 8)
  assert.equal(w.start, 0)
  assert.equal(w.end, 0)
  assert.equal(w.topPad, 0)
})
