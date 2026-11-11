import { test } from 'node:test'
import assert from 'node:assert/strict'
import { formatJsonLossless, parseJsonLossless, stringifyJsonLossless, JsonNumber } from './losslessJson.js'

test('JSON formatting preserves numeric lexemes, duplicate keys and escapes', () => {
  const raw = '{"id":9223372036854775807,"n":1.2300e+400,"z":-0,"id":2,"s":"\\u4f60\\n"}'
  const formatted = formatJsonLossless(raw)
  for (const text of ['9223372036854775807', '1.2300e+400', '-0', '"\\u4f60\\n"']) assert.ok(formatted.includes(text))
  assert.equal((formatted.match(/"id"/g) || []).length, 2)
  assert.equal(formatJsonLossless(formatted), formatted)
})

test('tree copy preserves numeric text and rejects lossy duplicate-key preview', () => {
  const raw = '{"id":9223372036854775807,"number":1.2300e400,"zero":-0}'
  assert.equal(stringifyJsonLossless(parseJsonLossless(raw)), raw)
  assert.throws(() => parseJsonLossless('{"id":1,"id":2}'), /Duplicate keys/)
})

test('JSON depth, input size and formatting growth are bounded', () => {
  const nested = '['.repeat(258) + '1' + ']'.repeat(258)
  assert.throws(() => formatJsonLossless(nested), /nesting/)
  assert.throws(() => parseJsonLossless(nested), /nesting/)
  assert.throws(() => formatJsonLossless('"' + 'x'.repeat(2 * 1024 * 1024) + '"'), /limit/)
  const wide = '['.repeat(200) + '[' + '0,'.repeat(22000) + '0]' + ']'.repeat(200)
  assert.throws(() => formatJsonLossless(wide), /limit/)
})

test('lossless preview uses numeric leaves and handles prototype keys safely', () => {
  const value = parseJsonLossless('{"__proto__":{"id":9223372036854775807},"list":[-0,1e400,null,true,"1"]}')
  assert.equal(Object.getPrototypeOf(value), null)
  assert.ok(value.__proto__.id instanceof JsonNumber)
  assert.equal(String(value.__proto__.id), '9223372036854775807')
  assert.equal(String(value.list[1]), '1e400')
  assert.equal(value.list[4], '1')
})

test('lossless formatting supports root scalars, empty containers and invalid JSON', () => {
  for (const raw of ['[]', '{}', '[{},[],1]', '"hello"', 'null', '-1.00']) assert.doesNotThrow(() => parseJsonLossless(formatJsonLossless(raw)))
  for (const raw of ['[1,]', '{"x":01}', 'undefined', '']) assert.throws(() => formatJsonLossless(raw))
})
