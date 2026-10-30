import assert from 'node:assert/strict'
import test from 'node:test'

import { buildKeyTree, findKeyAncestors, flattenVisibleRows } from './keyTree.js'

const keys = [
  { name: 'user:1:name', type: 'string', ttl: -1 },
  { name: 'user:1:mail', type: 'string', ttl: -1 },
  { name: 'user:2:name', type: 'string', ttl: -1 },
  { name: 'cart:9', type: 'hash', ttl: -1 },
]

test('flattenVisibleRows: 默认全部展开，深度优先', () => {
  const tree = buildKeyTree(keys)
  const rows = flattenVisibleRows(tree)
  assert.deepEqual(
    rows.map((row) => row.depth + ':' + row.node.label),
    ['0:cart', '1:9', '0:user', '1:1', '2:mail', '2:name', '1:2', '2:name']
  )
})

test('flattenVisibleRows: 折叠的目录不输出子节点', () => {
  const tree = buildKeyTree(keys)
  const rows = flattenVisibleRows(tree, (node) => node.label !== 'user')
  assert.deepEqual(
    rows.map((row) => row.node.label),
    ['cart', '9', 'user']
  )
})

test('flattenVisibleRows: 折叠只影响自己这一层', () => {
  const tree = buildKeyTree(keys)
  // 只折叠 user/1 这一支，user/2 仍然展开
  const rows = flattenVisibleRows(tree, (node) => node.label !== '1')
  assert.deepEqual(
    rows.map((row) => row.depth + ':' + row.node.label),
    ['0:cart', '1:9', '0:user', '1:1', '1:2', '2:name']
  )
})

test('flattenVisibleRows: 空树返回空数组', () => {
  assert.deepEqual(flattenVisibleRows([]), [])
  assert.deepEqual(flattenVisibleRows(undefined), [])
})
const ancestorKeys = [
  { name: 'user:1:name', type: 'string', ttl: -1 },
  { name: 'user:1:mail', type: 'string', ttl: -1 },
  { name: 'cart:9', type: 'hash', ttl: -1 },
  { name: 'ping', type: 'string', ttl: -1 },
]

test('findKeyAncestors: 返回从根到父级的目录链', () => {
  const tree = buildKeyTree(ancestorKeys)
  assert.deepEqual(findKeyAncestors(tree, 'user:1:name'), ['user', 'user:1'])
  assert.deepEqual(findKeyAncestors(tree, 'cart:9'), ['cart'])
})

test('findKeyAncestors: 顶层 key 返回空数组，找不到返回 null', () => {
  const tree = buildKeyTree(ancestorKeys)
  assert.deepEqual(findKeyAncestors(tree, 'ping'), [])
  assert.equal(findKeyAncestors(tree, 'nope:1'), null)
  assert.equal(findKeyAncestors(tree, ''), null)
})
