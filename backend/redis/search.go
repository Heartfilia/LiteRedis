package redis

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"LiteRedis/backend/config"

	"github.com/redis/go-redis/v9"
)

const maxSearchScanRounds = 64

func searchPageSize(configured, fallback int64) int64 {
	if configured > 0 {
		return configured
	}
	return fallback
}

func searchScanCount(pageSize int64) int64 {
	count := pageSize * 8
	if count < 1000 {
		return 1000
	}
	if count > 10000 {
		return 10000
	}
	return count
}

// The UI exposes * as its wildcard. Treat the other Redis glob operators as
// literals so fields such as JSON arrays beginning with '[' search naturally.
func userSearchGlob(pattern string) string {
	var builder strings.Builder
	for _, char := range pattern {
		switch char {
		case '?', '[', ']', '\\':
			builder.WriteRune('\\')
			builder.WriteRune(char)
		default:
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

// SearchValue 按 pattern 在 Redis 后端搜索集合成员，返回与 GetValue 相同的 KeyValue 结构。
// Hash/Set：UI 中 * 是通配符，其余 Redis glob 操作符按普通字符处理；
// ZSet：pattern 是 Redis glob（如 "user:*"）；
// List：pattern 是大小写不敏感的子串匹配（Redis 无 LSCAN）。
// 空 pattern → 等同 "*"，退化为重新加载前 loadCount 条。
// exact=true 时，Set 使用 SIsMember，Hash 使用 HGet 进行精确匹配。
func SearchValue(
	ctx context.Context,
	client redis.UniversalClient,
	key, keyType, pattern string,
	settings config.AppSettings,
	exact bool,
	cursor uint64,
) (config.KeyValue, error) {
	if strings.TrimSpace(pattern) == "" {
		pattern = "*"
	}

	kv := config.KeyValue{Key: key, Type: keyType}
	def := config.DefaultSettings()

	switch keyType {
	case "hash":
		if exact {
			kv.HashVal = map[string]string{}
			val, err := client.HGet(ctx, key, pattern).Result()
			if err != nil && err != redis.Nil {
				return kv, err
			}
			if err == nil {
				kv.HashVal = map[string]string{pattern: val}
			}
			break
		}
		pageSize := searchPageSize(settings.HashLoadCount, def.HashLoadCount)
		glob := userSearchGlob(pattern)
		result := make(map[string]string)
		scanCursor := cursor
		for round := 0; round < maxSearchScanRounds; round++ {
			keys, newCursor, err := client.HScan(ctx, key, scanCursor, glob, searchScanCount(pageSize)).Result()
			if err != nil {
				return kv, err
			}
			for i := 0; i+1 < len(keys); i += 2 {
				result[keys[i]] = keys[i+1]
			}
			scanCursor = newCursor
			if scanCursor == 0 || int64(len(result)) >= pageSize {
				break
			}
		}
		kv.HashVal = result
		kv.TotalCount = int64(len(result))
		kv.HasMore = scanCursor != 0
		kv.NextCursor = scanCursor

	case "set":
		if exact {
			kv.SetVal = []string{}
			isMember, err := client.SIsMember(ctx, key, pattern).Result()
			if err != nil {
				return kv, err
			}
			if isMember {
				kv.SetVal = []string{pattern}
			}
			break
		}
		pageSize := searchPageSize(settings.SetLoadCount, def.SetLoadCount)
		glob := userSearchGlob(pattern)
		var members []string
		scanCursor := cursor
		for round := 0; round < maxSearchScanRounds; round++ {
			batch, newCursor, err := client.SScan(ctx, key, scanCursor, glob, searchScanCount(pageSize)).Result()
			if err != nil {
				return kv, err
			}
			members = append(members, batch...)
			scanCursor = newCursor
			if scanCursor == 0 || int64(len(members)) >= pageSize {
				break
			}
		}
		kv.SetVal = members
		kv.TotalCount = int64(len(members))
		kv.HasMore = scanCursor != 0
		kv.NextCursor = scanCursor

	case "zset":
		pageSize := searchPageSize(settings.ZSetLoadCount, def.ZSetLoadCount)
		var zsetMembers []config.ZSetMember
		scanCursor := cursor
		for round := 0; round < maxSearchScanRounds; round++ {
			items, newCursor, err := client.ZScan(ctx, key, scanCursor, pattern, pageSize).Result()
			if err != nil {
				return kv, err
			}
			for i := 0; i+1 < len(items); i += 2 {
				score, err := strconv.ParseFloat(items[i+1], 64)
				if err != nil {
					return kv, fmt.Errorf("invalid zset score for member %q: %w", items[i], err)
				}
				zsetMembers = append(zsetMembers, config.ZSetMember{Member: items[i], Score: score})
			}
			scanCursor = newCursor
			if len(zsetMembers) > 0 || scanCursor == 0 {
				break
			}
		}
		kv.ZSetVal = zsetMembers
		kv.TotalCount = int64(len(zsetMembers))
		kv.HasMore = scanCursor != 0
		kv.NextCursor = scanCursor

	case "list":
		pageSize := searchPageSize(settings.ListLoadCount, def.ListLoadCount)
		if cursor > uint64(1<<63-1) {
			return kv, fmt.Errorf("list search cursor is out of range")
		}
		total, err := client.LLen(ctx, key).Result()
		if err != nil {
			return kv, err
		}
		lp := strings.ToLower(pattern)
		isWild := pattern == "*"
		var matched []string
		nextIndex := int64(cursor)
		chunkSize := searchScanCount(pageSize)
		for round := 0; round < maxSearchScanRounds && nextIndex < total; round++ {
			values, err := client.LRange(ctx, key, nextIndex, nextIndex+chunkSize-1).Result()
			if err != nil {
				return kv, err
			}
			if len(values) == 0 {
				nextIndex = total
				break
			}
			batchStart := nextIndex
			for index, value := range values {
				nextIndex = batchStart + int64(index) + 1
				if isWild || strings.Contains(strings.ToLower(value), lp) {
					matched = append(matched, value)
					if int64(len(matched)) >= pageSize {
						break
					}
				}
			}
			if int64(len(matched)) >= pageSize {
				break
			}
		}
		kv.ListVal = matched
		kv.TotalCount = int64(len(matched))
		kv.HasMore = nextIndex < total
		if kv.HasMore {
			kv.NextCursor = uint64(nextIndex)
		}
	}

	return kv, nil
}
