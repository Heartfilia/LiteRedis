package redis

import "strings"

// readonlyAllowedCommands 保护模式下允许直接执行的只读命令
var readonlyAllowedCommands = map[string]struct{}{
	"PING": {}, "ECHO": {}, "TIME": {}, "INFO": {}, "DBSIZE": {}, "LASTSAVE": {},
	"ROLE": {}, "EXISTS": {}, "TYPE": {}, "TTL": {}, "PTTL": {}, "KEYS": {},
	"SCAN": {}, "RANDOMKEY": {}, "DUMP": {}, "GET": {}, "MGET": {}, "STRLEN": {},
	"GETRANGE": {}, "SUBSTR": {}, "BITCOUNT": {}, "BITPOS": {}, "GETBIT": {},
	"HGET": {}, "HMGET": {}, "HGETALL": {}, "HKEYS": {}, "HVALS": {}, "HLEN": {},
	"HEXISTS": {}, "HSTRLEN": {}, "HRANDFIELD": {}, "HSCAN": {},
	"LLEN": {}, "LRANGE": {}, "LINDEX": {}, "LPOS": {},
	"SMEMBERS": {}, "SCARD": {}, "SISMEMBER": {}, "SMISMEMBER": {},
	"SRANDMEMBER": {}, "SSCAN": {},
	"ZCARD": {}, "ZCOUNT": {}, "ZSCORE": {}, "ZMSCORE": {}, "ZRANGE": {},
	"ZRANGEBYSCORE": {}, "ZREVRANGE": {}, "ZREVRANGEBYSCORE": {}, "ZRANGEBYLEX": {},
	"ZREVRANGEBYLEX": {}, "ZRANK": {}, "ZREVRANK": {}, "ZSCAN": {}, "ZLEXCOUNT": {},
	"ZRANDMEMBER": {},
	"XLEN":        {}, "XRANGE": {}, "XREVRANGE": {}, "XPENDING": {},
	"GEOPOS": {}, "GEODIST": {}, "GEOHASH": {}, "GEOSEARCH": {},
}

// readonlyAllowedSubcommands 多词命令只放行只读子命令
var readonlyAllowedSubcommands = map[string]map[string]struct{}{
	"CONFIG":  {"GET": {}},
	"ACL":     {"WHOAMI": {}, "LIST": {}, "USERS": {}, "CAT": {}, "GETUSER": {}},
	"CLIENT":  {"LIST": {}, "INFO": {}, "GETNAME": {}, "ID": {}},
	"MEMORY":  {"USAGE": {}, "STATS": {}, "DOCTOR": {}},
	"OBJECT":  {"ENCODING": {}, "REFCOUNT": {}, "IDLETIME": {}, "FREQ": {}},
	"XINFO":   {"STREAM": {}, "GROUPS": {}, "CONSUMERS": {}},
	"LATENCY": {"LATEST": {}, "HISTORY": {}, "DOCTOR": {}, "RESET": {}},
	"SLOWLOG": {"GET": {}, "LEN": {}},
	"COMMAND": {"COUNT": {}, "INFO": {}, "DOCS": {}, "GETKEYS": {}},
	"CLUSTER": {"INFO": {}, "NODES": {}, "SLOTS": {}, "COUNTKEYSINSLOT": {}, "KEYSLOT": {}},
}

// IsReadOnlyCommandAllowed 判断命令在保护模式下是否可以直接执行。
// 只认白名单：没列出来的命令一律需要用户确认，避免漏掉新的写命令。
func IsReadOnlyCommandAllowed(command string) bool {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return true
	}
	name := strings.ToUpper(fields[0])
	if subcommands, ok := readonlyAllowedSubcommands[name]; ok {
		// 多词命令必须显式给出白名单里的子命令，裸命令一律按需确认处理
		if len(fields) < 2 {
			return false
		}
		_, ok := subcommands[strings.ToUpper(fields[1])]
		return ok
	}
	_, ok := readonlyAllowedCommands[name]
	return ok
}
