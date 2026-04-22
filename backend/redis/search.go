package redis

import (
	"context"
	"strconv"
	"strings"

	"LiteRedis/backend/config"

	"github.com/redis/go-redis/v9"
)

// SearchValue 按 pattern 在 Redis 后端搜索集合成员，返回与 GetValue 相同的 KeyValue 结构。
// Hash/Set/ZSet：pattern 是 Redis glob（如 "user:*"）；
// List：pattern 是大小写不敏感的子串匹配（Redis 无 LSCAN）。
// 空 pattern → 等同 "*"，退化为重新加载前 loadCount 条。
func SearchValue(
	ctx context.Context,
	client redis.UniversalClient,
	key, keyType, pattern string,
	settings config.AppSettings,
) (config.KeyValue, error) {
	if strings.TrimSpace(pattern) == "" {
		pattern = "*"
	}

	kv := config.KeyValue{Key: key, Type: keyType}
	def := config.DefaultSettings()

	switch keyType {
	case "hash":
		count := settings.HashLoadCount
		if count <= 0 {
			count = def.HashLoadCount
		}
		result := make(map[string]string)
		var cursor uint64
		for {
			keys, newCursor, err := client.HScan(ctx, key, cursor, pattern, count).Result()
			if err != nil {
				return kv, err
			}
			for i := 0; i+1 < len(keys); i += 2 {
				result[keys[i]] = keys[i+1]
				if int64(len(result)) >= count {
					goto hashDone
				}
			}
			cursor = newCursor
			if cursor == 0 {
				break
			}
		}
	hashDone:
		kv.HashVal = result

	case "set":
		count := settings.SetLoadCount
		if count <= 0 {
			count = def.SetLoadCount
		}
		var members []string
		iter := client.SScan(ctx, key, 0, pattern, count).Iterator()
		for iter.Next(ctx) {
			members = append(members, iter.Val())
			if int64(len(members)) >= count {
				break
			}
		}
		if err := iter.Err(); err != nil {
			return kv, err
		}
		kv.SetVal = members

	case "zset":
		count := settings.ZSetLoadCount
		if count <= 0 {
			count = def.ZSetLoadCount
		}
		var zsetMembers []config.ZSetMember
		var cursor uint64
		for {
			items, newCursor, err := client.ZScan(ctx, key, cursor, pattern, count).Result()
			if err != nil {
				return kv, err
			}
			for i := 0; i+1 < len(items); i += 2 {
				score, _ := strconv.ParseFloat(items[i+1], 64)
				zsetMembers = append(zsetMembers, config.ZSetMember{Member: items[i], Score: score})
				if int64(len(zsetMembers)) >= count {
					goto zsetDone
				}
			}
			cursor = newCursor
			if cursor == 0 {
				break
			}
		}
	zsetDone:
		kv.ZSetVal = zsetMembers

	case "list":
		count := settings.ListLoadCount
		if count <= 0 {
			count = def.ListLoadCount
		}
		all, err := client.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			return kv, err
		}
		lp := strings.ToLower(pattern)
		isWild := pattern == "*"
		var matched []string
		for _, v := range all {
			if isWild || strings.Contains(strings.ToLower(v), lp) {
				matched = append(matched, v)
				if int64(len(matched)) >= count {
					break
				}
			}
		}
		kv.ListVal = matched
	}

	return kv, nil
}
