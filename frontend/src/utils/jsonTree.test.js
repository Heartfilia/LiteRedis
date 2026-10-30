import assert from 'node:assert/strict'
import test from 'node:test'

import { flattenJsonRows } from './jsonTree.js'

const data = {
  name: 'lite',
  tags: ['a', 'b'],
  nested: { deep: { value: 1 } },
}

test('flattenJsonRows: 默认只展开根节点', () => {
  const rows = flattenJsonRows(data, (path) => path === '')
  assert.deepEqual(
    rows.map((row) => row.depth + ':' + (row.label === null ? '<root>' : row.label)),
    ['0:<root>', '1:name', '1:tags', '1:nested']
  )
})

test('flattenJsonRows: 展开子节点后按顺序插入子行', () => {
  const expanded = new Set(['', '/tags', '/nested'])
  const rows = flattenJsonRows(data, (path) => expanded.has(path))
  assert.deepEqual(
    rows.map((row) => row.depth + ':' + (row.label === null ? '<root>' : row.label)),
    ['0:<root>', '1:name', '1:tags', '2:0', '2:1', '1:nested', '2:deep']
  )
})

test('flattenJsonRows: 叶子行带 isContainer=false，容器行带 true', () => {
  const rows = flattenJsonRows(data, (path) => path === '')
  const byLabel = Object.fromEntries(rows.map((row) => [String(row.label), row]))
  assert.equal(byLabel.name.isContainer, false)
  assert.equal(byLabel.tags.isContainer, true)
  assert.equal(byLabel['<root>'], undefined)
  assert.equal(rows[0].isContainer, true)
})

test('flattenJsonRows: key 里的 / 和 ~ 不会破坏路径', () => {
  const tricky = { 'a/b': { 'c~d': 1 } }
  const rows = flattenJsonRows(tricky, () => true)
  assert.deepEqual(
    rows.map((row) => row.path),
    ['', '/a~1b', '/a~1b/c~0d']
  )
})
