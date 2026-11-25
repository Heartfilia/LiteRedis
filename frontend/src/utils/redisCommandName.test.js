import test from 'node:test'
import assert from 'node:assert/strict'
import { redisCommandName, isReadOnlyConsoleCommand } from './redisCommandName.js'

test('extracts complete command using console quote and escape semantics', () => {
  for (const [input, expected] of [
    ['  get key', 'GET'], ['\t\n"GET" key', 'GET'],
    ["'G'ET key", 'GET'], ['G\\ET key', 'GET'],
    ['GET"DEL" victim', 'GETDEL'], ["GET'EX' victim EX 1", 'GETEX'],
    ['GET\\DEL victim', 'GETDEL'], ['GET\\ key', 'GET KEY'],
    ['"" GET key', ''], ['GET\\', 'GET\\'],
    ['GET\r key', 'GET\r'], ['GET\u00a0key', 'GET\u00a0KEY'],
    ['GET key\\', 'GET'], ['', ''],
  ]) assert.equal(redisCommandName(input), expected, input)
})

test('production allowlist cannot be bypassed with adjacent command fragments', () => {
  for (const input of ['GET"DEL" victim', "GET'EX' victim EX 1", 'GET\\DEL victim', 'GETSET key value', 'GET-DEL key', 'SET key value', 'FLUSHALL']) {
    assert.equal(isReadOnlyConsoleCommand(input), false, input)
  }
  for (const input of ['GET key', '"GET" key', "'G'ET key", 'G\\ET key', 'SCAN 0 MATCH *']) {
    assert.equal(isReadOnlyConsoleCommand(input), true, input)
  }
})

test('malformed input is unknown even when command starts read-only', () => {
  for (const input of ['GET "unterminated', "GET key 'unterminated", '"GET', null, undefined, 123]) {
    assert.equal(redisCommandName(input), '', String(input))
    assert.equal(isReadOnlyConsoleCommand(input), false, String(input))
  }
})
