package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"LiteRedis/backend/config"

	"github.com/redis/go-redis/v9"
)

var (
	errSetMemberNotFound  = errors.New("set member does not exist")
	errZSetMemberNotFound = errors.New("zset member does not exist")
	errStringKeyNotFound  = errors.New("string key does not exist")

	setStringPreserveTTLScript = redis.NewScript(`
local ttl = redis.call('PTTL', KEYS[1])
if ttl == -2 then
  return 0
end
redis.call('SET', KEYS[1], ARGV[1])
if ttl > 0 then
  redis.call('PEXPIRE', KEYS[1], ttl)
end
return 1
`)

	renameSetMemberScript = redis.NewScript(`
local source = ARGV[1]
local destination = ARGV[2]
if redis.call('SISMEMBER', KEYS[1], source) == 0 then
  return 0
end
if source == destination then
  return 1
end
redis.call('SREM', KEYS[1], source)
redis.call('SADD', KEYS[1], destination)
return 1
`)

	renameZSetMemberScript = redis.NewScript(`
local source = ARGV[1]
local destination = ARGV[2]
local score = redis.call('ZSCORE', KEYS[1], source)
if not score then
  return 0
end
if source == destination then
  return 1
end
redis.call('ZREM', KEYS[1], source)
redis.call('ZADD', KEYS[1], score, destination)
return 1
`)
)

// GetValue 读取 key 的值（按类型分支），支持 cursor/offset 分页。
// cursor=0, offset=0 表示第一页。loadCount 控制每次加载条数（≤0 取默认）。
func GetValue(ctx context.Context, client redis.UniversalClient, key string, settings config.AppSettings, cursor uint64, offset int, zsetSort, streamStart string) (config.KeyValue, error) {
	keyInfo, err := GetKeyInfo(ctx, client, key)
	if err != nil {
		return config.KeyValue{}, err
	}

	kv := config.KeyValue{
		Key:        key,
		Type:       keyInfo.Type,
		TTL:        keyInfo.TTL,
		TotalCount: -1,
	}

	def := config.DefaultSettings()

	hashCount := settings.HashLoadCount
	if hashCount <= 0 {
		hashCount = def.HashLoadCount
	}
	listCount := settings.ListLoadCount
	if listCount <= 0 {
		listCount = def.ListLoadCount
	}
	setCount := settings.SetLoadCount
	if setCount <= 0 {
		setCount = def.SetLoadCount
	}
	zsetCount := settings.ZSetLoadCount
	if zsetCount <= 0 {
		zsetCount = def.ZSetLoadCount
	}
	streamCount := settings.StreamLoadCount
	if streamCount <= 0 {
		streamCount = def.StreamLoadCount
	}

	switch keyInfo.Type {
	case "string":
		val, err := client.Get(ctx, key).Result()
		if err != nil && err != redis.Nil {
			return kv, err
		}
		kv.StringVal = val
		kv.HasMore = false

	case "hash":
		result := make(map[string]string)
		loaded := int64(0)
		for {
			keys, newCursor, err := client.HScan(ctx, key, cursor, "*", hashCount).Result()
			if err != nil {
				return kv, err
			}
			for i := 0; i+1 < len(keys); i += 2 {
				result[keys[i]] = keys[i+1]
				loaded++
			}
			cursor = newCursor
			if cursor == 0 || loaded >= hashCount {
				break
			}
		}
		kv.HashVal = result
		kv.NextCursor = cursor
		kv.HasMore = cursor != 0
		total, err := client.HLen(ctx, key).Result()
		if err != nil {
			return kv, err
		}
		kv.TotalCount = total

	case "list":
		end := int64(offset) + listCount - 1
		val, err := client.LRange(ctx, key, int64(offset), end).Result()
		if err != nil {
			return kv, err
		}
		total, err := client.LLen(ctx, key).Result()
		if err != nil {
			return kv, err
		}
		kv.ListVal = val
		kv.NextOffset = offset + len(val)
		kv.TotalCount = total
		kv.HasMore = int64(offset+len(val)) < total

	case "set":
		var members []string
		for {
			batch, newCursor, err := client.SScan(ctx, key, cursor, "*", setCount).Result()
			if err != nil {
				return kv, err
			}
			members = append(members, batch...)
			cursor = newCursor
			if cursor == 0 || int64(len(members)) >= setCount {
				break
			}
		}
		kv.SetVal = members
		kv.NextCursor = cursor
		kv.HasMore = cursor != 0
		total, err := client.SCard(ctx, key).Result()
		if err != nil {
			return kv, err
		}
		kv.TotalCount = total

	case "zset":
		end := int64(offset) + zsetCount - 1
		var (
			vals []redis.Z
			err  error
		)
		if zsetSort == "desc" {
			vals, err = client.ZRevRangeWithScores(ctx, key, int64(offset), end).Result()
		} else {
			vals, err = client.ZRangeWithScores(ctx, key, int64(offset), end).Result()
		}
		if err != nil {
			return kv, err
		}
		total, err := client.ZCard(ctx, key).Result()
		if err != nil {
			return kv, err
		}
		members := make([]config.ZSetMember, len(vals))
		for i, z := range vals {
			members[i] = config.ZSetMember{
				Member: fmt.Sprintf("%v", z.Member),
				Score:  z.Score,
			}
		}
		kv.ZSetVal = members
		kv.NextOffset = offset + len(vals)
		kv.TotalCount = total
		kv.HasMore = int64(offset+len(vals)) < total

	case "stream":
		entries, nextID, hasMore, err := getStreamPage(ctx, client, key, streamCount, streamStart)
		if err != nil {
			return kv, err
		}
		total, err := client.XLen(ctx, key).Result()
		if err != nil {
			return kv, err
		}
		kv.StreamVal = entries
		kv.NextStreamID = nextID
		kv.HasMore = hasMore
		kv.TotalCount = total
	}

	return kv, nil
}

