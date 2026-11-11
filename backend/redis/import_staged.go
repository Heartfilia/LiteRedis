package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"LiteRedis/backend/config"
	"github.com/redis/go-redis/v9"
)

// Construction is one atomic script, not a server-side streaming operation.
// Keep its budget below the export/read budget to bound work and extra memory.
const ImportMaxKeyBytes = 5 << 20
const ImportMaxKeyItems = 10000

// All fallible construction happens on KEYS[2]. The only mutation of the target
// is the final RENAME (or DEL for an immediately-expired entry). Lua is not a
// rollback transaction: pcall + cleanup are intentional, not implicit rollback.
const stagedExportScript = `
if ARGV[1] == '0' and redis.call('EXISTS', KEYS[1]) == 1 then return 0 end
if redis.call('EXISTS', KEYS[2]) == 1 then return redis.error_reply('ERR staging key collision') end
local function abort(message)
  redis.pcall('PEXPIRE', KEYS[2], 600000)
  local cleaned = redis.pcall('DEL', KEYS[2])
  if type(cleaned) == 'table' and cleaned.err then
    return redis.error_reply(message .. '; staging cleanup failed: ' .. KEYS[2] .. '; ' .. cleaned.err)
  end
  return redis.error_reply(message)
end
local index = 3
while index <= #ARGV do
  local command = ARGV[index]
  local count = tonumber(ARGV[index+1])
  local args = {KEYS[2]}
  for i = 1,count do args[#args+1] = ARGV[index+1+i] end
  local result = redis.pcall(command, unpack(args))
  if type(result) == 'table' and result.err then return abort(result.err) end
  -- Bound abandoned staging data even if the connection disappears. If expiry
  -- is forbidden, fail closed and attempt explicit cleanup before any commit.
  local expiring = redis.pcall('PEXPIRE', KEYS[2], 600000)
  if type(expiring) == 'table' and expiring.err then return abort(expiring.err) end
  index = index + count + 2
end
local ttl = tonumber(ARGV[2])
local expiry
if ttl < 0 then expiry = redis.pcall('PERSIST', KEYS[2])
else expiry = redis.pcall('EXPIRE', KEYS[2], ttl) end
if type(expiry) == 'table' and expiry.err then return abort(expiry.err) end
local committed
if ttl == 0 then committed = redis.pcall('DEL', KEYS[1])
else committed = redis.pcall('RENAME', KEYS[2], KEYS[1]) end
if type(committed) == 'table' and committed.err then return abort(committed.err) end
return 1`

func writeStagedExport(ctx context.Context, client redis.UniversalClient, entry config.ExportedKey, overwrite bool) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, &InvalidExportError{Err: errors.New("导入已在发送前取消")}
	}
	temporary, err := stagingKey(entry.Key)
	if err != nil {
		return false, &InvalidExportError{Err: err}
	}
	flag := "0"
	if overwrite {
		flag = "1"
	}
	args := []interface{}{flag, entry.TTL}
	appendCommand := func(name string, values ...interface{}) {
		args = append(args, name, len(values))
		args = append(args, values...)
	}
	switch entry.Type {
	case "string":
		appendCommand("SET", entry.String)
	case "hash":
		values := make([]interface{}, 0, exportPageSize*2)
		for field, value := range entry.Hash {
			values = append(values, field, value)
			if len(values) == exportPageSize*2 {
				appendCommand("HMSET", values...)
				values = values[:0]
			}
		}
		if len(values) > 0 {
			appendCommand("HMSET", values...)
		}
	case "list", "set":
		members, command := entry.List, "RPUSH"
		if entry.Type == "set" {
			members, command = entry.Set, "SADD"
		}
		for start := 0; start < len(members); start += exportPageSize {
			values := make([]interface{}, 0, exportPageSize)
			for _, member := range members[start:min(start+exportPageSize, len(members))] {
				values = append(values, member)
			}
			appendCommand(command, values...)
		}
	case "zset":
		for start := 0; start < len(entry.ZSet); start += exportPageSize {
			values := make([]interface{}, 0, exportPageSize*2)
			for _, member := range entry.ZSet[start:min(start+exportPageSize, len(entry.ZSet))] {
				values = append(values, strconv.FormatFloat(member.Score, 'g', -1, 64), member.Member)
			}
			appendCommand("ZADD", values...)
		}
	case "stream":
		for _, item := range entry.Stream {
			values := []interface{}{item.ID}
			for field, value := range item.Fields {
				values = append(values, field, value)
			}
			appendCommand("XADD", values...)
		}
	}
	n, err := client.Eval(ctx, stagedExportScript, []string{entry.Key, temporary}, args...).Int()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

// Redis Cluster hashes the first nonempty {...} tag, otherwise the whole key.
// Generated tags let even untagged or brace-containing keys stage in one slot.
func redisKeySlot(key string) uint16 {
	if start := strings.IndexByte(key, '{'); start >= 0 {
		if end := strings.IndexByte(key[start+1:], '}'); end > 0 {
			key = key[start+1 : start+1+end]
		}
	}
	var crc uint16
	for i := 0; i < len(key); i++ {
		crc ^= uint16(key[i]) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc % 16384
}

var stagingTags struct {
	sync.Once
	values [16384]string
}

func stagingKey(target string) (string, error) {
	stagingTags.Do(func() {
		remaining := len(stagingTags.values)
		for i := 0; remaining > 0; i++ {
			tag := strconv.Itoa(i)
			slot := redisKeySlot(tag)
			if stagingTags.values[slot] == "" {
				stagingTags.values[slot] = tag
				remaining--
			}
		}
	})
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("staging key randomness: %w", err)
	}
	return "{" + stagingTags.values[redisKeySlot(target)] + "}:__literedis_import:" + hex.EncodeToString(nonce[:]), nil
}
