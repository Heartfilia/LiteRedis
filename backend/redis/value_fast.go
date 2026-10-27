package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"LiteRedis/backend/config"

	"github.com/redis/go-redis/v9"
)

func pickCount(value, fallback int64) int64 {
	if value > 0 {
		return value
	}
	return fallback
}

func ttlSecondsOf(ttl time.Duration) int64 {
	if ttl < 0 {
		return int64(ttl)
	}
	return int64(ttl / time.Second)
}

// getValueKnownType 已知 key 类型时，把 TTL、总数和数据读取合并成同一次往返。
// 类型如果和实际不符（key 被改过类型），会返回 WRONGTYPE，由调用方回退到带探测的路径。
func getValueKnownType(ctx context.Context, client redis.UniversalClient, key, keyType string, settings config.AppSettings, cursor uint64, offset int, zsetSort, scope string) (config.KeyValue, error) {
	kv := config.KeyValue{Key: key, Type: keyType, TotalCount: -1}
	def := config.DefaultSettings()
	hashCount := pickCount(settings.HashLoadCount, def.HashLoadCount)
	listCount := pickCount(settings.ListLoadCount, def.ListLoadCount)
	setCount := pickCount(settings.SetLoadCount, def.SetLoadCount)
	zsetCount := pickCount(settings.ZSetLoadCount, def.ZSetLoadCount)

	pipe := client.Pipeline()
	ttlCmd := pipe.TTL(ctx, key)

	switch keyType {
	case "string":
		getCmd := pipe.Get(ctx, key)
		if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
			return kv, err
		}
		kv.TTL = ttlSecondsOf(ttlCmd.Val())
		val := getCmd.Val()
		if len(val) > maxStringValueBytes {
			val = truncateUTF8Safe(val, maxStringValueBytes)
			kv.Truncated = true
		}
		kv.StringVal = val
		return kv, nil

	case "list":
		cached, hit := peekListChunk(scope, key, int64(offset), listCount)
		lenCmd := pipe.LLen(ctx, key)
		var rangeCmd *redis.StringSliceCmd
		if !hit {
			size := int64(listChunkSize)
			if listCount > size {
				size = listCount
			}
			rangeCmd = pipe.LRange(ctx, key, int64(offset), int64(offset)+size-1)
		}
		if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
			return kv, err
		}
		kv.TTL = ttlSecondsOf(ttlCmd.Val())
		total := lenCmd.Val()
		items := cached
		if !hit {
			items = rangeCmd.Val()
			storeListChunk(scope, key, int64(offset), items)
			if int64(len(items)) > listCount {
				items = items[:listCount]
			}
		}
		kv.ListVal = items
		kv.NextOffset = offset + len(items)
		kv.TotalCount = total
		kv.HasMore = int64(offset+len(items)) < total
		return kv, nil

	case "hash":
		lenCmd := pipe.HLen(ctx, key)
		scanCmd := pipe.HScan(ctx, key, cursor, "*", hashCount)
		if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
			return kv, err
		}
		kv.TTL = ttlSecondsOf(ttlCmd.Val())
		result := make(map[string]string)
		values, next, _ := scanCmd.Result()
		loaded := int64(0)
		for i := 0; i+1 < len(values); i += 2 {
			result[values[i]] = values[i+1]
			loaded++
		}
		cursor = next
		for cursor != 0 && loaded < hashCount {
			values, next, err := client.HScan(ctx, key, cursor, "*", hashCount).Result()
			if err != nil {
				return kv, err
			}
			for i := 0; i+1 < len(values); i += 2 {
				result[values[i]] = values[i+1]
				loaded++
			}
			cursor = next
		}
		kv.HashVal = result
		kv.NextCursor = cursor
		kv.HasMore = cursor != 0
		kv.TotalCount = lenCmd.Val()
		return kv, nil

	case "set":
		lenCmd := pipe.SCard(ctx, key)
		scanCmd := pipe.SScan(ctx, key, cursor, "*", setCount)
		if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
			return kv, err
		}
		kv.TTL = ttlSecondsOf(ttlCmd.Val())
		members, next, _ := scanCmd.Result()
		cursor = next
		for cursor != 0 && int64(len(members)) < setCount {
			batch, next, err := client.SScan(ctx, key, cursor, "*", setCount).Result()
			if err != nil {
				return kv, err
			}
			members = append(members, batch...)
			cursor = next
		}
		kv.SetVal = members
		kv.NextCursor = cursor
		kv.HasMore = cursor != 0
		kv.TotalCount = lenCmd.Val()
		return kv, nil

	case "zset":
		end := int64(offset) + zsetCount - 1
		lenCmd := pipe.ZCard(ctx, key)
		var rangeCmd *redis.ZSliceCmd
		if zsetSort == "desc" {
			rangeCmd = pipe.ZRevRangeWithScores(ctx, key, int64(offset), end)
		} else {
			rangeCmd = pipe.ZRangeWithScores(ctx, key, int64(offset), end)
		}
		if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
			return kv, err
		}
		kv.TTL = ttlSecondsOf(ttlCmd.Val())
		vals := rangeCmd.Val()
		total := lenCmd.Val()
		members := make([]config.ZSetMember, len(vals))
		for i, z := range vals {
			members[i] = config.ZSetMember{Member: fmt.Sprintf("%v", z.Member), Score: z.Score}
		}
		kv.ZSetVal = members
		kv.NextOffset = offset + len(vals)
		kv.TotalCount = total
		kv.HasMore = int64(offset+len(vals)) < total
		return kv, nil
	}

	// 其他类型（如 stream）走原来的路径
	return kv, errFallbackToProbe
}

var errFallbackToProbe = errors.New("fallback to probe path")
