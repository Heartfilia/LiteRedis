import assert from 'node:assert/strict'
import test from 'node:test'
import { parseCommandTemplate, prepareCommandTemplate, quoteRedisArgument } from './commandTemplates.js'

test('template accepts static quoted tokens and deduplicates whole parameters', () => {
  assert.deepEqual(parseCommandTemplate('SET {{key}} {{value}} NX').parameters, ['key', 'value'])
  assert.deepEqual(parseCommandTemplate('MGET {{key}} {{key}}').parameters, ['key'])
  assert.equal(prepareCommandTemplate('GET {{key}}', { key: '' }), 'GET ""')
  assert.equal(prepareCommandTemplate('PING'), 'PING')
  assert.equal(prepareCommandTemplate('SET "static key" {{value}}', { value: 'a b' }), 'SET "static key" "a b"')
})

test('template rejects partial, quoted, malformed and command-name placeholders', () => {
  for (const template of ['', '{{cmd}} key', 'GET prefix{{key}}', 'GET "{{key}}"', 'GET {{bad-name}}', 'GET {{key', 'GET key}}', 'GET "unclosed', 'GET a\\', 'GET a\nFLUSHALL', 'GET a\r', 'GET a\0', 'x'.repeat(4097)]) {
    assert.throws(() => parseCommandTemplate(template), template)
  }
  assert.throws(() => prepareCommandTemplate('GET {{key}}', {}))
  assert.throws(() => prepareCommandTemplate('GET {{toString}}', {}))
})

test('parameters quote spaces, quotes and slash without creating additional arguments', () => {
  const raw = 'a" \\ FLUSHALL " b\tend'
  const prepared = prepareCommandTemplate('GET {{key}}', { key: raw })
  assert.equal(prepared, 'GET "a\\" \\\\ FLUSHALL \\" b\tend"')
  // Mirror the console splitter: backslashes escape the next character, quotes
  // protect whitespace. The payload must still be exactly one argument.
  const tokens = []
  let token = '', quote = '', escaped = false
  for (const character of prepared) {
    if (escaped) { token += character; escaped = false }
    else if (character === '\\') escaped = true
    else if (quote) { if (character === quote) quote = ''; else token += character }
    else if (character === '"' || character === "'") quote = character
    else if (/\s/.test(character)) { tokens.push(token); token = '' }
    else token += character
  }
  tokens.push(token)
  assert.deepEqual(tokens, ['GET', raw])
})

test('CR LF NUL parameters are rejected rather than altered or injected', () => {
  for (const value of ['a\nb', 'a\rb', '\0', 123, null]) assert.throws(() => quoteRedisArgument(value))
})