// getStreamPage reads newest-to-oldest stream entries. The next ID is used as
// an exclusive upper bound so entries added while paging cannot duplicate a
// previously returned item.
func getStreamPage(ctx context.Context, client redis.UniversalClient, key string, count int64, start string) ([]config.StreamEntry, string, bool, error) {
	if count <= 0 {
		count = config.DefaultSettings().StreamLoadCount
	}
	if start == "" {
		start = "+"
	} else if start != "+" && !strings.HasPrefix(start, "(") {
		start = "(" + start
	}
	vals, err := client.XRevRangeN(ctx, key, start, "-", count+1).Result()
	if err != nil {
		return nil, "", false, err
	}
	hasMore := int64(len(vals)) > count
	if hasMore {
		vals = vals[:count]
	}
	entries := make([]config.StreamEntry, len(vals))
	for i, msg := range vals {
		fields := make(map[string]string, len(msg.Values))
		for k, v := range msg.Values {
			fields[k] = fmt.Sprintf("%v", v)
		}
		entries[i] = config.StreamEntry{ID: msg.ID, Fields: fields}
	}
	nextID := ""
	if hasMore && len(entries) > 0 {
		nextID = entries[len(entries)-1].ID
	}
	return entries, nextID, hasMore, nil
}

// SetStringPreserveTTL atomically updates an existing string without resetting
// its current TTL. An expired key is not recreated.
func SetStringPreserveTTL(ctx context.Context, client redis.UniversalClient, key, value string) error {
	updated, err := setStringPreserveTTLScript.Run(ctx, client, []string{key}, value).Int()
	if err != nil {
		return err
	}
	if updated == 0 {
		return fmt.Errorf("%w: %q", errStringKeyNotFound, key)
	}
	return nil
}

// HSet 设置 hash field
func HSet(ctx context.Context, client redis.UniversalClient, key, field, value string) error {
	return client.HSet(ctx, key, field, value).Err()
}

// HDel 删除 hash field
func HDel(ctx context.Context, client redis.UniversalClient, key, field string) error {
	return client.HDel(ctx, key, field).Err()
}

// LPush 向 list 头部插入
func LPush(ctx context.Context, client redis.UniversalClient, key, value string) error {
	return client.LPush(ctx, key, value).Err()
}

// RPush 向 list 尾部插入
func RPush(ctx context.Context, client redis.UniversalClient, key, value string) error {
	return client.RPush(ctx, key, value).Err()
}

// LSet 设置 list 指定索引的值
func LSet(ctx context.Context, client redis.UniversalClient, key string, index int64, value string) error {
	return client.LSet(ctx, key, index, value).Err()
}

// LRem 删除 list 中与 value 相等的元素（count=0 全部删除）
func LRem(ctx context.Context, client redis.UniversalClient, key string, count int64, value string) error {
	return client.LRem(ctx, key, count, value).Err()
}

// SAdd 向 set 添加成员
func SAdd(ctx context.Context, client redis.UniversalClient, key, member string) error {
	return client.SAdd(ctx, key, member).Err()
}

// SRem 从 set 删除成员
func SRem(ctx context.Context, client redis.UniversalClient, key, member string) error {
	return client.SRem(ctx, key, member).Err()
}

// RenameSetMember atomically replaces a set member. If destination already
// exists, Redis merges both members, matching SREM followed by SADD semantics.
func RenameSetMember(ctx context.Context, client redis.UniversalClient, key, source, destination string) error {
	updated, err := renameSetMemberScript.Run(ctx, client, []string{key}, source, destination).Int()
	if err != nil {
		return err
	}
	if updated == 0 {
		return fmt.Errorf("%w: %q", errSetMemberNotFound, source)
	}
	return nil
}

// ZAdd 向 zset 添加成员
func ZAdd(ctx context.Context, client redis.UniversalClient, key, member string, score float64) error {
	return client.ZAdd(ctx, key, redis.Z{Score: score, Member: member}).Err()
}

// ZRem 从 zset 删除成员
func ZRem(ctx context.Context, client redis.UniversalClient, key, member string) error {
	return client.ZRem(ctx, key, member).Err()
}

// RenameZSetMember atomically moves the source member's current score to the
// destination member. Existing destinations are overwritten as with ZADD.
func RenameZSetMember(ctx context.Context, client redis.UniversalClient, key, source, destination string) error {
	updated, err := renameZSetMemberScript.Run(ctx, client, []string{key}, source, destination).Int()
	if err != nil {
		return err
	}
	if updated == 0 {
		return fmt.Errorf("%w: %q", errZSetMemberNotFound, source)
	}
	return nil
}
