/**
 * Redis 命令名 -> 参数签名，用于命令台的参数提示和命令名补全
 */
export const REDIS_COMMAND_HINTS = {
  append: ['key', 'value'],
  decr: ['key'],
  decrby: ['key', 'decrement'],
  del: ['key', '[key ...]'],
  exists: ['key', '[key ...]'],
  expire: ['key', 'seconds'],
  get: ['key'],
  hdel: ['key', 'field', '[field ...]'],
  hexists: ['key', 'field'],
  hget: ['key', 'field'],
  hgetall: ['key'],
  hincrby: ['key', 'field', 'increment'],
  hkeys: ['key'],
  hlen: ['key'],
  hmget: ['key', 'field', '[field ...]'],
  hmset: ['key', 'field', 'value', '[field value ...]'],
  hset: ['key', 'field', 'value', '[field value ...]'],
  hvals: ['key'],
  incr: ['key'],
  incrby: ['key', 'increment'],
  info: ['[section]'],
  keys: ['pattern'],
  llen: ['key'],
  lpop: ['key', '[count]'],
  lpush: ['key', 'element', '[element ...]'],
  lrange: ['key', 'start', 'stop'],
  lrem: ['key', 'count', 'element'],
  lset: ['key', 'index', 'element'],
  ltrim: ['key', 'start', 'stop'],
  mget: ['key', '[key ...]'],
  mset: ['key', 'value', '[key value ...]'],
  persist: ['key'],
  ping: ['[message]'],
  rename: ['key', 'newkey'],
  rpop: ['key', '[count]'],
  rpush: ['key', 'element', '[element ...]'],
  sadd: ['key', 'member', '[member ...]'],
  scard: ['key'],
  set: ['key', 'value', '[EX seconds|PX milliseconds|NX|XX ...]'],
  select: ['index'],
  setex: ['key', 'seconds', 'value'],
  sembers: ['key'],
  smembers: ['key'],
  sismember: ['key', 'member'],
  smismember: ['key', 'member', '[member ...]'],
  spop: ['key', '[count]'],
  srem: ['key', 'member', '[member ...]'],
  strlen: ['key'],
  ttl: ['key'],
  type: ['key'],
  unlink: ['key', '[key ...]'],
  zadd: ['key', 'score', 'member', '[score member ...]'],
  zcard: ['key'],
  zcount: ['key', 'min', 'max'],
  zrange: ['key', 'start', 'stop', '[WITHSCORES]'],
  zrangebyscore: ['key', 'min', 'max', '[WITHSCORES]', '[LIMIT offset count]'],
  zrank: ['key', 'member'],
  zrem: ['key', 'member', '[member ...]'],
  zrevrange: ['key', 'start', 'stop', '[WITHSCORES]'],
  zscore: ['key', 'member'],
  // 补全用到的其余常用命令
  bgsave: [],
  copy: ['source', 'destination', '[DB destination-db]', '[REPLACE]'],
  dbsize: [],
  flushdb: ['[ASYNC|SYNC]'],
  flushall: ['[ASYNC|SYNC]'],
  getrange: ['key', 'start', 'end'],
  getset: ['key', 'value'],
  hscan: ['key', 'cursor', '[MATCH pattern]', '[COUNT count]'],
  incrbyfloat: ['key', 'increment'],
  linsert: ['key', 'BEFORE|AFTER', 'pivot', 'element'],
  lpos: ['key', 'element', '[RANK rank]', '[COUNT num-matches]'],
  move: ['key', 'db'],
  object: ['ENCODING|FREQ|IDLETIME', 'key'],
  pfadd: ['key', 'element', '[element ...]'],
  pfcount: ['key', '[key ...]'],
  publish: ['channel', 'message'],
  randomkey: [],
  save: [],
  scan: ['cursor', '[MATCH pattern]', '[COUNT count]'],
  sscan: ['key', 'cursor', '[MATCH pattern]', '[COUNT count]'],
  slowlog: ['GET|LEN|RESET', '[count]'],
  subscribe: ['channel', '[channel ...]'],
  time: [],
  xadd: ['key', '[NOMKSTREAM]', '[MAXLEN count]', 'id', 'field', 'value', '[field value ...]'],
  xdel: ['key', 'id', '[id ...]'],
  xlen: ['key'],
  xrange: ['key', 'start', 'end', '[COUNT count]'],
  xrevrange: ['key', 'end', 'start', '[COUNT count]'],
  xtrim: ['key', 'MAXLEN count'],
  zscan: ['key', 'cursor', '[MATCH pattern]', '[COUNT count]'],
  zrevrank: ['key', 'member'],
}

const COMMAND_NAMES = Object.keys(REDIS_COMMAND_HINTS).sort()

// 按前缀匹配命令名；上限避免下拉太长
export function filterCommandNames(token, limit = 8) {
  const needle = String(token || '').trim().toLowerCase()
  if (!needle || needle.includes(' ')) return []
  return COMMAND_NAMES.filter((name) => name.startsWith(needle)).slice(0, limit)
}

