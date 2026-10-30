import assert from 'node:assert/strict'
import test from 'node:test'

import { REDIS_COMMAND_HINTS, filterCommandNames, tokenizeCommandLine } from './redisCommands.js'

test('filterCommandNames: 按前缀匹配并保持字母序', () => {
  assert.deepEqual(filterCommandNames('zr'), ['zrange', 'zrangebyscore', 'zrank', 'zrem', 'zrevrange', 'zrevrank'])
})

test('filterCommandNames: 大小写与前后空格不影响结果', () => {
  assert.deepEqual(filterCommandNames('  GET '), ['get', 'getrange', 'getset'])
})

test('filterCommandNames: 空输入或已进入参数阶段返回空', () => {
  assert.deepEqual(filterCommandNames(''), [])
  assert.deepEqual(filterCommandNames('   '), [])
  assert.deepEqual(filterCommandNames('get my:key'), [])
})

test('filterCommandNames: 结果数量受 limit 限制', () => {
  assert.equal(filterCommandNames('s', 3).length, 3)
})

test('tokenizeCommandLine: 保留空白分隔符，第一段是命令名', () => {
  assert.deepEqual(tokenizeCommandLine('GET my:key'), ['GET', ' ', 'my:key'])
  assert.deepEqual(tokenizeCommandLine('hset user:1 name Ada'), ['hset', ' ', 'user:1', ' ', 'name', ' ', 'Ada'])
  assert.deepEqual(tokenizeCommandLine(''), [])
  assert.deepEqual(tokenizeCommandLine('  PING'), ['  ', 'PING'])
})

test('REDIS_COMMAND_HINTS: 常用命令都有参数签名', () => {
  for (const name of ['get', 'set', 'hset', 'lpush', 'zadd', 'xadd', 'scan']) {
    assert.ok(Array.isArray(REDIS_COMMAND_HINTS[name]), name + ' should have a signature')
  }
})
