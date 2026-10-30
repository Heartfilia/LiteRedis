import assert from 'node:assert/strict'
import test from 'node:test'

import {
  REDIS_COMMAND_HINTS,
  filterCommandNames,
  suggestArguments,
  tokenizeCommandLine,
} from './redisCommands.js'

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
test('suggestArguments: 命令后的第一个参数补子命令', () => {
  assert.deepEqual(
    suggestArguments('config ').map((item) => item.value),
    ['GET', 'HELP', 'RESETSTAT', 'REWRITE', 'SET']
  )
  assert.deepEqual(
    suggestArguments('config ge').map((item) => item.value),
    ['GET']
  )
  assert.deepEqual(suggestArguments('object ').map((item) => item.value), ['ENCODING', 'FREQ', 'IDLETIME', 'REFCOUNT'])
})

test('suggestArguments: 参数阶段补关键字并按已输入内容过滤', () => {
  assert.deepEqual(
    suggestArguments('set my:key v ').map((item) => item.value),
    ['EX', 'EXAT', 'GET', 'KEEPTTL', 'NX', 'PX', 'PXAT', 'XX']
  )
  assert.deepEqual(suggestArguments('set my:key v n').map((item) => item.value), ['NX'])
  // 已经写过的选项不再推荐
  assert.deepEqual(suggestArguments('set my:key v NX ').map((item) => item.value).includes('NX'), false)
})

test('suggestArguments: 命令名阶段、无子命令无关键字的命令返回空', () => {
  assert.deepEqual(suggestArguments('get'), [])
  assert.deepEqual(suggestArguments('get '), [])
  assert.deepEqual(suggestArguments('config get '), [])
  assert.deepEqual(suggestArguments(''), [])
})

test('suggestArguments: 扫描类命令支持 MATCH / COUNT', () => {
  assert.deepEqual(suggestArguments('scan 0 ').map((item) => item.value), ['COUNT', 'MATCH', 'TYPE'])
  assert.deepEqual(suggestArguments('zrange key 0 -1 wi').map((item) => item.value), ['WITHSCORES'])
})