// 命令按空白切分并保留分隔符，方便给命令名单独上色
export function tokenizeCommandLine(text) {
  return String(text || '')
    .split(/(\s+)/)
    .filter((part) => part !== '')
}
// 子命令：命令后面的第一个参数（CONFIG GET / CLIENT LIST …）
export const REDIS_SUBCOMMANDS = {
  acl: ['CAT', 'DELUSER', 'GENPASS', 'GETUSER', 'LIST', 'SETUSER', 'USERS', 'WHOAMI'],
  client: ['GETNAME', 'ID', 'INFO', 'KILL', 'LIST', 'NO-EVICT', 'PAUSE', 'REPLY', 'SETNAME', 'UNPAUSE'],
  cluster: ['COUNTKEYSINSLOT', 'GETKEYSINSLOT', 'INFO', 'KEYSLOT', 'NODES', 'REPLICAS', 'SLOTS', 'MYID'],
  command: ['COUNT', 'DOCS', 'GETKEYS', 'INFO', 'LIST'],
  config: ['GET', 'HELP', 'RESETSTAT', 'REWRITE', 'SET'],
  latency: ['DOCTOR', 'GRAPH', 'HISTORY', 'LATEST', 'RESET'],
  memory: ['DOCTOR', 'MALLOC-STATS', 'PURGE', 'STATS', 'USAGE'],
  object: ['ENCODING', 'FREQ', 'IDLETIME', 'REFCOUNT'],
  pubsub: ['CHANNELS', 'NUMPAT', 'NUMSUB', 'SHARDCHANNELS', 'SHARDNUMSUB'],
  script: ['EXISTS', 'FLUSH', 'LOAD'],
  slowlog: ['GET', 'HELP', 'LEN', 'RESET'],
  xgroup: ['CREATE', 'CREATECONSUMER', 'DELCONSUMER', 'DESTROY', 'SETID'],
  xinfo: ['CONSUMERS', 'GROUPS', 'STREAM'],
}

// 关键字：跟在参数后面的选项（SET … NX EX 30 / SCAN 0 MATCH …）
export const REDIS_COMMAND_KEYWORDS = {
  copy: ['DB', 'REPLACE'],
  expire: ['GT', 'LT', 'NX', 'XX'],
  getex: ['EX', 'EXAT', 'PERSIST', 'PX', 'PXAT'],
  lpos: ['COUNT', 'MAXLEN', 'RANK'],
  pexpire: ['GT', 'LT', 'NX', 'XX'],
  scan: ['COUNT', 'MATCH', 'TYPE'],
  hscan: ['COUNT', 'MATCH', 'NOVALUES'],
  set: ['EX', 'EXAT', 'GET', 'KEEPTTL', 'NX', 'PX', 'PXAT', 'XX'],
  sscan: ['COUNT', 'MATCH'],
  xadd: ['LIMIT', 'MAXLEN', 'MINID', 'NOMKSTREAM'],
  xrange: ['COUNT'],
  xrevrange: ['COUNT'],
  xtrim: ['LIMIT', 'MAXLEN', 'MINID'],
  zadd: ['CH', 'GT', 'INCR', 'LT', 'NX', 'XX'],
  zrange: ['BYLEX', 'BYSCORE', 'LIMIT', 'REV', 'WITHSCORES'],
  zrangebylex: ['LIMIT'],
  zrangebyscore: ['LIMIT', 'WITHSCORES'],
  zrevrange: ['WITHSCORES'],
  zrevrangebyscore: ['LIMIT', 'WITHSCORES'],
  zscan: ['COUNT', 'MATCH'],
}

/**
 * 参数补全：命令名之后，第一个参数优先补子命令，其余补关键字。
 * 返回 [{ value, hint }]，已输入过的关键字不再重复推荐。
 */
export function suggestArguments(input, limit = 8) {
  const raw = String(input || '')
  if (!raw.trim()) return []
  const endsWithSpace = /\s$/.test(raw)
  const tokens = raw.trim().split(/\s+/)
  // 还在敲命令名，交给 filterCommandNames
  if (tokens.length === 1 && !endsWithSpace) return []

  const command = tokens[0].toLowerCase()
  const partial = (endsWithSpace ? '' : tokens[tokens.length - 1]).toUpperCase()
  const argIndex = (endsWithSpace ? tokens.length : tokens.length - 1) - 1
  const used = new Set(tokens.slice(1).map((token) => token.toUpperCase()))

  if (argIndex === 0) {
    const subcommands = REDIS_SUBCOMMANDS[command]
    if (subcommands) {
      return subcommands
        .filter((value) => value.startsWith(partial))
        .slice(0, limit)
        .map((value) => ({ value, hint: '' }))
    }
  }

  // 关键字都在参数后面，第一个参数位置不推荐，避免刚敲完命令就弹一堆选项
  const keywords = argIndex >= 1 ? REDIS_COMMAND_KEYWORDS[command] || [] : []
  return keywords
    .filter((value) => value.startsWith(partial) && !used.has(value))
    .slice(0, limit)
    .map((value) => ({ value, hint: '' }))
}
