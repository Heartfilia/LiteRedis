// Match backend/redis/overview.go splitCommand: escapes consume the next
// character even inside quotes; adjacent quoted/unquoted fragments are one token.
// Parse the entire input so malformed arguments are not classified as read-only.
export function redisCommandName(input) {
  if (typeof input !== 'string') return ''
  let first = null
  let token = ''
  let started = false
  let quote = ''
  let escaped = false
  function flush() {
    if (!started) return
    if (first === null) first = token
    token = ''
    started = false
  }
  for (const character of input) {
    if (escaped) {
      token += character
      started = true
      escaped = false
    } else if (character === '\\') {
      escaped = true
      started = true
    } else if (quote) {
      if (character === quote) quote = ''
      else token += character
    } else if (character === "'" || character === '"') {
      quote = character
      started = true
    } else if (character === ' ' || character === '\t' || character === '\n') {
      flush()
    } else {
      token += character
      started = true
    }
  }
  if (quote) return ''
  // The backend treats a trailing escape as a literal backslash.
  if (escaped) { token += '\\'; started = true }
  flush()
  return (first ?? '').toUpperCase()
}

const READ_ONLY_CONSOLE_COMMANDS = new Set([
  'PING', 'GET', 'MGET', 'EXISTS', 'TYPE', 'TTL', 'PTTL', 'STRLEN',
  'HGET', 'HGETALL', 'HLEN', 'LRANGE', 'LLEN', 'SCARD', 'SMEMBERS',
  'ZRANGE', 'ZCARD', 'INFO', 'DBSIZE', 'SCAN', 'HSCAN', 'SSCAN', 'ZSCAN',
])

export function isReadOnlyConsoleCommand(input) {
  return READ_ONLY_CONSOLE_COMMANDS.has(redisCommandName(input))
}
